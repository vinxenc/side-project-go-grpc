// Package limenstore provides a non-persistent in-memory DatabaseAdapter for
// use with the limen authentication library. It is ported (with renaming) from
// limen's MIT-licensed testing_support.go (github.com/thecodearcher/limen@v0.1.4).
//
// All data is stored in a process-lifetime map and is lost on restart.
// This is intentional for the current iteration; a real database adapter
// (gorm/sql) should be wired in for production persistence.
package limenstore

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/thecodearcher/limen"
)

// memTable holds the rows for a single logical table.
type memTable struct {
	rows   []map[string]any
	nextID int64
}

// MemoryAdapter is a fully in-memory implementation of limen.DatabaseAdapter.
// It is safe for concurrent use; all operations hold the mutex for their
// duration. It does NOT implement TransactionalAdapter, so limen's
// context_transaction.go falls back to invoking fn(ctx) directly.
type MemoryAdapter struct {
	mu     sync.Mutex
	tables map[limen.SchemaTableName]*memTable
}

// NewMemoryAdapter returns an empty MemoryAdapter ready for use.
func NewMemoryAdapter() *MemoryAdapter {
	return &MemoryAdapter{
		tables: make(map[limen.SchemaTableName]*memTable),
	}
}

// table returns the memTable for the given name, creating it on first access.
// Must be called with mu held.
func (a *MemoryAdapter) table(name limen.SchemaTableName) *memTable {
	t, ok := a.tables[name]
	if !ok {
		t = &memTable{nextID: 1}
		a.tables[name] = t
	}
	return t
}

// Create inserts a new row into the named table. If the row has no "id" key,
// an auto-incrementing integer id is assigned.
func (a *MemoryAdapter) Create(_ context.Context, tableName limen.SchemaTableName, data map[string]any) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(data) == 0 {
		return fmt.Errorf("no data to insert")
	}

	tbl := a.table(tableName)

	row := make(map[string]any, len(data))
	for k, v := range data {
		row[k] = derefPointer(v)
	}

	if _, hasID := row["id"]; !hasID {
		row["id"] = tbl.nextID
		tbl.nextID++
	}

	tbl.rows = append(tbl.rows, row)
	return nil
}

// FindOne returns the first row matching conditions after applying optional
// ordering. Returns limen.ErrRecordNotFound when no row matches.
func (a *MemoryAdapter) FindOne(_ context.Context, tableName limen.SchemaTableName, conditions []limen.Where, orderBy []limen.OrderBy) (map[string]any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	tbl := a.table(tableName)
	matched := filterRows(tbl.rows, conditions)
	sortRows(matched, orderBy)

	if len(matched) == 0 {
		return nil, limen.ErrRecordNotFound
	}
	return maps.Clone(matched[0]), nil
}

// FindMany returns all rows matching conditions, with optional limit/offset/ordering.
func (a *MemoryAdapter) FindMany(_ context.Context, tableName limen.SchemaTableName, conditions []limen.Where, options *limen.QueryOptions) ([]map[string]any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	tbl := a.table(tableName)
	matched := filterRows(tbl.rows, conditions)

	if options != nil {
		sortRows(matched, options.OrderBy)
		if options.Offset > 0 {
			if options.Offset < len(matched) {
				matched = matched[options.Offset:]
			} else {
				matched = nil
			}
		}
		if options.Limit > 0 && options.Limit < len(matched) {
			matched = matched[:options.Limit]
		}
	}

	results := make([]map[string]any, len(matched))
	for i, r := range matched {
		results[i] = maps.Clone(r)
	}
	return results, nil
}

// Update mutates all rows matching conditions in place, applying the given
// field updates.
func (a *MemoryAdapter) Update(_ context.Context, tableName limen.SchemaTableName, conditions []limen.Where, updates map[string]any) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(updates) == 0 {
		return nil
	}

	tbl := a.table(tableName)
	for _, row := range tbl.rows {
		if matchesConditions(row, conditions) {
			for k, v := range updates {
				row[k] = derefPointer(v)
			}
		}
	}
	return nil
}

// Delete removes all rows matching conditions.
func (a *MemoryAdapter) Delete(_ context.Context, tableName limen.SchemaTableName, conditions []limen.Where) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	tbl := a.table(tableName)
	tbl.rows = slices.DeleteFunc(tbl.rows, func(row map[string]any) bool {
		return matchesConditions(row, conditions)
	})
	return nil
}

// Exists reports whether at least one row matches conditions.
func (a *MemoryAdapter) Exists(_ context.Context, tableName limen.SchemaTableName, conditions []limen.Where) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	tbl := a.table(tableName)
	for _, row := range tbl.rows {
		if matchesConditions(row, conditions) {
			return true, nil
		}
	}
	return false, nil
}

