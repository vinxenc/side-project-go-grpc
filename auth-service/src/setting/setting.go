// Package setting provides typed, validated configuration for auth-service.
// It reads the .env file (best-effort) and environment variables, validates all
// required fields, and returns a Setting value that callers can trust is safe
// to use.
package setting

import (
	"fmt"
	"log"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// devSecret is a fixed 32-byte key used only in local development when
// LIMEN_SECRET is unset. It is publicly known (committed in source) and MUST
// NOT be used in any shared or production environment. resolveSecret logs a
// prominent warning whenever this fallback is active.
var devSecret = []byte("0123456789abcdef0123456789abcdef")

// Setting holds the validated configuration for auth-service.
type Setting struct {
	DatabaseURL string // required; Postgres DSN/URL
	Secret      []byte // exactly 32 bytes (validated)
	BaseURL     string // defaults to http://localhost:8080
	Addr        string // TCP listen address for the HTTP server; defaults to :8080
}

// rawSetting mirrors the raw environment variables as parsed by caarlos0/env.
// Parsing enforces presence and type (DATABASE_URL must be set and non-empty;
// ALLOW_DEV_SECRET must be a valid bool). Load performs the remaining
// domain validation — chiefly the 32-byte secret rule — before producing a
// Setting.
type rawSetting struct {
	DatabaseURL    string `env:"DATABASE_URL,notEmpty"`
	Secret         string `env:"LIMEN_SECRET"`
	AllowDevSecret bool   `env:"ALLOW_DEV_SECRET"`
	BaseURL        string `env:"BASE_URL" envDefault:"http://localhost:8080"`
	Addr           string `env:"ADDR" envDefault:":8080"`
}

// Load reads the .env file (best-effort: ignores not-found so containers that
// inject real env vars directly still work), parses and validates the
// environment via caarlos0/env, and returns a populated Setting.
//
// Fail-fast rules:
//   - DATABASE_URL unset/empty → error (no in-memory fallback; enforced by the
//     notEmpty tag).
//   - LIMEN_SECRET set but not exactly 32 bytes → error.
//   - LIMEN_SECRET unset and ALLOW_DEV_SECRET != true → error (CWE-798 guard).
func Load() (Setting, error) {
	// Best-effort .env load; real env vars in containers take precedence.
	if err := godotenv.Load(); err == nil {
		log.Println("INFO: loaded .env file")
	}

	var raw rawSetting
	if err := env.Parse(&raw); err != nil {
		return Setting{}, fmt.Errorf("parse environment: %w", err)
	}

	secret, err := resolveSecret(raw.Secret, raw.AllowDevSecret)
	if err != nil {
		return Setting{}, fmt.Errorf("limen secret: %w", err)
	}

	return Setting{
		DatabaseURL: raw.DatabaseURL,
		Secret:      secret,
		BaseURL:     raw.BaseURL,
		Addr:        raw.Addr,
	}, nil
}

// resolveSecret returns a validated 32-byte signing secret from the raw
// LIMEN_SECRET value. If raw is set it MUST be exactly 32 bytes or an error is
// returned so the caller can fail fast.
//
// The publicly-known dev fallback is fail-closed: when raw is empty the function
// errors out UNLESS allowDev is true (ALLOW_DEV_SECRET=true, local
// development only). This prevents a real deployment from silently booting with
// a hardcoded, zero-security key (CWE-798).
func resolveSecret(raw string, allowDev bool) ([]byte, error) {
	if raw == "" {
		if allowDev {
			log.Println("WARNING: LIMEN_SECRET is not set. Using the hardcoded dev secret " +
				"(publicly known, zero security) because ALLOW_DEV_SECRET=true. " +
				"NEVER set ALLOW_DEV_SECRET outside local development.")
			return devSecret, nil
		}
		return nil, fmt.Errorf("LIMEN_SECRET is not set; provide a 32-byte value " +
			"(set ALLOW_DEV_SECRET=true to use the insecure dev secret for local development only)")
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("LIMEN_SECRET must be exactly 32 bytes, got %d", len(raw))
	}
	return []byte(raw), nil
}
