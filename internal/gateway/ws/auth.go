package ws

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AuthMode controls WebSocket upgrade authentication.
type AuthMode int

const (
	AuthNone AuthMode = iota
	AuthBearer
	AuthJWT
)

func (m AuthMode) String() string {
	switch m {
	case AuthBearer:
		return "bearer"
	case AuthJWT:
		return "jwt"
	default:
		return "none"
	}
}

// ParseAuthMode maps WS_AUTH_MODE env (none|bearer|jwt).
func ParseAuthMode(s string) (AuthMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "none", "off":
		return AuthNone, nil
	case "bearer":
		return AuthBearer, nil
	case "jwt", "gva_jwt":
		return AuthJWT, nil
	default:
		return AuthNone, fmt.Errorf("ws: unknown WS_AUTH_MODE %q", s)
	}
}

// UpgradeAuthResult is produced after validating the HTTP request before Upgrade.
type UpgradeAuthResult struct {
	JWTUserID string
}

func authBearer(r *http.Request, secret string, allowQuery bool) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return errors.New("ws: WS_BEARER_TOKEN is empty while WS_AUTH_MODE=bearer")
	}
	if got, ok := bearerToken(r); ok {
		if constantTimeEqual(got, secret) {
			return nil
		}
		return errors.New("ws: unauthorized")
	}
	if allowQuery {
		q := strings.TrimSpace(r.URL.Query().Get("bearer_token"))
		if q != "" && constantTimeEqual(q, secret) {
			return nil
		}
	}
	return errors.New("ws: unauthorized")
}

// jwtClaimsMinimal is a minimal HS256 payload: numeric user id + standard time claims.
// Compatible with common admin templates that embed a uint `ID` at the top level; replace or extend if your IdP differs.
type jwtClaimsMinimal struct {
	ID uint `json:"ID"`
	jwt.RegisteredClaims
}

func authJWTRequest(r *http.Request, signingKey []byte, allowQuery bool) (UpgradeAuthResult, error) {
	var out UpgradeAuthResult
	if len(signingKey) == 0 {
		return out, errors.New("ws: WS_JWT_HS256_SECRET is empty while WS_AUTH_MODE=jwt")
	}
	if raw, ok := bearerToken(r); ok {
		return parseJWTMinimal(raw, signingKey)
	}
	if allowQuery {
		q := strings.TrimSpace(r.URL.Query().Get("access_token"))
		if q != "" {
			return parseJWTMinimal(q, signingKey)
		}
	}
	return out, errors.New("ws: missing bearer token")
}

func parseJWTMinimal(raw string, signingKey []byte) (UpgradeAuthResult, error) {
	var out UpgradeAuthResult
	tok, err := jwt.ParseWithClaims(raw, &jwtClaimsMinimal{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return signingKey, nil
	})
	if err != nil {
		return out, fmt.Errorf("ws: invalid token")
	}
	claims, ok := tok.Claims.(*jwtClaimsMinimal)
	if !ok || !tok.Valid {
		return out, fmt.Errorf("ws: invalid token")
	}
	if claims.ID == 0 {
		return out, fmt.Errorf("ws: token missing user id")
	}
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return out, fmt.Errorf("ws: token expired")
	}
	out.JWTUserID = strconv.FormatUint(uint64(claims.ID), 10)
	return out, nil
}

func bearerToken(r *http.Request) (string, bool) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if h == "" {
		return "", false
	}
	const p = "Bearer "
	if !strings.HasPrefix(h, p) {
		return "", false
	}
	t := strings.TrimSpace(strings.TrimPrefix(h, p))
	if t == "" {
		return "", false
	}
	return t, true
}

func constantTimeEqual(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
