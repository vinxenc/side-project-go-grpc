package auth_test

import (
	"testing"

	"auth-service/src/modules/auth"
)

// secret32 is the publicly known 32-byte dev key, valid for constructing the
// module in tests.
var secret32 = []byte("0123456789abcdef0123456789abcdef")

// TestNew_UnreachablePostgres verifies auth.New (→ LimenModule.New) fails fast
// when the database cannot be reached: gorm.Open is lazy, so the connection
// pool opens but the ping fails, exercising the ping-error path. No live
// Postgres is required — the connection to a closed port is refused immediately.
func TestNew_UnreachablePostgres(t *testing.T) {
	_, err := auth.New(auth.Config{
		DatabaseURL: "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1",
		Secret:      secret32,
		BaseURL:     "http://localhost:8080",
	})
	if err == nil {
		t.Fatal("auth.New expected an error for an unreachable Postgres, got nil")
	}
}

// TestNew_MalformedDSN verifies auth.New fails when the DSN cannot be parsed,
// exercising the gorm.Open error path.
func TestNew_MalformedDSN(t *testing.T) {
	_, err := auth.New(auth.Config{
		DatabaseURL: "://not-a-valid-dsn",
		Secret:      secret32,
		BaseURL:     "http://localhost:8080",
	})
	if err == nil {
		t.Fatal("auth.New expected an error for a malformed DSN, got nil")
	}
}
