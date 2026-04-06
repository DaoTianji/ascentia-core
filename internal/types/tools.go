package types

import (
	"context"
	"time"
)

// ToolDefinition is the schema that we expose to Claude-compatible
// function calling.
//
// We mirror the claude-code-local "custom" tool schema shape:
// - type: "custom"
// - name
// - description
// - input_schema: JSON schema
type ToolDefinition struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type NATSPublisher interface {
	Publish(subject string, payload []byte) error
}

type MemoryStore interface {
	Write(ctx context.Context, sessionID string, key string, content string) error
	Read(ctx context.Context, sessionID string, query string, limit int) ([]MemoryRecord, error)
}

type MemoryRecord struct {
	ID        int64
	SessionID string
	UserID    string
	AgentID   string
	Category  string
	Key       string
	Content   string
	CreatedAt time.Time
}
