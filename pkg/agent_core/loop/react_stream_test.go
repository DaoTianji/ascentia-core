package loop

import (
	"context"
	"testing"

	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/llm"
	"ascentia-core/pkg/agent_core/tools"
	"ascentia-core/pkg/agent_core/transcript"
)

func TestRunStreamedMatchesRun(t *testing.T) {
	reg, _ := tools.NewRegistry(nil)
	mc := &llm.MockClient{
		Queue: []llm.CompleteResponse{
			{Assistant: transcript.Message{Role: transcript.RoleAssistant, Content: "hello"}},
		},
	}
	tcx := tools.Context{Scope: identity.TenantScope{UserID: "u", AgentID: "a"}, Allowed: nil}
	var deltas string
	out, text, err := RunStreamed(context.Background(), tcx,
		[]transcript.Message{{Role: transcript.RoleUser, Content: "hi"}},
		reg, mc, Options{MaxTurns: 4, MaxConsecutiveToolFailures: 2},
		StreamHooks{OnTextDelta: func(s string) { deltas += s }},
	)
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello" || deltas != "hello" || len(out) != 2 {
		t.Fatalf("text=%q deltas=%q n=%d", text, deltas, len(out))
	}
}
