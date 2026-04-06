package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	pgintegration "ascentia-core/internal/integration/pg"
	"ascentia-core/pkg/agent_core/memory"
	"ascentia-core/pkg/agent_core/tools"
)

type WriteMemoryTool struct {
	PG *pgintegration.MemoryStore
}

func NewWriteMemoryTool(pg *pgintegration.MemoryStore) tools.Tool {
	return &WriteMemoryTool{PG: pg}
}

func (WriteMemoryTool) Name() string { return "WriteMemory" }

func (t WriteMemoryTool) Definition() tools.Definition {
	return tools.Definition{
		Type: "function",
		Name: "WriteMemory",
		Description: strings.Join([]string{
			"Persist long-term memory for this session and tenant.",
			"Optional category: user, feedback, project, reference (default reference).",
			"Do not store secrets (API keys, tokens, passwords).",
		}, " "),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"key":      map[string]any{"type": "string"},
				"content":  map[string]any{"type": "string"},
				"category": map[string]any{"type": "string", "description": "user|feedback|project|reference"},
			},
			"required": []string{"key", "content"},
		},
	}
}

func (t *WriteMemoryTool) Execute(ctx context.Context, tcx tools.Context, input json.RawMessage) (string, error) {
	if t.PG == nil {
		return "", fmt.Errorf("memory store not available")
	}
	var in struct {
		Key      string `json:"key"`
		Content  string `json:"content"`
		Category string `json:"category"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	key := strings.TrimSpace(in.Key)
	content := strings.TrimSpace(in.Content)
	if key == "" || content == "" {
		return "", fmt.Errorf("key and content required")
	}
	if len(key) > 200 || len(content) > 8000 {
		return "", fmt.Errorf("key or content too long")
	}
	cat := strings.TrimSpace(in.Category)
	if cat == "" {
		cat = string(memory.CategoryRef)
	}
	// Normalize to known taxonomy when possible.
	switch memory.Category(cat) {
	case memory.CategoryUser, memory.CategoryFeedback, memory.CategoryProject, memory.CategoryRef:
	default:
		cat = string(memory.CategoryRef)
	}
	if err := t.PG.WriteScoped(ctx, tcx.SessionID, tcx.Scope.UserID, tcx.Scope.AgentID, cat, key, content); err != nil {
		return "", err
	}
	return fmt.Sprintf("Memory written: key=%q category=%s", key, cat), nil
}
