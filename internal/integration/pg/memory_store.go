package pgintegration

import (
	"context"
	"fmt"
	"time"

	"ascentia-core/internal/types"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MemoryStore struct {
	pool *pgxpool.Pool
}

func NewMemoryStore(pool *pgxpool.Pool) *MemoryStore {
	return &MemoryStore{pool: pool}
}

func (s *MemoryStore) EnsureSchema(ctx context.Context) error {
	ddl := `
CREATE TABLE IF NOT EXISTS overseer_memories (
  id BIGSERIAL PRIMARY KEY,
  session_id TEXT NOT NULL,
  user_id TEXT NOT NULL DEFAULT '',
  agent_id TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL DEFAULT 'reference',
  key TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_overseer_memories_session_id ON overseer_memories(session_id);
CREATE INDEX IF NOT EXISTS idx_overseer_memories_key ON overseer_memories(key);
CREATE INDEX IF NOT EXISTS idx_overseer_memories_tenant ON overseer_memories(session_id, user_id, agent_id);
`
	if _, err := s.pool.Exec(ctx, ddl); err != nil {
		return err
	}
	// Idempotent migration from pre-tenant schema.
	_, _ = s.pool.Exec(ctx, `ALTER TABLE overseer_memories ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT ''`)
	_, _ = s.pool.Exec(ctx, `ALTER TABLE overseer_memories ADD COLUMN IF NOT EXISTS agent_id TEXT NOT NULL DEFAULT ''`)
	_, _ = s.pool.Exec(ctx, `ALTER TABLE overseer_memories ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'reference'`)
	_, _ = s.pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_overseer_memories_tenant ON overseer_memories(session_id, user_id, agent_id)`)
	// Dedupe: one logical row per (session, tenant, key) for LTM quality.
	_, _ = s.pool.Exec(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_overseer_memories_dedupe ON overseer_memories(session_id, user_id, agent_id, key)`)
	_, _ = s.pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_overseer_memories_user_agent_time ON overseer_memories(user_id, agent_id, created_at DESC)`)
	return nil
}

// WriteScoped upserts LTM scoped by session + tenant (same key updates content/category).
func (s *MemoryStore) WriteScoped(ctx context.Context, sessionID, userID, agentID, category, key, content string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("pg memory store not initialized")
	}
	if category == "" {
		category = "reference"
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO overseer_memories(session_id, user_id, agent_id, category, key, content)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (session_id, user_id, agent_id, key)
DO UPDATE SET category = EXCLUDED.category, content = EXCLUDED.content
`, sessionID, userID, agentID, category, key, content)
	return err
}

