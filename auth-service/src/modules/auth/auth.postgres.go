package auth

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// This file holds the Postgres connection bootstrap for the auth module. It
// opens a real database and is therefore exercised by the e2e suite rather than
// unit tests (a live Postgres is required to reach the connection-setup and
// ping paths); the unit coverage report excludes this file for that reason.
// Pure, in-memory wiring lives in auth.limen.go (newLimen), which IS unit-tested.

// LimenConfig holds the inputs LimenModule.New needs to open Postgres and wire
// the limen instance.
type LimenConfig struct {
	DatabaseURL string // Postgres DSN/URL
	Secret      []byte // 32-byte signing secret
	BaseURL     string // base URL used for cookies/links
}

// limenModule is a stateless namespace type; its only purpose is to provide the
// LimenModule.New(...) constructor call syntax.
type limenModule struct{}

// LimenModule builds the limen layer of the auth module:
//
//	auth.LimenModule.New(auth.LimenConfig{...})
var LimenModule limenModule

// New opens a Postgres connection from cfg.DatabaseURL, verifies connectivity
// (5-second deadline), and delegates the limen wiring to newLimen. The caller
// must handle the returned error (fail fast) so the service does not start with
// a broken auth layer.
func (limenModule) New(cfg LimenConfig) (*Module, error) {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return newLimen(db, cfg.Secret, cfg.BaseURL)
}
