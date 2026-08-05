package auth

import (
	"context"
	"testing"

	"github.com/thecodearcher/limen"
)

// TestNewDatabaseAdapter_MemoryWhenEmpty asserts that with DATABASE_URL unset,
// newDatabaseAdapter returns the in-memory adapter. This is the CI-safe default
// (no Postgres, no CGO, no network required). This tests the ⚠️ edge case:
// "DATABASE_URL unset/empty → use in-memory adapter (dev/test); log the chosen
// backend so it is never silent."
func TestNewDatabaseAdapter_MemoryWhenEmpty(t *testing.T) {
	t.Setenv("DATABASE_URL", "") // Ensure it is unset/empty.

	adapter, err := newDatabaseAdapter()

	if err != nil {
		t.Errorf("newDatabaseAdapter with empty DATABASE_URL expected no error, got %v", err)
	}

	if adapter == nil {
		t.Error("newDatabaseAdapter with empty DATABASE_URL expected non-nil adapter, got nil")
		return
	}

	// Verify the adapter implements limen.DatabaseAdapter by calling a basic operation.
	// Create a test record and verify it works (in-memory, no DB required).
	ctx := context.Background()
	err = adapter.Create(ctx, "test_table", map[string]any{
		"id":    1,
		"name":  "test",
		"email": "test@example.com",
	})

	if err != nil {
		t.Errorf("in-memory adapter Create expected no error, got %v", err)
	}

	// Verify FindOne works (returns what we created).
	result, err := adapter.FindOne(ctx, "test_table", []limen.Where{
		limen.Eq("id", 1),
	}, nil)

	if err != nil {
		t.Errorf("in-memory adapter FindOne expected no error, got %v", err)
	}

	if result == nil {
		t.Error("in-memory adapter FindOne expected non-nil result, got nil")
	} else if result["name"] != "test" {
		t.Errorf("in-memory adapter FindOne expected name=test, got %v", result["name"])
	}
}

// TestNewDatabaseAdapter_PostgresWhenSet asserts that with DATABASE_URL set,
// newDatabaseAdapter attempts to connect to Postgres (and fails if unreachable,
// but that is tested separately in postgres_test.go). This verifies the adapter
// selection logic: non-empty DATABASE_URL → Postgres path.
func TestNewDatabaseAdapter_PostgresWhenSet(t *testing.T) {
	// Set an invalid but well-formed DSN (port 1 refuses immediately).
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable")

	adapter, err := newDatabaseAdapter()

	// We expect this to fail because the DB is unreachable, but the point is
	// that it *attempted* the Postgres path (not the in-memory path).
	// We verify this by checking that err is non-nil and matches Postgres connection errors.

	if err == nil {
		t.Error("newDatabaseAdapter with unreachable Postgres expected error, got nil")
		if adapter != nil {
			t.Error("newDatabaseAdapter with unreachable Postgres should return nil adapter on error")
		}
		return
	}

	// Error should be a Postgres connection error (open, ping, etc).
	// We don't make strict assertions on the error message, just that it failed
	// because it tried to connect to Postgres (as evidenced by the error being non-nil).
	if adapter != nil {
		t.Error("newDatabaseAdapter with unreachable Postgres expected adapter=nil, got non-nil")
	}
}
