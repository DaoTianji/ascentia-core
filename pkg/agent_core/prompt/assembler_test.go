package prompt

import (
	"context"
	"strings"
	"testing"

	"ascentia-core/pkg/agent_core/identity"
)

func TestDefaultAssemblerOrderAndTodoLast(t *testing.T) {
	var a DefaultAssembler
	msgs, err := a.BuildSystemMessages(context.Background(), AssembleInput{
		Scope:        identity.TenantScope{UserID: "u", AgentID: "a"},
		CoreRules:    "CORE",
		Persona:      "PERSONA",
		ProjectRules: []string{"R1"},
		Temporal:     "TIME",
		MemoryRecall: "MEM",
		TodoFocus:    "TODO_BLOCK",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Role != "system" {
		t.Fatalf("%+v", msgs)
	}
	body := msgs[0].Content
	if !strings.HasPrefix(body, "## Core") {
		t.Fatalf("expected Core first: %q", body)
	}
	if !strings.HasSuffix(strings.TrimSpace(body), "TODO_BLOCK") {
		t.Fatalf("todo focus should be last: %q", body)
	}
}
