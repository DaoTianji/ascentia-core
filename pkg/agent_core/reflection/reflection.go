package reflection

import (
	"context"

	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/transcript"
)

// PostTurnExtractor runs after a successful user-visible turn (caller may async).
type PostTurnExtractor interface {
	AfterTurn(ctx context.Context, scope identity.TenantScope, sessionID string, turn []transcript.Message) error
}

// ConsolidationOptions configures cross-session memory maintenance.
type ConsolidationOptions struct {
	Label string
	// SessionID limits consolidation to one chat session; empty means all sessions for the tenant.
	SessionID string
}

// Consolidator performs dream-style consolidation across sessions.
type Consolidator interface {
	Consolidate(ctx context.Context, scope identity.TenantScope, opts ConsolidationOptions) error
}

// NoopPostTurn is a no-op extractor.
type NoopPostTurn struct{}

func (NoopPostTurn) AfterTurn(ctx context.Context, scope identity.TenantScope, sessionID string, turn []transcript.Message) error {
	_ = ctx
	_ = scope
	_ = sessionID
	_ = turn
	return nil
}

// NoopConsolidator is a no-op consolidator.
type NoopConsolidator struct{}

func (NoopConsolidator) Consolidate(ctx context.Context, scope identity.TenantScope, opts ConsolidationOptions) error {
	_ = ctx
	_ = scope
	_ = opts
	return nil
}
