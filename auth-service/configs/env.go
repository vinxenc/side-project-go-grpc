// Package configs provides typed, validated configuration for auth-service.
// It reads the .env file (best-effort) and environment variables, validates all
// required fields, and returns an Env value that callers can trust is safe to use.
package configs

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// devSecret is a fixed 32-byte key used only in local development when
// LIMEN_SECRET is unset. It is publicly known (committed in source) and MUST
// NOT be used in any shared or production environment. resolveSecret logs a
// prominent warning whenever this fallback is active.
var devSecret = []byte("0123456789abcdef0123456789abcdef")

// Env holds the validated configuration for auth-service.
type Env struct {
	DatabaseURL string // required; Postgres DSN/URL
	Secret      []byte // exactly 32 bytes (validated)
	BaseURL     string // defaults to http://localhost:8080
}

// Load reads the .env file (best-effort: ignores not-found so containers that
// inject real env vars directly still work), then reads and validates all
// required environment variables and returns a populated Env.
//
// Fail-fast rules:
//   - DATABASE_URL unset/empty → error (no in-memory fallback).
//   - LIMEN_SECRET wrong length → error.
//   - LIMEN_SECRET unset, AUTH_ALLOW_DEV_SECRET != "true" → error (CWE-798 guard).
func Load() (Env, error) {
	// Best-effort .env load; real env vars in containers take precedence.
	if err := godotenv.Load(); err == nil {
		log.Println("INFO: loaded .env file")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Env{}, fmt.Errorf("DATABASE_URL is required but not set")
	}

	secret, err := resolveSecret()
	if err != nil {
		return Env{}, fmt.Errorf("limen secret: %w", err)
	}

	baseURL := os.Getenv("AUTH_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return Env{
		DatabaseURL: databaseURL,
		Secret:      secret,
		BaseURL:     baseURL,
	}, nil
}

// resolveSecret returns a validated 32-byte signing secret. It reads
// LIMEN_SECRET from the environment. If LIMEN_SECRET is set it MUST be exactly
// 32 bytes or an error is returned so the caller can fail fast.
//
// The publicly-known dev fallback is fail-closed: when LIMEN_SECRET is unset
// the function errors out UNLESS AUTH_ALLOW_DEV_SECRET=true is set explicitly
// (local development only). This prevents a real deployment from silently
// booting with a hardcoded, zero-security key (CWE-798).
func resolveSecret() ([]byte, error) {
	raw := os.Getenv("LIMEN_SECRET")
	if raw == "" {
		if os.Getenv("AUTH_ALLOW_DEV_SECRET") == "true" {
			log.Println("WARNING: LIMEN_SECRET is not set. Using the hardcoded dev secret " +
				"(publicly known, zero security) because AUTH_ALLOW_DEV_SECRET=true. " +
				"NEVER set AUTH_ALLOW_DEV_SECRET outside local development.")
			return devSecret, nil
		}
		return nil, fmt.Errorf("LIMEN_SECRET is not set; provide a 32-byte value " +
			"(set AUTH_ALLOW_DEV_SECRET=true to use the insecure dev secret for local development only)")
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("LIMEN_SECRET must be exactly 32 bytes, got %d", len(raw))
	}
	return []byte(raw), nil
}
