package compaction

import (
	"context"
	"strings"
	"testing"

	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/transcript"
)

func TestRoughTokenEstimatorPositive(t *testing.T) {
	msgs := []transcript.Message{
		{Role: transcript.RoleSystem, Content: strings.Repeat("a", 300)},
		{Role: transcript.RoleUser, Content: "hi"},
	}
	n := RoughTokenEstimator(msgs)
	if n <= 0 {
		t.Fatalf("expected positive estimate, got %d", n)
	}
}

func TestThresholdCompactorDropsOldest(t *testing.T) {
	var mid []transcript.Message
	for i := 0; i < 20; i++ {
		mid = append(mid, transcript.Message{Role: transcript.RoleUser, Content: strings.Repeat("x", 4000)})
		mid = append(mid, transcript.Message{Role: transcript.RoleAssistant, Content: strings.Repeat("y", 4000)})
	}
	last := transcript.Message{Role: transcript.RoleUser, Content: "final question"}
	msgs := append(append([]transcript.Message{
		{Role: transcript.RoleSystem, Content: "sys"},
	}, mid...), last)

	c := NewThresholdCompactor(8000, 4)
	tok := RoughTokenEstimator(msgs)
	out, rep, err := c.MaybeCompact(context.Background(), identity.TenantScope{UserID: "u", AgentID: "a"}, "s", msgs, tok)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Applied {
		t.Fatalf("expected compaction, report=%+v", rep)
	}
	if len(out) >= len(msgs) {
		t.Fatalf("expected shorter transcript")
	}
	if out[len(out)-1].Content != "final question" {
		t.Fatalf("last message not preserved")
	}
	if out[0].Role != transcript.RoleSystem || out[0].Content != "sys" {
		t.Fatalf("system prefix broken")
	}
}

func TestConsumeBlockFromFrontToolBatch(t *testing.T) {
	mid := []transcript.Message{
		{Role: transcript.RoleAssistant, ToolCalls: []transcript.ToolCall{{ID: "c1", Name: "x", Arguments: "{}"}}},
		{Role: transcript.RoleTool, ToolCallID: "c1", Content: "result"},
		{Role: transcript.RoleUser, Content: "next"},
	}
	n, rest := consumeBlockFromFront(mid)
	if n != 2 {
		t.Fatalf("expected remove 2, got %d", n)
	}
	if len(rest) != 1 || rest[0].Content != "next" {
		t.Fatalf("unexpected rest: %+v", rest)
	}
}

func TestTruncateToolPruner(t *testing.T) {
	p := TruncateToolPruner{MaxRunes: 10, Ellipsis: "…"}
	long := strings.Repeat("あ", 20) // 20 runes
	out := p.Prune(transcript.Message{Role: transcript.RoleTool, Content: long})
	if utf8Count(out.Content) > 15 {
		t.Fatalf("content too long: %q", out.Content)
	}
	u := p.Prune(transcript.Message{Role: transcript.RoleUser, Content: long})
	if u.Content != long {
		t.Fatalf("user message should not truncate")
	}
}

func utf8Count(s string) int {
	return len([]rune(s))
}
