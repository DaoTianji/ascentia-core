package planner

import (
	"fmt"
	"sync"
)

// Store holds session-scoped todo lists.
type Store interface {
	ReplaceTasks(sessionID string, tasks []Task)
	List(sessionID string) []Task
	MarkDone(sessionID, id string) error
}

// MemoryStore is a thread-safe in-memory Store.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string][]Task
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: make(map[string][]Task)}
}

func (m *MemoryStore) ReplaceTasks(sessionID string, tasks []Task) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]Task, len(tasks))
	copy(cp, tasks)
	m.sessions[sessionID] = cp
}

func (m *MemoryStore) List(sessionID string) []Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.sessions[sessionID]
	out := make([]Task, len(src))
	copy(out, src)
	return out
}

func (m *MemoryStore) MarkDone(sessionID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tasks, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("planner: no tasks for session")
	}
	for i := range tasks {
		if tasks[i].ID == id {
			tasks[i].Status = StatusDone
			m.sessions[sessionID] = tasks
			return nil
		}
	}
	return fmt.Errorf("planner: task %q not found", id)
}
