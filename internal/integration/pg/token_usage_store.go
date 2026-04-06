package pgintegration

import (
	"context"
	"fmt"
	"log"
	"time"

	"ascentia-core/internal/usage"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TokenUsageStore appends rows to overseer_token_usage (async insert by default).
type TokenUsageStore struct {
	pool *pgxpool.Pool
}

func NewTokenUsageStore(pool *pgxpool.Pool) *TokenUsageStore {
	if pool == nil {
		return nil
	}
	return &TokenUsageStore{pool: pool}
}

func (s *TokenUsageStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("token usage store not initialized")
	}
	ddl := `
CREATE TABLE IF NOT EXISTS overseer_token_usage (
  id BIGSERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  user_id TEXT NOT NULL DEFAULT '',
  agent_id TEXT NOT NULL DEFAULT '',
  operator_role TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  request_id TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL,
  channel TEXT NOT NULL,
  prompt_tokens INT NOT NULL DEFAULT 0,
  completion_tokens INT NOT NULL DEFAULT 0,
  total_tokens INT NOT NULL DEFAULT 0,
  provider_request_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_overseer_token_usage_created ON overseer_token_usage(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_overseer_token_usage_tenant_time ON overseer_token_usage(user_id, agent_id, created_at DESC);
`
	_, err := s.pool.Exec(ctx, ddl)
	return err
}

// RecordAsync implements usage.AsyncRecorder (non-blocking).
func (s *TokenUsageStore) RecordAsync(row usage.Row) {
	if s == nil || s.pool == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := s.pool.Exec(ctx, `
INSERT INTO overseer_token_usage(
  user_id, agent_id, operator_role, session_id, request_id,
  model, channel, prompt_tokens, completion_tokens, total_tokens, provider_request_id
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
`, row.UserID, row.AgentID, row.OperatorRole, row.SessionID, row.RequestID,
			row.Model, row.Channel, row.PromptTokens, row.CompletionTokens, row.TotalTokens, row.ProviderRequestID)
		if err != nil {
			log.Printf("[token_usage] insert failed: %v", err)
		}
	}()
}

var _ usage.AsyncRecorder = (*TokenUsageStore)(nil)
