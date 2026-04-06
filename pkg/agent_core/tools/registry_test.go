package tools

import (
	"context"
	"encoding/json"
	"testing"

	"ascentia-core/pkg/agent_core/identity"
)

func TestRegistryAllowlist(t *testing.T) {
	ft := FuncTool{
		N: "x",
		D: Definition{Parameters: map[string]any{"type": "object"}},
		F: func(ctx context.Context, tcx Context, raw json.RawMessage) (string, error) {
			return "ok", nil
		},
	}
	reg, err := NewRegistry([]Tool{ft})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tcx := Context{
		Scope:   identity.TenantScope{UserID: "u", AgentID: "a"},
		Allowed: map[string]struct{}{"y": {}},
	}
	_, err = reg.Execute(ctx, tcx, "x", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected allowlist denial")
	}
	tcx.Allowed["x"] = struct{}{}
	s, err := reg.Execute(ctx, tcx, "x", json.RawMessage(`{}`))
	if err != nil || s != "ok" {
		t.Fatalf("%q %v", s, err)
	}
}
