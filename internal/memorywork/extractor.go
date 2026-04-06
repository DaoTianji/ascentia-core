package memorywork

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	pgintegration "ascentia-core/internal/integration/pg"
	httpllm "ascentia-core/internal/llm"
	"ascentia-core/internal/types"
	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/memory"
	"ascentia-core/pkg/agent_core/transcript"
)

// LLMExtractor uses a cheap text-only completion to propose durable memories from a turn.
type LLMExtractor struct {
	LLM        *httpllm.Holder
	PG         *pgintegration.MemoryStore
	MaxTokens  int
	MinTurnLen int
}

// NewLLMExtractor returns an extractor ready to assign to runtime.Service.PostTurn.
func NewLLMExtractor(llm *httpllm.Holder, pg *pgintegration.MemoryStore) *LLMExtractor {
	return &LLMExtractor{LLM: llm, PG: pg, MaxTokens: 512, MinTurnLen: 64}
}

func (e *LLMExtractor) llmClient() *httpllm.Client {
	if e == nil || e.LLM == nil {
		return nil
	}
	return e.LLM.Load()
}

func (e *LLMExtractor) AfterTurn(ctx context.Context, scope identity.TenantScope, sessionID string, turn []transcript.Message) error {
	if e == nil || e.PG == nil {
		return nil
	}
	client := e.llmClient()
	if client == nil {
		return nil
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	text := summarizeTurn(turn)
	if len([]rune(text)) < e.minLen() {
		return nil
	}
	maxTok := e.MaxTokens
	if maxTok <= 0 {
		maxTok = 512
	}
	sys := types.Message{
		Role: types.RoleSystem,
		Content: strings.TrimSpace(`
你是记忆策展助手。根据对话片段，仅提取**值得跨轮次保留**的稳定事实（用户偏好、项目规则、世界设定、用户明确反馈）。
不要保存：临时任务步骤、一次性命令、已在工具结果里的冗长原文、密钥与隐私。
输出必须是 **JSON 数组**，元素形如：
[{"category":"user|feedback|project|reference","key":"snake_case_id","content":"一句具体事实"}]
最多 5 条；若无值得保存的内容，输出 []。
只输出 JSON，不要 markdown 围栏。`),
	}
	user := types.Message{Role: types.RoleUser, Content: "对话片段：\n" + text}
	resp, err := client.ChatTextOnly(ctx, []types.Message{sys, user}, maxTok)
	if err != nil {
		return err
	}
	raw := strings.TrimSpace(resp.AssistantMsg.Content)
	raw = stripJSONFence(raw)
	items, err := parseMemoryItems(raw)
	if err != nil {
		return fmt.Errorf("memory extract parse: %w", err)
	}
	for _, it := range items {
		cat := normalizeCategory(it.Category)
		key := strings.TrimSpace(it.Key)
		content := strings.TrimSpace(it.Content)
		if key == "" || content == "" {
			continue
		}
		if len(key) > 200 || len(content) > 8000 {
			continue
		}
		if err := e.PG.WriteScoped(ctx, sessionID, scope.UserID, scope.AgentID, string(cat), key, content); err != nil {
			return err
		}
	}
	return nil
}

func (e *LLMExtractor) minLen() int {
	if e.MinTurnLen > 0 {
		return e.MinTurnLen
	}
	return 64
}

func summarizeTurn(turn []transcript.Message) string {
	var b strings.Builder
	for _, m := range turn {
		line := strings.TrimSpace(m.Content)
		if len(line) > 4000 {
			line = line[:4000] + "…(truncated)"
		}
		switch m.Role {
		case transcript.RoleUser:
			b.WriteString("用户: ")
			b.WriteString(line)
			b.WriteByte('\n')
		case transcript.RoleAssistant:
			b.WriteString("助手: ")
			b.WriteString(line)
			if len(m.ToolCalls) > 0 {
				b.WriteString(" [tool_calls]")
			}
			b.WriteByte('\n')
		case transcript.RoleTool:
			b.WriteString("工具结果: ")
			b.WriteString(line)
			b.WriteByte('\n')
		default:
			b.WriteString(string(m.Role))
			b.WriteString(": ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

type memItem struct {
	Category string `json:"category"`
	Key      string `json:"key"`
	Content  string `json:"content"`
}

func parseMemoryItems(raw string) ([]memItem, error) {
	i := strings.Index(raw, "[")
	j := strings.LastIndex(raw, "]")
	if i < 0 || j <= i {
		return nil, fmt.Errorf("no json array in model output")
	}
	raw = raw[i : j+1]
	var items []memItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, err
	}
	if len(items) > 5 {
		items = items[:5]
	}
	return items, nil
}

func normalizeCategory(s string) memory.Category {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "user":
		return memory.CategoryUser
	case "feedback":
		return memory.CategoryFeedback
	case "project":
		return memory.CategoryProject
	case "reference", "ref":
		return memory.CategoryRef
	default:
		return memory.CategoryRef
	}
}
