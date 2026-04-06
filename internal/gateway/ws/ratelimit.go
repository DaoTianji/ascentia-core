package ws

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// messageRateLimiter enforces a simple per-IP sliding window (minute) on message count.
type messageRateLimiter struct {
	mu        sync.Mutex
	perIP     map[string][]time.Time
	maxPerMin int
}

func newMessageRateLimiter(maxPerMin int) *messageRateLimiter {
	if maxPerMin <= 0 {
		return nil
	}
	return &messageRateLimiter{
		perIP:     make(map[string][]time.Time),
		maxPerMin: maxPerMin,
	}
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// allow returns false if this IP exceeded the per-minute budget.
func (l *messageRateLimiter) allow(ip string) bool {
	if l == nil {
		return true
	}
	now := time.Now()
	cutoff := now.Add(-time.Minute)

	l.mu.Lock()
	defer l.mu.Unlock()

	ts := l.perIP[ip]
	kept := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.maxPerMin {
		l.perIP[ip] = kept
		return false
	}
	kept = append(kept, now)
	l.perIP[ip] = kept
	return true
}
