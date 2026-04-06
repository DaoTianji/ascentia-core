package memory

import (
	"context"
	"time"

	"ascentia-core/pkg/agent_core/identity"
)

// Category aligns with Claude Code semantic memory taxonomy.
type Category string

const (
	CategoryUser     Category = "user"
	CategoryFeedback Category = "feedback"
	CategoryProject  Category = "project"
	CategoryRef      Category = "reference"
)

// Record is a durable memory row scoped by tenant (storage layer enforces isolation).
type Record struct {
	ID        string
	Category  Category
	Key       string
	Content   string
	Metadata  map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Write describes a new or updated memory payload.
type Write struct {
	Category Category
	Key      string
	Content  string
	Metadata map[string]string
}

// Query filters memory retrieval.
type Query struct {
	Text       string
	Keys       []string
	Categories []Category
	Limit      int
}

// Provider persists and queries tenant-scoped memories.
type Provider interface {
	Write(ctx context.Context, scope identity.TenantScope, w Write) error
	Query(ctx context.Context, scope identity.TenantScope, q Query) ([]Record, error)
}

// SideQuerySelector selects a small set of relevant memories before the main completion.
type SideQuerySelector interface {
	SelectForTurn(ctx context.Context, scope identity.TenantScope, query string, recentToolNames []string, limit int) ([]Record, error)
}
