package memory

import (
	"context"
	"sort"
	"strconv"
	"strings"

	pgintegration "ascentia-core/internal/integration/pg"
	"ascentia-core/internal/types"
	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/memory"
)

// SideQuery implements memory.SideQuerySelector using PostgreSQL:
// merges keyword hits with recent writes for better recall coverage.
type SideQuery struct {
	PG        *pgintegration.MemoryStore
	SessionID string
	// RecentExtra is how many recent rows to union (default 12).
	RecentExtra int
}

func (s *SideQuery) SelectForTurn(ctx context.Context, scope identity.TenantScope, query string, _ []string, limit int) ([]memory.Record, error) {
	if s.PG == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	recentN := s.RecentExtra
	if recentN <= 0 {
		recentN = 12
	}

	q := strings.TrimSpace(query)
	byID := make(map[int64]types.MemoryRecord)

	if q != "" {
		match, err := s.PG.ReadForTenant(ctx, scope.UserID, scope.AgentID, q, limit+8)
		if err != nil {
			return nil, err
		}
		for _, r := range match {
			byID[r.ID] = r
		}
	}

	recent, err := s.PG.ListRecentForTenant(ctx, scope.UserID, scope.AgentID, recentN)
	if err != nil {
		return nil, err
	}
	for _, r := range recent {
		byID[r.ID] = r
	}

	merged := make([]types.MemoryRecord, 0, len(byID))
	for _, r := range byID {
		merged = append(merged, r)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].CreatedAt.After(merged[j].CreatedAt)
	})
	if len(merged) > limit {
		merged = merged[:limit]
	}

	out := make([]memory.Record, 0, len(merged))
	for _, r := range merged {
		out = append(out, memory.Record{
			ID:        strconv.FormatInt(r.ID, 10),
			Category:  memory.Category(r.Category),
			Key:       r.Key,
			Content:   r.Content,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}
