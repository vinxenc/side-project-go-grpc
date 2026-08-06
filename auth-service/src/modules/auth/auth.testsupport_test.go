package auth_test

import (
	_ "embed"
	"fmt"
	"strings"
	"testing"
	"time"

	"auth-service/src/modules/auth"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

//go:embed testdata/limen_schema_sqlite.sql
var limenSchemaSQL string

// testSecret is the fixed 32-byte dev key reused from .example.env. It is
// publicly known and committed in source — acceptable for test-only use.
var testSecret = []byte("0123456789abcdef0123456789abcdef")

// limenTimestampCols is the set of column names that limen's FromStorage
// functions expect to contain time.Time values. SQLite stores them as TEXT;
// the sqliteTimestampCallback parses them back to time.Time so that limen's
// hard type assertions (data["expires_at"].(time.Time)) don't panic.
var limenTimestampCols = map[string]bool{
	"created_at":              true,
	"updated_at":              true,
	"expires_at":              true,
	"last_access":             true,
	"email_verified_at":       true,
	"access_token_expires_at": true,
}

// sqliteTimestampFormats lists the time layouts that modernc.org/sqlite may
// use when serialising time.Time bind values as TEXT column data.
var sqliteTimestampFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999-07:00",
	"2006-01-02 15:04:05-07:00",
	"2006-01-02 15:04:05",
}

func parseTimestamp(s string) (time.Time, bool) {
	for _, f := range sqliteTimestampFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func fixTimestampsInMap(m map[string]any) {
	for k, v := range m {
		if !limenTimestampCols[k] {
			continue
		}
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		if t, ok := parseTimestamp(s); ok {
			m[k] = t
		}
	}
}

// sqliteTimestampCallback is a GORM query callback that converts string
// timestamp fields in map[string]any query results to time.Time. This is
// necessary because the glebarez/sqlite (modernc) driver returns TEXT columns
// as strings, but limen's FromStorage functions perform hard time.Time
// assertions. The callback is registered After("gorm:query") so results are
// already populated when it runs.
func sqliteTimestampCallback(tx *gorm.DB) {
	if tx.Error != nil || tx.Statement == nil || tx.Statement.Dest == nil {
		return
	}
	switch v := tx.Statement.Dest.(type) {
	case *map[string]any:
		fixTimestampsInMap(*v)
	case *[]map[string]any:
		for i := range *v {
			fixTimestampsInMap((*v)[i])
		}
	}
}

// newTestModule opens a unique per-test in-memory SQLite database, applies the
// limen schema, registers a timestamp-conversion GORM callback (needed because
// modernc.org/sqlite returns TEXT columns as strings), and returns an
// auth.Module backed by the SQLite GORM adapter.
// No network connection or Docker is required.
//
// Isolation: each test gets its own in-memory DB identified by the *testing.T
// pointer address. SetMaxOpenConns(1) ensures all pool connections target the
// same in-memory database. t.Cleanup closes the underlying connection.
func newTestModule(t *testing.T) *auth.Module {
	t.Helper()

	// Unique per-test DSN; mode=memory + cache=shared keeps the DB alive as
	// long as at least one connection is open (until t.Cleanup closes it).
	dsn := fmt.Sprintf("file:authtest_%p?mode=memory&cache=shared", t)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	// A single open connection ensures all GORM operations hit the same
	// in-memory database (shared-cache mode + max=1 prevents a second
	// connection from being opened to a different in-memory instance).
	sqlDB.SetMaxOpenConns(1)

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	// Register the timestamp-conversion callback BEFORE applying the schema so
	// it is in place for any internal limen queries once the module is created.
	if err := db.Callback().Query().
		After("gorm:query").
		Register("sqlite:fix_timestamps", sqliteTimestampCallback); err != nil {
		t.Fatalf("register sqlite timestamp callback: %v", err)
	}

	// Apply schema statements one at a time (splitting on ';' is safe for DDL
	// that contains no procedural bodies or embedded semicolons).
	for _, stmt := range strings.Split(limenSchemaSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("apply schema statement %q: %v", stmt, err)
		}
	}

	module, err := auth.NewWithDB(db, testSecret, "http://localhost:8080")
	if err != nil {
		t.Fatalf("NewWithDB: %v", err)
	}
	return module
}