// Count returns the number of rows that match conditions.
func (a *MemoryAdapter) Count(_ context.Context, tableName limen.SchemaTableName, conditions []limen.Where) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	tbl := a.table(tableName)
	var count int64
	for _, row := range tbl.rows {
		if matchesConditions(row, conditions) {
			count++
		}
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Pointer dereferencing (mirrors limen's testDerefPointer)
// ---------------------------------------------------------------------------

// derefPointer unwraps *string and *time.Time values that limen's ToStorage
// helper produces, so FromStorage type assertions match real driver behaviour.
func derefPointer(v any) any {
	switch p := v.(type) {
	case *string:
		if p == nil {
			return nil
		}
		return *p
	case *time.Time:
		if p == nil {
			return nil
		}
		return *p
	default:
		return v
	}
}

// ---------------------------------------------------------------------------
// Condition matching (mirrors limen's test* helpers)
// ---------------------------------------------------------------------------

func filterRows(rows []map[string]any, conditions []limen.Where) []map[string]any {
	if len(conditions) == 0 {
		return slices.Clone(rows)
	}
	var out []map[string]any
	for _, row := range rows {
		if matchesConditions(row, conditions) {
			out = append(out, row)
		}
	}
	return out
}

func matchesConditions(row map[string]any, conditions []limen.Where) bool {
	if len(conditions) == 0 {
		return true
	}
	groups := limen.GroupConditionsByConnector(conditions)
	for _, group := range groups {
		if !matchesGroup(row, group) {
			return false
		}
	}
	return true
}

func matchesGroup(row map[string]any, group []limen.Where) bool {
	for _, c := range group {
		if matchesSingle(row, c) {
			return true
		}
	}
	return false
}

func matchesSingle(row map[string]any, c limen.Where) bool {
	val := row[c.Column]

	switch c.Operator {
	case limen.OpIsNull:
		return val == nil
	case limen.OpIsNotNull:
		return val != nil
	case limen.OpEq, "":
		return valuesEqual(val, c.Value)
	case limen.OpNe:
		return !valuesEqual(val, c.Value)
	case limen.OpLt:
		cmp, ok := compareValues(val, c.Value)
		return ok && cmp < 0
	case limen.OpLte:
		cmp, ok := compareValues(val, c.Value)
		return ok && cmp <= 0
	case limen.OpGt:
		cmp, ok := compareValues(val, c.Value)
		return ok && cmp > 0
	case limen.OpGte:
		cmp, ok := compareValues(val, c.Value)
		return ok && cmp >= 0
	case limen.OpContains:
		return strings.Contains(fmt.Sprint(val), fmt.Sprint(c.Value))
	case limen.OpStartsWith:
		return strings.HasPrefix(fmt.Sprint(val), fmt.Sprint(c.Value))
	case limen.OpEndsWith:
		return strings.HasSuffix(fmt.Sprint(val), fmt.Sprint(c.Value))
	case limen.OpIn:
		vals, ok := c.Value.([]any)
		if !ok {
			return false
		}
		for _, v := range vals {
			if valuesEqual(val, v) {
				return true
			}
		}
		return false
	case limen.OpNotIn:
		vals, ok := c.Value.([]any)
		if !ok {
			return true
		}
		for _, v := range vals {
			if valuesEqual(val, v) {
				return false
			}
		}
		return true
	default:
		return valuesEqual(val, c.Value)
	}
}

func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a == b
}

func compareValues(a, b any) (int, bool) {
	switch av := a.(type) {
	case time.Time:
		bv, ok := b.(time.Time)
		if !ok {
			return 0, false
		}
		return av.Compare(bv), true
	case int64:
		bv, ok := b.(int64)
		if !ok {
			return 0, false
		}
		if av < bv {
			return -1, true
		}
		if av > bv {
			return 1, true
		}
		return 0, true
	case string:
		bv, ok := b.(string)
		if !ok {
			return 0, false
		}
		if av < bv {
			return -1, true
		}
		if av > bv {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

// ---------------------------------------------------------------------------
// Row sorting (mirrors limen's testSortRows)
// ---------------------------------------------------------------------------

func sortRows(rows []map[string]any, orderBy []limen.OrderBy) {
	if len(orderBy) == 0 {
		return
	}
	slices.SortStableFunc(rows, func(a, b map[string]any) int {
		for _, ob := range orderBy {
			if ob.Column == "" {
				continue
			}
			cmp, ok := compareValues(a[ob.Column], b[ob.Column])
			if !ok || cmp == 0 {
				continue
			}
			if ob.Direction == limen.OrderByDesc {
				return -cmp
			}
			return cmp
		}
		return 0
	})
}
