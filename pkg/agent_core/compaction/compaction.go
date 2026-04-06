package compaction

import (
	"context"

	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/transcript"
)

// TokenEstimator estimates total tokens for a message slice (caller-defined).
type TokenEstimator func(msgs []transcript.Message) int

// Report describes compaction outcome.
type Report struct {
	Applied bool
	Reason  string
}

// Compactor optionally shrinks transcript before model calls.
type Compactor interface {
	MaybeCompact(ctx context.Context, scope identity.TenantScope, sessionID string, msgs []transcript.Message, estTokens int) ([]transcript.Message, Report, error)
}

// ToolResultPruner rewrites tool result messages to save context.
type ToolResultPruner interface {
	Prune(msg transcript.Message) transcript.Message
}

// NoopCompactor does nothing.
type NoopCompactor struct{}

func (NoopCompactor) MaybeCompact(ctx context.Context, scope identity.TenantScope, sessionID string, msgs []transcript.Message, estTokens int) ([]transcript.Message, Report, error) {
	_ = ctx
	_ = scope
	_ = sessionID
	_ = estTokens
	return msgs, Report{}, nil
}

// NoopToolPruner returns the message unchanged.
type NoopToolPruner struct{}

func (NoopToolPruner) Prune(msg transcript.Message) transcript.Message { return msg }
