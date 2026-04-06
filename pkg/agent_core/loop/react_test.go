package loop

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/llm"
	"ascentia-core/pkg/agent_core/tools"
	"ascentia-core/pkg/agent_core/transcript"
)

func TestRunToolThenText(t *testing.T) {
	echo := tools.FuncTool{
		N: "echo",
		D: tools.Definition{
			Description: "echo",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"msg": map[string]any{"type": "string"},
				},
				"required": []string{"msg"},
			},
		},
		F: func(ctx context.Context, tcx tools.Context, raw json.RawMessage) (string, error) {
			_ = ctx
			_ = tcx
			var in struct {
				Msg string `json:"msg"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", err
			}
			return in.Msg, nil
		},
	}
	reg, err := tools.NewRegistry([]tools.Tool{echo})
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]struct{}{"echo": {}}
	tcx := tools.Context{
		Scope:     identity.TenantScope{UserID: "u", AgentID: "a"},
		SessionID: "s",
		RequestID: "r",
		Allowed:   allowed,
	}
	mc := &llm.MockClient{
		Queue: []llm.CompleteResponse{
			{
				Assistant: transcript.Message{
					Role:    transcript.RoleAssistant,
					Content: "",
					ToolCalls: []transcript.ToolCall{
						{ID: "1", Type: "function", Name: "echo", Arguments: `{"msg":"hi"}`},
					},
				},
			},
			{
				Assistant: transcript.Message{
					Role:    transcript.RoleAssistant,
					Content: "done",
				},
			},
		},
	}
	msgs := []transcript.Message{
		{Role: transcript.RoleSystem, Content: "sys"},
		{Role: transcript.RoleUser, Content: "go"},
	}
	out, text, err := Run(context.Background(), tcx, msgs, reg, mc, Options{MaxTurns: 8, MaxConsecutiveToolFailures: 3})
	if err != nil {
		t.Fatal(err)
	}
	if text != "done" {
		t.Fatalf("text=%q", text)
	}
	if len(out) < 4 {
		t.Fatalf("short out: %d", len(out))
	}
}

func TestRunCircuitBreaker(t *testing.T) {
	boom := tools.FuncTool{
		N: "fail_tool",
		D: tools.Definition{Description: "fails", Parameters: map[string]any{"type": "object"}},
		F: func(ctx context.Context, tcx tools.Context, raw json.RawMessage) (string, error) {
			return "", errors.New("boom")
		},
	}
	reg, err := tools.NewRegistry([]tools.Tool{boom})
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]struct{}{"fail_tool": {}}
	tcx := tools.Context{
		Scope:   identity.TenantScope{UserID: "u", AgentID: "a"},
		Allowed: allowed,
	}
	mc := &llm.MockClient{}
	for range 5 {
		mc.Queue = append(mc.Queue, llm.CompleteResponse{
			Assistant: transcript.Message{
				Role: transcript.RoleAssistant,
				ToolCalls: []transcript.ToolCall{
					{ID: "x", Type: "function", Name: "fail_tool", Arguments: `{}`},
				},
			},
		})
	}
	msgs := []transcript.Message{{Role: transcript.RoleUser, Content: "x"}}
	_, _, err = Run(context.Background(), tcx, msgs, reg, mc, Options{MaxTurns: 10, MaxConsecutiveToolFailures: 2})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("want ErrCircuitOpen got %v", err)
	}
}
