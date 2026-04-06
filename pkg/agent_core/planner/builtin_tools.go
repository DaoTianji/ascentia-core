package planner

import (
	"context"
	"encoding/json"
	"fmt"

	"ascentia-core/pkg/agent_core/tools"
)

const (
	ToolUpdateTodo   = "update_todo"
	ToolMarkTaskDone = "mark_task_done"
)

// BuiltinToolNames returns planner tool names for allowlist merging.
func BuiltinToolNames() []string {
	return []string{ToolUpdateTodo, ToolMarkTaskDone}
}

// BuiltinTools returns update_todo and mark_task_done bound to store.
func BuiltinTools(store Store) []tools.Tool {
	if store == nil {
		return nil
	}
	return []tools.Tool{updateTodoTool{store: store}, markDoneTool{store: store}}
}

type updateTodoTool struct {
	store Store
}

func (updateTodoTool) Name() string { return ToolUpdateTodo }

func (updateTodoTool) Definition() tools.Definition {
	return tools.Definition{
		Type:        "function",
		Name:        ToolUpdateTodo,
		Description: "Replace the entire todo list for this session. Set exactly one task to in_progress when starting work.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tasks": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":        map[string]any{"type": "string"},
							"parent_id": map[string]any{"type": "string"},
							"title":     map[string]any{"type": "string"},
							"status": map[string]any{
								"type": "string",
								"enum": []string{
									string(StatusPending),
									string(StatusInProgress),
									string(StatusDone),
									string(StatusCancelled),
								},
							},
							"order": map[string]any{"type": "integer"},
						},
						"required": []string{"id", "title", "status"},
					},
				},
			},
			"required": []string{"tasks"},
		},
	}
}

func (t updateTodoTool) Execute(ctx context.Context, tcx tools.Context, input json.RawMessage) (string, error) {
	_ = ctx
	var body struct {
		Tasks []Task `json:"tasks"`
	}
	if err := json.Unmarshal(input, &body); err != nil {
		return "", fmt.Errorf("update_todo: %w", err)
	}
	t.store.ReplaceTasks(tcx.SessionID, body.Tasks)
	return fmt.Sprintf("ok: %d tasks stored", len(body.Tasks)), nil
}

type markDoneTool struct {
	store Store
}

func (markDoneTool) Name() string { return ToolMarkTaskDone }

func (markDoneTool) Definition() tools.Definition {
	return tools.Definition{
		Type:        "function",
		Name:        ToolMarkTaskDone,
		Description: "Mark a todo item as done by id.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string"},
			},
			"required": []string{"id"},
		},
	}
}

func (t markDoneTool) Execute(ctx context.Context, tcx tools.Context, input json.RawMessage) (string, error) {
	_ = ctx
	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(input, &body); err != nil {
		return "", fmt.Errorf("mark_task_done: %w", err)
	}
	if err := t.store.MarkDone(tcx.SessionID, body.ID); err != nil {
		return "", err
	}
	return "ok", nil
}
