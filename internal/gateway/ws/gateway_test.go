package ws

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascentia-core/internal/runtime"
	"ascentia-core/pkg/agent_core/identity"
)

func TestNewOriginChecker_AllowStar(t *testing.T) {
	chk := NewOriginChecker("*", true)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.example")
	if !chk(req) {
		t.Fatal("expected allow")
	}
}

func TestNewOriginChecker_List(t *testing.T) {
	chk := NewOriginChecker("https://ok.example,https://two.example", false)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://ok.example")
	if !chk(req) {
		t.Fatal("expected allow listed")
	}
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Origin", "https://nope.example")
	if chk(req2) {
		t.Fatal("expected deny")
	}
}

func TestNewOriginChecker_StrictEmptyList(t *testing.T) {
	chk := NewOriginChecker("", true)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://any.example")
	if chk(req) {
		t.Fatal("expected deny when strict and Origin set")
	}
	noOrigin := httptest.NewRequest(http.MethodGet, "/", nil)
	if !chk(noOrigin) {
		t.Fatal("expected allow when Origin missing")
	}
}

func TestValidateStreamConn(t *testing.T) {
	if err := validateStreamConn(runtime.StreamConn{UserID: string(make([]byte, 200))}); err == nil {
		t.Fatal("expected too long user_id")
	}
	if err := validateStreamConn(runtime.StreamConn{UserID: "u", AgentID: "a"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateSessionID(t *testing.T) {
	if err := validateSessionID("550e8400-e29b-41d4-a716-446655440000"); err != nil {
		t.Fatal(err)
	}
	if err := validateSessionID("bad\x1fid"); err == nil {
		t.Fatal("expected reject RS")
	}
}

func TestClientVisibleChatError(t *testing.T) {
	if s := clientVisibleChatError(identity.ValidationError{Msg: "x"}); s != "x" {
		t.Fatalf("got %q", s)
	}
	if s := clientVisibleChatError(errors.New("postgres: secret")); s != "Request failed. Please try again later." {
		t.Fatalf("got %q", s)
	}
}

func TestMessageRateLimiter(t *testing.T) {
	l := newMessageRateLimiter(2)
	if !l.allow("1.2.3.4") || !l.allow("1.2.3.4") {
		t.Fatal("expected first two allowed")
	}
	if l.allow("1.2.3.4") {
		t.Fatal("expected third denied")
	}
	if !l.allow("5.6.7.8") {
		t.Fatal("other ip allowed")
	}
}
