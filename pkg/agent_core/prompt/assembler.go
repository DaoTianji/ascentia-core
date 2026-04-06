package prompt

import (
	"context"
	"strings"

	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/transcript"
)

// AssembleInput carries modular prompt pieces. TodoFocus and MemoryRecall are
// usually injected by the engine (planner focus + side-query).
type AssembleInput struct {
	Scope     identity.TenantScope
	SessionID string

	CoreRules    string
	Persona      string
	ProjectRules []string
	Temporal     string

	MemoryRecall string
	TodoFocus    string
}

// Assembler builds system messages from fragments.
type Assembler interface {
	BuildSystemMessages(ctx context.Context, in AssembleInput) ([]transcript.Message, error)
}

// DefaultAssembler concatenates non-empty sections in stable order.
// TodoFocus is placed last so it remains salient (recency bias for instructions).
type DefaultAssembler struct{}

func (DefaultAssembler) BuildSystemMessages(ctx context.Context, in AssembleInput) ([]transcript.Message, error) {
	_ = ctx
	_ = in.Scope
	_ = in.SessionID
	var b strings.Builder
	add := func(title, body string) {
		body = strings.TrimSpace(body)
		if body == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		if title != "" {
			b.WriteString("## ")
			b.WriteString(title)
			b.WriteString("\n")
		}
		b.WriteString(body)
	}
	add("Core", in.CoreRules)
	add("Persona", in.Persona)
	if len(in.ProjectRules) > 0 {
		joined := strings.TrimSpace(strings.Join(in.ProjectRules, "\n"))
		if joined != "" {
			add("Project rules", joined)
		}
	}
	add("Temporal context", in.Temporal)
	add("Memory recall", in.MemoryRecall)
	add("", in.TodoFocus)

	content := strings.TrimSpace(b.String())
	if content == "" {
		content = "You are a helpful assistant."
	}
	return []transcript.Message{{Role: transcript.RoleSystem, Content: content}}, nil
}
