package auth_test

import (
	"testing"

	"auth-service/src/modules/auth"
)

// secret32 is the publicly known 32-byte dev key, valid for constructing the
// module in tests.
var secret32 = []byte("0123456789abcdef0123456789abcdef")

// TestNew_UnreachablePostgres verifies auth.New (→ LimenModule.New) degrades to
// a route-less module when the database cannot be reached: gorm.Open is lazy, so
// the connection pool opens but the ping fails, exercising the ping-error path.
// No live Postgres is required — the connection to a closed port is refused
// immediately. New logs the cause and returns a Module with a nil controller.
func TestNew_UnreachablePostgres(t *testing.T) {
	m := auth.New(auth.Config{
		DatabaseURL: "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1",
		Secret:      secret32,
		BaseURL:     "http://localhost:8080",
	})
	if m.Controller() != nil {
		t.Fatal("auth.New expected a nil controller for an unreachable Postgres")
	}
}

// TestNew_MalformedDSN verifies auth.New degrades to a route-less module when
// the DSN cannot be parsed, exercising the gorm.Open error path.
func TestNew_MalformedDSN(t *testing.T) {
	m := auth.New(auth.Config{
		DatabaseURL: "://not-a-valid-dsn",
		Secret:      secret32,
		BaseURL:     "http://localhost:8080",
	})
	if m.Controller() != nil {
		t.Fatal("auth.New expected a nil controller for a malformed DSN")
	}
}
