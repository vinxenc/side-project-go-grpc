package limenstore

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/thecodearcher/limen"
	gormadapter "github.com/thecodearcher/limen/adapters/gorm"
)

// openGormDB opens a Postgres connection from dsn, configures the connection
// pool, and verifies connectivity via PingContext with a 5-second deadline.
func openGormDB(ctx context.Context, dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("db handle: %w", err)
	}

	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

// NewGormAdapter opens a Postgres connection from dsn, verifies connectivity,
// applies the limen schema migrations, and returns a limen DatabaseAdapter
// backed by GORM. The *gorm.DB is also returned so the caller may close it or
// inspect it; the caller is responsible for closing the underlying connection
// when it is no longer needed.
func NewGormAdapter(ctx context.Context, dsn string) (limen.DatabaseAdapter, *gorm.DB, error) {
	db, err := openGormDB(ctx, dsn)
	if err != nil {
		return nil, nil, err
	}

	if err := Migrate(ctx, db); err != nil {
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}

	return gormadapter.New(db), db, nil
}
