package limenstore_test

import (
	"context"
	"testing"
	"time"

	"auth-service/src/modules/auth/limenstore"
)

// TestNewGormAdapter_UnreachableDB asserts that NewGormAdapter fails fast with
// a clear error when the database is unreachable. This tests the ⚠️ edge case:
// "DATABASE_URL set but Postgres unreachable/wrong creds → PingContext fails
// within 5s → newLimen() returns a wrapped error".
//
// Port 1 refuses TCP immediately (well under 5s timeout), triggering a quick,
// clear error from the ping/connection layer.
func TestNewGormAdapter_UnreachableDB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// DSN with port 1 (refuses immediately, not listening).
	// sslmode=disable to avoid TLS negotiation overhead and to test pure connection failure.
	dsn := "postgres://user:pass@127.0.0.1:1/db?sslmode=disable"

	adapter, db, err := limenstore.NewGormAdapter(ctx, dsn)

	// Must return an error; adapter and db must be nil.
	if err == nil {
		t.Error("NewGormAdapter with unreachable DB expected error, got nil")
	}
	if adapter != nil {
		t.Error("NewGormAdapter with unreachable DB expected adapter=nil, got non-nil")
	}
	if db != nil {
		t.Error("NewGormAdapter with unreachable DB expected db=nil, got non-nil")
	}

	// Error should be fast (much less than the 10s timeout we gave the outer context).
	// This is implicit in the test completing quickly; we do not panic is verified
	// by the test runner not crashing.
}

// TestNewGormAdapter_MalformedDSN asserts that an obviously malformed DSN
// fails fast with a clear error (not a panic). This covers the ⚠️ edge case:
// "DATABASE_URL malformed → gorm.Open/postgres.Open error → fail fast".
func TestNewGormAdapter_MalformedDSN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// DSN missing protocol scheme.
	dsn := "not-a-valid-dsn"

	adapter, db, err := limenstore.NewGormAdapter(ctx, dsn)

	if err == nil {
		t.Error("NewGormAdapter with malformed DSN expected error, got nil")
	}
	if adapter != nil {
		t.Error("NewGormAdapter with malformed DSN expected adapter=nil, got non-nil")
	}
	if db != nil {
		t.Error("NewGormAdapter with malformed DSN expected db=nil, got non-nil")
	}
}

// TestNewGormAdapter_ContextCancellation asserts that context cancellation
// is respected during startup (open/ping/migrate). This tests the ⚠️ edge case:
// "context cancellation/deadline → open/ping/migrate all take ctx; use a bounded
// timeout so startup can't hang forever on a stuck DB".
func TestNewGormAdapter_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the open/ping/migrate must respect the cancellation.
	cancel()

	dsn := "postgres://user:pass@127.0.0.1:5432/db?sslmode=disable"

	adapter, db, err := limenstore.NewGormAdapter(ctx, dsn)

	// Must return an error due to context cancellation.
	if err == nil {
		t.Error("NewGormAdapter with cancelled context expected error, got nil")
	}
	if adapter != nil {
		t.Error("NewGormAdapter with cancelled context expected adapter=nil, got non-nil")
	}
	if db != nil {
		t.Error("NewGormAdapter with cancelled context expected db=nil, got non-nil")
	}
}