// ListRecentScoped returns the newest rows for a session + tenant (no text filter).
func (s *MemoryStore) ListRecentScoped(ctx context.Context, sessionID, userID, agentID string, limit int) ([]types.MemoryRecord, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("pg memory store not initialized")
	}
	if limit <= 0 || limit > 50 {
		limit = 15
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, session_id, user_id, agent_id, category, key, content, created_at
FROM overseer_memories
WHERE session_id = $1 AND user_id = $2 AND agent_id = $3
ORDER BY created_at DESC
LIMIT $4
`, sessionID, userID, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]types.MemoryRecord, 0, limit)
	for rows.Next() {
		var r types.MemoryRecord
		var created time.Time
		if err := rows.Scan(&r.ID, &r.SessionID, &r.UserID, &r.AgentID, &r.Category, &r.Key, &r.Content, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = created
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListRecentForTenant returns the newest rows for a user+agent across all chat sessions
// (LTM recall). Contrast with [ListRecentScoped], which is limited to one client session.
func (s *MemoryStore) ListRecentForTenant(ctx context.Context, userID, agentID string, limit int) ([]types.MemoryRecord, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("pg memory store not initialized")
	}
	if limit <= 0 || limit > 50 {
		limit = 15
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, session_id, user_id, agent_id, category, key, content, created_at
FROM overseer_memories
WHERE user_id = $1 AND agent_id = $2
ORDER BY created_at DESC
LIMIT $3
`, userID, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]types.MemoryRecord, 0, limit)
	for rows.Next() {
		var r types.MemoryRecord
		var created time.Time
		if err := rows.Scan(&r.ID, &r.SessionID, &r.UserID, &r.AgentID, &r.Category, &r.Key, &r.Content, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = created
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListDistinctSessionsForTenant returns session_ids that have at least one memory row.
func (s *MemoryStore) ListDistinctSessionsForTenant(ctx context.Context, userID, agentID string) ([]string, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("pg memory store not initialized")
	}
	rows, err := s.pool.Query(ctx, `
SELECT DISTINCT session_id
FROM overseer_memories
WHERE user_id = $1 AND agent_id = $2
ORDER BY session_id
`, userID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err != nil {
			return nil, err
		}
		out = append(out, sid)
	}
	return out, rows.Err()
}

// ListAllForSession returns up to limit rows for one session + tenant (stable key order).
func (s *MemoryStore) ListAllForSession(ctx context.Context, sessionID, userID, agentID string, limit int) ([]types.MemoryRecord, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("pg memory store not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 120
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, session_id, user_id, agent_id, category, key, content, created_at
FROM overseer_memories
WHERE session_id = $1 AND user_id = $2 AND agent_id = $3
ORDER BY key ASC, created_at ASC
LIMIT $4
`, sessionID, userID, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]types.MemoryRecord, 0, limit)
	for rows.Next() {
		var r types.MemoryRecord
		var created time.Time
		if err := rows.Scan(&r.ID, &r.SessionID, &r.UserID, &r.AgentID, &r.Category, &r.Key, &r.Content, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = created
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReplaceSessionMemories deletes all LTM rows for a session+tenant and inserts the given set (Dream / consolidate).
func (s *MemoryStore) ReplaceSessionMemories(ctx context.Context, sessionID, userID, agentID string, rows []struct {
	Category string
	Key      string
	Content  string
}) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("pg memory store not initialized")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
DELETE FROM overseer_memories
WHERE session_id = $1 AND user_id = $2 AND agent_id = $3
`, sessionID, userID, agentID); err != nil {
		return err
	}
	for _, r := range rows {
		cat := r.Category
		if cat == "" {
			cat = "reference"
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO overseer_memories(session_id, user_id, agent_id, category, key, content)
VALUES ($1,$2,$3,$4,$5,$6)
`, sessionID, userID, agentID, cat, r.Key, r.Content); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ReadScoped returns memories for session + tenant only (strict isolation).
func (s *MemoryStore) ReadScoped(ctx context.Context, sessionID, userID, agentID, query string, limit int) ([]types.MemoryRecord, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("pg memory store not initialized")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, session_id, user_id, agent_id, category, key, content, created_at
FROM overseer_memories
WHERE session_id = $1 AND user_id = $2 AND agent_id = $3
  AND (key = $4 OR content ILIKE '%' || $4 || '%')
ORDER BY created_at DESC
LIMIT $5
`, sessionID, userID, agentID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]types.MemoryRecord, 0, limit)
	for rows.Next() {
		var r types.MemoryRecord
		var created time.Time
		if err := rows.Scan(&r.ID, &r.SessionID, &r.UserID, &r.AgentID, &r.Category, &r.Key, &r.Content, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = created
		out = append(out, r)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

// ReadForTenant matches key or content across all sessions for this user+agent (LTM recall).
func (s *MemoryStore) ReadForTenant(ctx context.Context, userID, agentID, query string, limit int) ([]types.MemoryRecord, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("pg memory store not initialized")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, session_id, user_id, agent_id, category, key, content, created_at
FROM overseer_memories
WHERE user_id = $1 AND agent_id = $2
  AND (key = $3 OR content ILIKE '%' || $3 || '%')
ORDER BY created_at DESC
LIMIT $4
`, userID, agentID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]types.MemoryRecord, 0, limit)
	for rows.Next() {
		var r types.MemoryRecord
		var created time.Time
		if err := rows.Scan(&r.ID, &r.SessionID, &r.UserID, &r.AgentID, &r.Category, &r.Key, &r.Content, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = created
		out = append(out, r)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

// Write implements legacy MemoryStore (no tenant); uses empty user/agent.
func (s *MemoryStore) Write(ctx context.Context, sessionID string, key string, content string) error {
	return s.WriteScoped(ctx, sessionID, "", "", "reference", key, content)
}

// Read implements legacy MemoryStore (no tenant filter — only session + key/content match).
func (s *MemoryStore) Read(ctx context.Context, sessionID string, query string, limit int) ([]types.MemoryRecord, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("pg memory store not initialized")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, session_id, user_id, agent_id, category, key, content, created_at
FROM overseer_memories
WHERE session_id = $1
  AND (key = $2 OR content ILIKE '%' || $2 || '%')
ORDER BY created_at DESC
LIMIT $3
`, sessionID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]types.MemoryRecord, 0, limit)
	for rows.Next() {
		var r types.MemoryRecord
		var created time.Time
		if err := rows.Scan(&r.ID, &r.SessionID, &r.UserID, &r.AgentID, &r.Category, &r.Key, &r.Content, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = created
		out = append(out, r)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}
