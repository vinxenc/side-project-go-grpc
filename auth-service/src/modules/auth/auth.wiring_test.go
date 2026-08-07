package auth_test

import (
	"testing"

	"auth-service/src/modules/auth"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
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

// TestNew_LimenWiringFailure verifies the third failure mode: openDB succeeds
// but the limen wiring phase (newLimen) fails. The shared builder surfaces that
// as an error, which LimenModule.New logs and turns into a route-less module.
//
// It exercises newLimen directly via the NewWithDB seam rather than through
// auth.New, because forcing openDB to succeed while newLimen fails would require
// a live Postgres. limen.New rejects a secret that is not exactly 32 bytes, so a
// short secret drives the wiring failure with no network or Docker.
func TestNew_LimenWiringFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:wiringfail?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	if _, err := auth.NewWithDB(db, []byte("too-short-secret"), "http://localhost:8080"); err == nil {
		t.Fatal("NewWithDB expected an error when limen wiring fails (invalid secret)")
	}
}

// TestModule_Close_ReleasesDBPool verifies that Close releases the connection
// pool newLimen wired, so graceful shutdown does not leak connections. After
// Close, pinging the same underlying pool must fail.
func TestModule_Close_ReleasesDBPool(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:closepool?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	m, err := auth.NewWithDB(db, secret32, "http://localhost:8080")
	if err != nil {
		t.Fatalf("NewWithDB: %v", err)
	}

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	if err := sqlDB.Ping(); err == nil {
		t.Fatal("expected Ping to fail after Close; the pool is still open")
	}
}

// TestModule_Close_NoResources verifies that Close is a no-op for a module that
// owns no resources (a handler-injected module has no database pool).
func TestModule_Close_NoResources(t *testing.T) {
	m := auth.NewWithHandler(nil)
	if err := m.Close(); err != nil {
		t.Fatalf("Close on a resource-less module: %v", err)
	}
}
