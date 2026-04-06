package memory

import (
	"context"
	"testing"

	"ascentia-core/pkg/agent_core/identity"
)

func TestMockProviderWriteQueryScoped(t *testing.T) {
	m := NewMockProvider()
	scopeA := identity.TenantScope{UserID: "a", AgentID: "1"}
	scopeB := identity.TenantScope{UserID: "b", AgentID: "1"}
	ctx := context.Background()
	_ = m.Write(ctx, scopeA, Write{Category: CategoryUser, Key: "k", Content: "alpha"})
	_ = m.Write(ctx, scopeB, Write{Category: CategoryUser, Key: "k", Content: "beta"})
	ra, err := m.Query(ctx, scopeA, Query{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(ra) != 1 || ra[0].Content != "alpha" {
		t.Fatalf("A: %+v", ra)
	}
	rb, err := m.Query(ctx, scopeB, Query{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rb) != 1 || rb[0].Content != "beta" {
		t.Fatalf("B: %+v", rb)
	}
}
