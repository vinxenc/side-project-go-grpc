package auth

import (
	"context"
	"fmt"
	"time"

	"auth-service/configs"
	"auth-service/core"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Module is the authentication feature module. It owns the limen instance and
// the controller that registers all auth routes.
type Module struct {
	controller *controller
}

// New creates the auth module using the provided validated configuration. It
// opens a Postgres connection, verifies connectivity (5-second deadline), and
// delegates limen construction to newModule. The caller — main.go — must handle
// the returned error with log.Fatalf so the service does not start with a broken
// auth layer.
func New(cfg configs.Env) (*Module, error) {
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

	return newModule(db, cfg.Secret, cfg.BaseURL)
}

// Controller returns the controller that owns this module's routes, satisfying
// core.Module. Route registration lives on the controller (RegisterRoutes),
// next to the handlers it points at.
func (m *Module) Controller() core.Controller {
	return m.controller
}
