package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"ascentia-core/pkg/agent_core/identity"
)

// MockProvider is an in-memory Provider for tests and local harnesses.
type MockProvider struct {
	mu      sync.Mutex
	records []Record
	nextID  int64
}

func NewMockProvider() *MockProvider {
	return &MockProvider{records: make([]Record, 0)}
}

func scopeKey(s identity.TenantScope) string {
	return s.UserID + "\x00" + s.AgentID
}

func (m *MockProvider) Write(ctx context.Context, scope identity.TenantScope, w Write) error {
	_ = ctx
	if err := scope.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	sk := scopeKey(scope)
	for i := range m.records {
		r := &m.records[i]
		if r.Metadata == nil {
			continue
		}
		if r.Metadata["_scope"] != sk || r.Key != w.Key {
			continue
		}
		r.Category = w.Category
		r.Content = w.Content
		r.Metadata = cloneMeta(w.Metadata)
		r.Metadata["_scope"] = sk
		r.UpdatedAt = now
		return nil
	}
	m.nextID++
	id := fmt.Sprintf("m%d", m.nextID)
	meta := cloneMeta(w.Metadata)
	if meta == nil {
		meta = map[string]string{}
	}
	meta["_scope"] = sk
	m.records = append(m.records, Record{
		ID:        id,
		Category:  w.Category,
		Key:       w.Key,
		Content:   w.Content,
		Metadata:  meta,
		CreatedAt: now,
		UpdatedAt: now,
	})
	return nil
}

func cloneMeta(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if k == "_scope" {
			continue
		}
		out[k] = v
	}
	return out
}

func (m *MockProvider) Query(ctx context.Context, scope identity.TenantScope, q Query) ([]Record, error) {
	_ = ctx
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	sk := scopeKey(scope)
	var out []Record
	for _, r := range m.records {
		if r.Metadata == nil || r.Metadata["_scope"] != sk {
			continue
		}
		if len(q.Categories) > 0 {
			ok := false
			for _, c := range q.Categories {
				if r.Category == c {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		if len(q.Keys) > 0 {
			ok := false
			for _, k := range q.Keys {
				if r.Key == k {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		if q.Text != "" {
			if !strings.Contains(strings.ToLower(r.Key+r.Content), strings.ToLower(q.Text)) {
				continue
			}
		}
		out = append(out, r)
		if q.Limit > 0 && len(out) >= q.Limit {
			break
		}
	}
	return out, nil
}

// MockSideQuery is a trivial SideQuerySelector: substring match on query vs key/content.
type MockSideQuery struct {
	Inner *MockProvider
}

func (s *MockSideQuery) SelectForTurn(ctx context.Context, scope identity.TenantScope, query string, recentToolNames []string, limit int) ([]Record, error) {
	_ = recentToolNames
	if s.Inner == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	return s.Inner.Query(ctx, scope, Query{Text: query, Limit: limit})
}
