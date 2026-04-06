package profile

import (
	"context"

	"ascentia-core/pkg/agent_core/identity"
)

// Provider supplies persona / agent-facing profile text for prompt assembly.
type Provider interface {
	Persona(ctx context.Context, scope identity.TenantScope) (string, error)
}

// MockProvider returns a fixed persona string.
type MockProvider struct {
	Text string
	Err  error
}

func (m MockProvider) Persona(ctx context.Context, scope identity.TenantScope) (string, error) {
	_ = ctx
	_ = scope
	if m.Err != nil {
		return "", m.Err
	}
	return m.Text, nil
}
