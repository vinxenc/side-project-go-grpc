package limenstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"auth-service/src/modules/auth/limenstore"
	"github.com/thecodearcher/limen"
)

func TestMemoryAdapter_Create(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		wantErr bool
	}{
		{
			name: "create row with explicit id",
			data: map[string]any{
				"id":    42,
				"email": "test@example.com",
			},
			wantErr: false,
		},
		{
			name: "create row without id (auto-assigned)",
			data: map[string]any{
				"email": "test@example.com",
			},
			wantErr: false,
		},
		{
			name:    "create empty row",
			data:    map[string]any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := limenstore.NewMemoryAdapter()
			ctx := context.Background()

			err := adapter.Create(ctx, "users", tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryAdapter_Create_AutoIncrement(t *testing.T) {
	adapter := limenstore.NewMemoryAdapter()
	ctx := context.Background()

	// Create first row without id
	if err := adapter.Create(ctx, "users", map[string]any{
		"email": "user1@example.com",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create second row without id
	if err := adapter.Create(ctx, "users", map[string]any{
		"email": "user2@example.com",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Find both rows and verify IDs were auto-assigned differently
	row1, err := adapter.FindOne(ctx, "users", []limen.Where{
		{Column: "email", Operator: limen.OpEq, Value: "user1@example.com"},
	}, nil)
	if err != nil {
		t.Fatalf("FindOne failed: %v", err)
	}

	row2, err := adapter.FindOne(ctx, "users", []limen.Where{
		{Column: "email", Operator: limen.OpEq, Value: "user2@example.com"},
	}, nil)
	if err != nil {
		t.Fatalf("FindOne failed: %v", err)
	}

	id1 := row1["id"]
	id2 := row2["id"]
	if id1 == id2 {
		t.Errorf("auto-generated IDs are the same: %v", id1)
	}
}

func TestMemoryAdapter_FindOne(t *testing.T) {
	adapter := limenstore.NewMemoryAdapter()
	ctx := context.Background()

	if err := adapter.Create(ctx, "users", map[string]any{
		"email": "test@example.com",
		"name":  "Test User",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	tests := []struct {
		name       string
		conditions []limen.Where
		wantErr    bool
		errIs      error
	}{
		{
			name: "find by exact email",
			conditions: []limen.Where{
				{Column: "email", Operator: limen.OpEq, Value: "test@example.com"},
			},
			wantErr: false,
		},
		{
			name:       "find non-existent record",
			conditions: []limen.Where{{Column: "email", Operator: limen.OpEq, Value: "notfound@example.com"}},
			wantErr:    true,
			errIs:      limen.ErrRecordNotFound,
		},
		{
			name:       "find with no conditions",
			conditions: []limen.Where{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, err := adapter.FindOne(ctx, "users", tt.conditions, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindOne() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errIs != nil && !errors.Is(err, tt.errIs) {
				t.Errorf("FindOne() error = %v, want %v", err, tt.errIs)
			}
			if !tt.wantErr && row == nil {
				t.Error("FindOne() returned nil row")
			}
		})
	}
}

func TestMemoryAdapter_FindMany(t *testing.T) {
	adapter := limenstore.NewMemoryAdapter()
	ctx := context.Background()

	// Insert 5 test rows
	for i := 1; i <= 5; i++ {
		email := "user" + string(rune('0'+i)) + "@example.com"
		if err := adapter.Create(ctx, "users", map[string]any{
			"email": email,
			"age":   int64(20 + i),
		}); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	tests := []struct {
		name       string
		conditions []limen.Where
		options    *limen.QueryOptions
		wantCount  int
	}{
		{
			name:       "find all with no conditions",
			conditions: []limen.Where{},
			wantCount:  5,
		},
		{
			name: "find with condition",
			conditions: []limen.Where{
				{Column: "age", Operator: limen.OpGt, Value: int64(22)},
			},
			wantCount: 3,
		},
		{
			name:       "find with limit",
			conditions: []limen.Where{},
			options:    &limen.QueryOptions{Limit: 2},
			wantCount:  2,
		},
		{
			name:       "find with offset",
			conditions: []limen.Where{},
			options:    &limen.QueryOptions{Offset: 3},
			wantCount:  2,
		},
		{
			name:       "find with limit and offset",
			conditions: []limen.Where{},
			options:    &limen.QueryOptions{Limit: 2, Offset: 2},
			wantCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := adapter.FindMany(ctx, "users", tt.conditions, tt.options)
			if err != nil {
				t.Errorf("FindMany() error = %v", err)
			}
			if len(rows) != tt.wantCount {
				t.Errorf("FindMany() got %d rows, want %d", len(rows), tt.wantCount)
			}
		})
	}
}

func TestMemoryAdapter_Update(t *testing.T) {
	adapter := limenstore.NewMemoryAdapter()
	ctx := context.Background()

	if err := adapter.Create(ctx, "users", map[string]any{
		"email": "test@example.com",
		"name":  "Original",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update the row
	if err := adapter.Update(ctx, "users", []limen.Where{
		{Column: "email", Operator: limen.OpEq, Value: "test@example.com"},
	}, map[string]any{
		"name": "Updated",
	}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	row, err := adapter.FindOne(ctx, "users", []limen.Where{
		{Column: "email", Operator: limen.OpEq, Value: "test@example.com"},
	}, nil)
	if err != nil {
		t.Fatalf("FindOne failed: %v", err)
	}

	if row["name"] != "Updated" {
		t.Errorf("Update() name = %v, want Updated", row["name"])
	}
}

func TestMemoryAdapter_Delete(t *testing.T) {
	adapter := limenstore.NewMemoryAdapter()
	ctx := context.Background()

	// Create two rows
	adapter.Create(ctx, "users", map[string]any{
		"email": "user1@example.com",
	})
	adapter.Create(ctx, "users", map[string]any{
		"email": "user2@example.com",
	})

	// Delete one row
	if err := adapter.Delete(ctx, "users", []limen.Where{
		{Column: "email", Operator: limen.OpEq, Value: "user1@example.com"},
	}); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deletion
	_, err := adapter.FindOne(ctx, "users", []limen.Where{
		{Column: "email", Operator: limen.OpEq, Value: "user1@example.com"},
	}, nil)
	if !errors.Is(err, limen.ErrRecordNotFound) {
		t.Errorf("Delete() failed: user1 still exists")
	}

	// Verify other row still exists
	row, err := adapter.FindOne(ctx, "users", []limen.Where{
		{Column: "email", Operator: limen.OpEq, Value: "user2@example.com"},
	}, nil)
	if err != nil || row == nil {
		t.Errorf("Delete() deleted wrong row: %v", err)
	}
}

func TestMemoryAdapter_Exists(t *testing.T) {
	adapter := limenstore.NewMemoryAdapter()
	ctx := context.Background()

	adapter.Create(ctx, "users", map[string]any{
		"email": "test@example.com",
	})

	tests := []struct {
		name       string
		conditions []limen.Where
		want       bool
	}{
		{
			name: "record exists",
			conditions: []limen.Where{
				{Column: "email", Operator: limen.OpEq, Value: "test@example.com"},
			},
			want: true,
		},
		{
			name: "record does not exist",
			conditions: []limen.Where{
				{Column: "email", Operator: limen.OpEq, Value: "notfound@example.com"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := adapter.Exists(ctx, "users", tt.conditions)
			if err != nil {
				t.Errorf("Exists() error = %v", err)
			}
			if exists != tt.want {
				t.Errorf("Exists() = %v, want %v", exists, tt.want)
			}
		})
	}
}

func TestMemoryAdapter_Count(t *testing.T) {
	adapter := limenstore.NewMemoryAdapter()
	ctx := context.Background()

	// Create 3 rows
	for i := 1; i <= 3; i++ {
		adapter.Create(ctx, "users", map[string]any{
			"status": "active",
		})
	}

	adapter.Create(ctx, "users", map[string]any{
		"status": "inactive",
	})

	tests := []struct {
		name       string
		conditions []limen.Where
		want       int64
	}{
		{
			name:       "count all",
			conditions: []limen.Where{},
			want:       4,
		},
		{
			name: "count active users",
			conditions: []limen.Where{
				{Column: "status", Operator: limen.OpEq, Value: "active"},
			},
			want: 3,
		},
		{
			name: "count inactive users",
			conditions: []limen.Where{
				{Column: "status", Operator: limen.OpEq, Value: "inactive"},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := adapter.Count(ctx, "users", tt.conditions)
			if err != nil {
				t.Errorf("Count() error = %v", err)
			}
			if count != tt.want {
				t.Errorf("Count() = %d, want %d", count, tt.want)
			}
		})
	}
}

func TestMemoryAdapter_Operators(t *testing.T) {
	adapter := limenstore.NewMemoryAdapter()
	ctx := context.Background()

	// Create test data
	now := time.Now()
	adapter.Create(ctx, "users", map[string]any{
		"id":         1,
		"name":       "alice",
		"email":      "alice@example.com",
		"age":        int64(25),
		"tags":       "admin,user",
		"created_at": now,
	})
	adapter.Create(ctx, "users", map[string]any{
		"id":         2,
		"name":       "bob",
		"email":      "bob@example.com",
		"age":        int64(30),
		"tags":       "user",
		"created_at": now.Add(1 * time.Hour),
	})

	tests := []struct {
		name        string
		condition   limen.Where
		wantMatches int
	}{
		{
			name:        "operator Eq",
			condition:   limen.Where{Column: "name", Operator: limen.OpEq, Value: "alice"},
			wantMatches: 1,
		},
		{
			name:        "operator Ne",
			condition:   limen.Where{Column: "name", Operator: limen.OpNe, Value: "alice"},
			wantMatches: 1,
		},
		{
			name:        "operator Gt",
			condition:   limen.Where{Column: "age", Operator: limen.OpGt, Value: int64(25)},
			wantMatches: 1,
		},
		{
			name:        "operator Gte",
			condition:   limen.Where{Column: "age", Operator: limen.OpGte, Value: int64(25)},
			wantMatches: 2,
		},
		{
			name:        "operator Lt",
			condition:   limen.Where{Column: "age", Operator: limen.OpLt, Value: int64(30)},
			wantMatches: 1,
		},
		{
			name:        "operator Contains",
			condition:   limen.Where{Column: "tags", Operator: limen.OpContains, Value: "admin"},
			wantMatches: 1,
		},
		{
			name:        "operator StartsWith",
			condition:   limen.Where{Column: "email", Operator: limen.OpStartsWith, Value: "alice"},
			wantMatches: 1,
		},
		{
			name:        "operator EndsWith",
			condition:   limen.Where{Column: "email", Operator: limen.OpEndsWith, Value: ".com"},
			wantMatches: 2,
		},
		{
			name:        "operator In",
			condition:   limen.Where{Column: "name", Operator: limen.OpIn, Value: []any{"alice", "charlie"}},
			wantMatches: 1,
		},
		{
			name:        "operator NotIn",
			condition:   limen.Where{Column: "name", Operator: limen.OpNotIn, Value: []any{"charlie", "dave"}},
			wantMatches: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := adapter.FindMany(ctx, "users", []limen.Where{tt.condition}, nil)
			if err != nil {
				t.Errorf("FindMany() error = %v", err)
			}
			if len(rows) != tt.wantMatches {
				t.Errorf("FindMany() got %d matches, want %d", len(rows), tt.wantMatches)
			}
		})
	}
}

func TestMemoryAdapter_Concurrent(t *testing.T) {
	adapter := limenstore.NewMemoryAdapter()
	ctx := context.Background()

	// Test concurrent Creates
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			err := adapter.Create(ctx, "users", map[string]any{
				"email": "user" + string(rune('0'+idx)) + "@example.com",
			})
			done <- err
		}(i)
	}

	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent Create() error = %v", err)
		}
	}

	// Verify all were created
	count, err := adapter.Count(ctx, "users", []limen.Where{})
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 10 {
		t.Errorf("Count() = %d, want 10", count)
	}
}

func TestMemoryAdapter_IsNullAndIsNotNull(t *testing.T) {
	adapter := limenstore.NewMemoryAdapter()
	ctx := context.Background()

	adapter.Create(ctx, "users", map[string]any{
		"name":        "Alice",
		"middle_name": "Grace",
	})
	adapter.Create(ctx, "users", map[string]any{
		"name":        "Bob",
		"middle_name": nil,
	})

	tests := []struct {
		name        string
		condition   limen.Where
		wantMatches int
	}{
		{
			name:        "operator IsNotNull",
			condition:   limen.Where{Column: "middle_name", Operator: limen.OpIsNotNull},
			wantMatches: 1,
		},
		{
			name:        "operator IsNull",
			condition:   limen.Where{Column: "middle_name", Operator: limen.OpIsNull},
			wantMatches: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := adapter.FindMany(ctx, "users", []limen.Where{tt.condition}, nil)
			if err != nil {
				t.Errorf("FindMany() error = %v", err)
			}
			if len(rows) != tt.wantMatches {
				t.Errorf("FindMany() got %d matches, want %d", len(rows), tt.wantMatches)
			}
		})
	}
}

func TestMemoryAdapter_OrderBy(t *testing.T) {
	adapter := limenstore.NewMemoryAdapter()
	ctx := context.Background()

	for _, name := range []string{"charlie", "alice", "bob"} {
		adapter.Create(ctx, "users", map[string]any{
			"name": name,
		})
	}

	// Find all and sort ascending
	rows, err := adapter.FindMany(ctx, "users", []limen.Where{}, &limen.QueryOptions{
		OrderBy: []limen.OrderBy{
			{Column: "name", Direction: limen.OrderByAsc},
		},
	})
	if err != nil {
		t.Fatalf("FindMany() error = %v", err)
	}

	expectedOrder := []string{"alice", "bob", "charlie"}
	if len(rows) != len(expectedOrder) {
		t.Fatalf("OrderBy() asc: got %d rows, want %d", len(rows), len(expectedOrder))
	}
	for i, name := range expectedOrder {
		if rows[i]["name"] != name {
			t.Errorf("OrderBy() row %d: got %v, want %v", i, rows[i]["name"], name)
		}
	}

	// Find all and sort descending
	rows, err = adapter.FindMany(ctx, "users", []limen.Where{}, &limen.QueryOptions{
		OrderBy: []limen.OrderBy{
			{Column: "name", Direction: limen.OrderByDesc},
		},
	})
	if err != nil {
		t.Fatalf("FindMany() error = %v", err)
	}

	expectedOrder = []string{"charlie", "bob", "alice"}
	if len(rows) != len(expectedOrder) {
		t.Fatalf("OrderBy() desc: got %d rows, want %d", len(rows), len(expectedOrder))
	}
	for i, name := range expectedOrder {
		if rows[i]["name"] != name {
			t.Errorf("OrderBy() desc row %d: got %v, want %v", i, rows[i]["name"], name)
		}
	}
}
