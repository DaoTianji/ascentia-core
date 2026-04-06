package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pgintegration "ascentia-core/internal/integration/pg"
	"ascentia-core/pkg/agent_core/tools"
)

type ReadMemoryTool struct {
	PG *pgintegration.MemoryStore
}

func NewReadMemoryTool(pg *pgintegration.MemoryStore) tools.Tool {
	return &ReadMemoryTool{PG: pg}
}

func (ReadMemoryTool) Name() string { return "ReadMemory" }

func (t ReadMemoryTool) Definition() tools.Definition {
	return tools.Definition{
		Type: "function",
		Name: "ReadMemory",
		Description: strings.Join([]string{
			"Retrieve long-term memory for this user and assistant across all chat sessions",
			"(same agent_id), not only the current conversation.",
			"Query matches key exactly or searches within content (case-insensitive).",
		}, " "),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
				"limit": map[string]any{"type": "integer", "description": "1-20, default 10"},
			},
			"required": []string{"query"},
		},
	}
}

func (t *ReadMemoryTool) Execute(ctx context.Context, tcx tools.Context, input json.RawMessage) (string, error) {
	if t.PG == nil {
		return "", fmt.Errorf("memory store not available")
	}
	var in struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return "", fmt.Errorf("query required")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	rows, err := t.PG.ReadForTenant(ctx, tcx.Scope.UserID, tcx.Scope.AgentID, q, limit)
	if err != nil {
		return "", err
	}
	type item struct {
		ID        int64  `json:"id"`
		Category  string `json:"category"`
		Key       string `json:"key"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
	}
	out := make([]item, 0, len(rows))
	for _, r := range rows {
		out = append(out, item{
			ID:        r.ID,
			Category:  r.Category,
			Key:       r.Key,
			Content:   r.Content,
			CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}
