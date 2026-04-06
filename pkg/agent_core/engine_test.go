package agent_core

import (
	"context"
	"strings"
	"testing"

	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/llm"
	"ascentia-core/pkg/agent_core/planner"
	"ascentia-core/pkg/agent_core/prompt"
	"ascentia-core/pkg/agent_core/tools"
	"ascentia-core/pkg/agent_core/transcript"
)

func TestEngineRunWithPlannerTool(t *testing.T) {
	store := planner.NewMemoryStore()
	cat := &tools.MockCatalog{
		BySkill: map[string][]tools.Tool{}, // no skill tools
	}
	mc := &llm.MockClient{
		Queue: []llm.CompleteResponse{
			{
				Assistant: transcript.Message{
					Role: transcript.RoleAssistant,
					ToolCalls: []transcript.ToolCall{
						{
							ID:        "t1",
							Type:      "function",
							Name:      planner.ToolUpdateTodo,
							Arguments: `{"tasks":[{"id":"1","title":"Ship feature","status":"in_progress","order":0}]}`,
						},
					},
				},
			},
			{
				Assistant: transcript.Message{
					Role:    transcript.RoleAssistant,
					Content: "planned",
				},
			},
		},
	}
	eng := NewEngine()
	res, err := eng.Run(context.Background(), RunRequest{
		Scope:           identity.TenantScope{UserID: "u1", AgentID: "ag1"},
		SessionID:       "sess",
		RequestID:       "req1",
		UserMessage:     transcript.Message{Role: transcript.RoleUser, Content: "help me plan"},
		AllowedSkillIDs: nil,
		TodoStore:       store,
		Assembler:       prompt.DefaultAssembler{},
		Prompt: prompt.AssembleInput{
			CoreRules: "You are a test agent.",
		},
		Catalog: cat,
		Model:   mc,
		Options: RunOptions{MaxToolTurns: 8, MaxConsecutiveToolFailures: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalText != "planned" {
		t.Fatalf("final=%q", res.FinalText)
	}
	tasks := store.List("sess")
	if len(tasks) != 1 || tasks[0].Title != "Ship feature" {
		t.Fatalf("tasks=%v", tasks)
	}
	// System prompt should have included todo focus fragment in assembled messages
	sys := res.Messages[0].Content
	if !strings.Contains(sys, "Current focus") {
		t.Fatalf("missing focus in system: %q", sys)
	}
}

func TestEngineRunStreamDemux(t *testing.T) {
	mc := &llm.MockClient{
		StreamChunks: []string{"Hi <thinking>secret</thinking> there"},
	}
	var th, tx strings.Builder
	sink := &sinkRecorder{t: &th, x: &tx}
	eng := NewEngine()
	err := eng.RunStream(context.Background(), RunRequest{
		Scope:       identity.TenantScope{UserID: "u", AgentID: "a"},
		SessionID:   "s",
		UserMessage: transcript.Message{Role: transcript.RoleUser, Content: "x"},
		Assembler:   prompt.DefaultAssembler{},
		Prompt:      prompt.AssembleInput{CoreRules: "c"},
		Catalog:     &tools.MockCatalog{},
		Model:       mc,
	}, sink)
	if err != nil {
		t.Fatal(err)
	}
	if th.String() != "secret" || !strings.Contains(tx.String(), "Hi") || !strings.Contains(tx.String(), "there") {
		t.Fatalf("thinking=%q text=%q", th.String(), tx.String())
	}
}

type sinkRecorder struct {
	t, x *strings.Builder
}

func (s *sinkRecorder) OnThinking(delta string) { s.t.WriteString(delta) }
func (s *sinkRecorder) OnText(delta string)     { s.x.WriteString(delta) }
func (s *sinkRecorder) OnError(err error)       { _ = err }
