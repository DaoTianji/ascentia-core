package main

import (
	"log"
	"os"
	"strings"
)

// jwtHS256SecretFromEnv returns the HS256 signing key for WS_AUTH_MODE=jwt.
// Prefer WS_JWT_HS256_SECRET; GVA_JWT_SIGNING_KEY is a deprecated alias for older deployments.
func jwtHS256SecretFromEnv() []byte {
	if s := strings.TrimSpace(os.Getenv("WS_JWT_HS256_SECRET")); s != "" {
		return []byte(s)
	}
	if s := strings.TrimSpace(os.Getenv("GVA_JWT_SIGNING_KEY")); s != "" {
		log.Printf("[config] using deprecated env GVA_JWT_SIGNING_KEY; prefer WS_JWT_HS256_SECRET")
		return []byte(s)
	}
	return nil
}
