package limenstore

import (
	"context"
	_ "embed"
	"fmt"

	"gorm.io/gorm"
)

//go:embed migrations/0001_init_limen.up.sql
var initSchemaSQL string

// Migrate applies the embedded limen schema DDL against the provided database.
// All statements use CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS, so
// Migrate is idempotent and safe to call on every startup.
func Migrate(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).Exec(initSchemaSQL).Error; err != nil {
		return fmt.Errorf("apply limen schema: %w", err)
	}
	return nil
}
