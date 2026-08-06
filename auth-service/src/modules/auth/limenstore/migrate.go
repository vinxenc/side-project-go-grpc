package limenstore

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

//go:embed migrations/0001_init_limen.up.sql
var initSchemaSQL string

// Migrate applies the embedded limen schema DDL against the provided database.
// All statements use CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS, so
// Migrate is idempotent and safe to call on every startup.
//
// Statements are executed one at a time rather than as a single multi-command
// Exec: PostgreSQL's extended query protocol rejects multiple commands in one
// Exec, so splitting keeps Migrate portable across driver protocol modes. The
// embedded DDL contains no procedural bodies, so splitting on ';' is safe.
func Migrate(ctx context.Context, db *gorm.DB) error {
	db = db.WithContext(ctx)
	for _, stmt := range strings.Split(initSchemaSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("apply limen schema statement %q: %w", firstLine(stmt), err)
		}
	}
	return nil
}

// firstLine returns the first non-empty line of a statement for error context.
func firstLine(stmt string) string {
	for _, line := range strings.Split(stmt, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return s
		}
	}
	return stmt
}
