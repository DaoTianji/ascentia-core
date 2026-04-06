package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"sync"

	"ascentia-core/internal/types"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	// When Redis is nil, we fall back to in-memory mode (useful for tests/local).
	Redis *redis.Client

	mu       sync.Mutex
	sessions map[string][]types.Message

	// MaxMessages bounds the number of chat messages we keep in STM.
	// We drop oldest first (claude-code style bounded growth).
	MaxMessages int

	// TTL bounds the STM lifetime in Redis.
	TTL time.Duration
}

func NewStore() *Store {
	return &Store{
		sessions:    make(map[string][]types.Message),
		MaxMessages: 60,
		TTL:         24 * time.Hour,
	}
}

func (s *Store) Get(sessionID string) []types.Message {
	if s.Redis != nil {
		return s.getRedis(sessionID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	msgs := s.sessions[sessionID]
	// Return a shallow copy so caller can't mutate internal slice.
	out := make([]types.Message, len(msgs))
	copy(out, msgs)
	return out
}

func (s *Store) Append(sessionID string, msgs ...types.Message) {
	if s.Redis != nil {
		_ = s.appendRedis(sessionID, msgs...)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[sessionID] = append(s.sessions[sessionID], msgs...)
	s.trimLocked(sessionID)
}

func (s *Store) Replace(sessionID string, msgs []types.Message) {
	if s.Redis != nil {
		_ = s.replaceRedis(sessionID, msgs)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[sessionID] = msgs
	s.trimLocked(sessionID)
}

func (s *Store) trimLocked(sessionID string) {
	max := s.MaxMessages
	if max <= 0 {
		return
	}
	msgs := s.sessions[sessionID]
	if len(msgs) <= max {
		return
	}
	// Drop oldest first.
	start := len(msgs) - max
	s.sessions[sessionID] = msgs[start:]
}

func (s *Store) key(sessionID string) string {
	return "overseer:session:" + sessionID + ":messages"
}

func (s *Store) getRedis(sessionID string) []types.Message {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	k := s.key(sessionID)
	raw, err := s.Redis.LRange(ctx, k, 0, -1).Result()
	if err != nil {
		return nil
	}

	out := make([]types.Message, 0, len(raw))
	for _, item := range raw {
		var m types.Message
		if err := json.Unmarshal([]byte(item), &m); err != nil {
			// If corrupted, skip this entry (fail-soft).
			continue
		}
		out = append(out, m)
	}
	return out
}

func (s *Store) appendRedis(sessionID string, msgs ...types.Message) error {
	if s.Redis == nil {
		return fmt.Errorf("redis client is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	k := s.key(sessionID)

	pipe := s.Redis.Pipeline()
	for _, m := range msgs {
		b, err := json.Marshal(m)
		if err != nil {
			continue
		}
		pipe.RPush(ctx, k, string(b))
	}

	// Trim to last MaxMessages.
	if s.MaxMessages > 0 {
		pipe.LTrim(ctx, k, int64(-s.MaxMessages), -1)
	}
	if s.TTL > 0 {
		pipe.Expire(ctx, k, s.TTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) replaceRedis(sessionID string, msgs []types.Message) error {
	if s.Redis == nil {
		return fmt.Errorf("redis client is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	k := s.key(sessionID)

	pipe := s.Redis.Pipeline()
	pipe.Del(ctx, k)
	for _, m := range msgs {
		b, err := json.Marshal(m)
		if err != nil {
			continue
		}
		pipe.RPush(ctx, k, string(b))
	}
	if s.MaxMessages > 0 {
		pipe.LTrim(ctx, k, int64(-s.MaxMessages), -1)
	}
	if s.TTL > 0 {
		pipe.Expire(ctx, k, s.TTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}
