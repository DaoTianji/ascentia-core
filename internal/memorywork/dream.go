package memorywork

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	pgintegration "ascentia-core/internal/integration/pg"
	httpllm "ascentia-core/internal/llm"
	"ascentia-core/internal/types"
	"ascentia-core/internal/usage"
	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/reflection"
)

// DreamConsolidator runs an LLM "dream" pass over stored memories per session (merge, drop noise, fix contradictions).
type DreamConsolidator struct {
	LLM           *httpllm.Holder
	PG            *pgintegration.MemoryStore
	MaxTokens     int
	MaxRowsLoad   int
	MinRowsToRun  int
	MaxRowsOutput int
}

// NewDreamConsolidator returns a reflection.Consolidator; LLM and PG must be non-nil.
func NewDreamConsolidator(llm *httpllm.Holder, pg *pgintegration.MemoryStore) *DreamConsolidator {
	return &DreamConsolidator{
		LLM:           llm,
		PG:            pg,
		MaxTokens:     3072,
		MaxRowsLoad:   120,
		MinRowsToRun:  3,
		MaxRowsOutput: 60,
	}
}

func (d *DreamConsolidator) llmClient() *httpllm.Client {
	if d == nil || d.LLM == nil {
		return nil
	}
	return d.LLM.Load()
}

func (d *DreamConsolidator) Consolidate(ctx context.Context, scope identity.TenantScope, opts reflection.ConsolidationOptions) error {
	if d == nil || d.PG == nil {
		return nil
	}
	if d.llmClient() == nil {
		return nil
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	sessions, err := d.sessionsToProcess(ctx, scope, opts.SessionID)
	if err != nil {
		return err
	}
	for _, sid := range sessions {
		if err := d.consolidateOneSession(ctx, scope, sid); err != nil {
			return err
		}
	}
	return nil
}

func (d *DreamConsolidator) sessionsToProcess(ctx context.Context, scope identity.TenantScope, single string) ([]string, error) {
	if strings.TrimSpace(single) != "" {
		return []string{single}, nil
	}
	return d.PG.ListDistinctSessionsForTenant(ctx, scope.UserID, scope.AgentID)
}

func (d *DreamConsolidator) consolidateOneSession(ctx context.Context, scope identity.TenantScope, sessionID string) error {
	client := d.llmClient()
	if client == nil {
		return nil
	}
	limit := d.MaxRowsLoad
	if limit <= 0 {
		limit = 120
	}
	rows, err := d.PG.ListAllForSession(ctx, sessionID, scope.UserID, scope.AgentID, limit)
	if err != nil {
		return err
	}
	if len(rows) < d.minRun() {
		return nil
	}

	var b strings.Builder
	for _, r := range rows {
		b.WriteString("- [")
		b.WriteString(r.Category)
		b.WriteString("] ")
		b.WriteString(r.Key)
		b.WriteString(" :: ")
		b.WriteString(strings.TrimSpace(r.Content))
		b.WriteByte('\n')
	}

	sys := types.Message{
		Role: types.RoleSystem,
		Content: strings.TrimSpace(`
你是记忆整理（Dream）助手。下面是一个会话下的长期记忆列表（可能重复、矛盾或含噪声）。
任务：输出**整理后**的 JSON 数组，每项 {"category":"user|feedback|project|reference","key":"snake_case","content":"合并后的一句事实"}。
规则：
- 语义重复合并为一条，保留信息更完整、更新的版本。
- 明显矛盾时保留更可能仍成立的一条，并在 content 里简短说明取舍（可选）。
- 删掉纯临时、无长期价值的条目。
- key 保持稳定可读；可重命名 key 若明显更清晰。
- 条目数不超过输入条数，且不超过 60。
输出只能是 JSON 数组，不要 markdown。`),
	}
	user := types.Message{
		Role:    types.RoleUser,
		Content: "待整理记忆：\n" + b.String(),
	}
	maxTok := d.MaxTokens
	if maxTok <= 0 {
		maxTok = 3072
	}
	dreamCtx := usage.WithAttribution(ctx, usage.Attribution{
		UserID:    scope.UserID,
		AgentID:   scope.AgentID,
		SessionID: sessionID,
		RequestID: "dream",
	})
	resp, err := client.ChatTextOnly(dreamCtx, []types.Message{sys, user}, maxTok)
	if err != nil {
		return err
	}
	raw := stripJSONFence(strings.TrimSpace(resp.AssistantMsg.Content))
	items, err := parseDreamItems(raw, d.maxOut())
	if err != nil {
		return fmt.Errorf("dream parse session %s: %w", sessionID, err)
	}
	if len(items) == 0 {
		log.Printf("[dream] skip session %s: model returned empty array", sessionID)
		return nil
	}
	repl := make([]struct {
		Category string
		Key      string
		Content  string
	}, 0, len(items))
	for _, it := range items {
		key := strings.TrimSpace(it.Key)
		content := strings.TrimSpace(it.Content)
		if key == "" || content == "" {
			continue
		}
		if len(key) > 200 || len(content) > 8000 {
			continue
		}
		cat := normalizeCategory(it.Category)
		repl = append(repl, struct {
			Category string
			Key      string
			Content  string
		}{Category: string(cat), Key: key, Content: content})
	}
	if len(repl) == 0 {
		return nil
	}
	if err := d.PG.ReplaceSessionMemories(ctx, sessionID, scope.UserID, scope.AgentID, repl); err != nil {
		return err
	}
	log.Printf("[dream] consolidated session %s: %d -> %d rows", sessionID, len(rows), len(repl))
	return nil
}

func (d *DreamConsolidator) minRun() int {
	if d.MinRowsToRun > 0 {
		return d.MinRowsToRun
	}
	return 3
}

func (d *DreamConsolidator) maxOut() int {
	if d.MaxRowsOutput > 0 {
		return d.MaxRowsOutput
	}
	return 60
}

func parseDreamItems(raw string, maxN int) ([]memItem, error) {
	i := strings.Index(raw, "[")
	j := strings.LastIndex(raw, "]")
	if i < 0 || j <= i {
		return nil, fmt.Errorf("no json array")
	}
	raw = raw[i : j+1]
	var items []memItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, err
	}
	if len(items) > maxN {
		items = items[:maxN]
	}
	return items, nil
}
