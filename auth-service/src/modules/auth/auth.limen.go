package auth

import (
	"context"
	"fmt"
	"log"
	"time"

	gormadapter "github.com/thecodearcher/limen/adapters/gorm"
	credentialpassword "github.com/thecodearcher/limen/plugins/credential-password"

	"github.com/thecodearcher/limen"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// This file is the auth module's limen/Postgres integration layer. It opens a
// real database (LimenModule.New) and wires the limen instance (newLimen).
// Because it requires a live Postgres to exercise, it is covered by the e2e
// suite (tests/) rather than unit tests, and .testcoverage.yml excludes it from
// the unit-coverage report for that reason.

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

// New opens the database (openDB) and delegates the limen wiring to newLimen. On
// any failure it logs the cause and returns a route-less Module (nil controller)
// so the service still starts; the auth endpoints are simply not registered.
func (limenModule) New(cfg LimenConfig) *Module {
	db, err := openDB(cfg.DatabaseURL)
	if err != nil {
		log.Printf("auth: %v", err)
		return &Module{}
	}

	module, err := newLimen(db, cfg.Secret, cfg.BaseURL)
	if err != nil {
		log.Printf("auth: wire limen: %v", err)
		return &Module{}
	}
	return module
}

// openDB opens a Postgres connection from databaseURL, configures the connection
// pool, and verifies connectivity with a 5-second ping. It returns a ready
// *gorm.DB, or an error identifying which step failed.
func openDB(databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
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

	return db, nil
}

// newLimen wires a fully configured limen instance from an already-open
// *gorm.DB and returns a Module wrapping the limen handler. It is the shared
// builder used by both LimenModule.New (which opens Postgres) and the test seam
// NewWithDB (which hands in a SQLite DB).
//
// Production considerations (iteration-scope choices left as-is):
//   - CSRF and origin checks are disabled; re-enable for production.
//   - CookieSecure is false; flip to true when serving over TLS.
func newLimen(db *gorm.DB, secret []byte, baseURL string) (*Module, error) {
	// Official GORM adapter: New(db *gorm.DB) *Adapter.
	adapter := gormadapter.New(db)

	config := &limen.Config{
		BaseURL:  baseURL,
		Secret:   secret,
		Database: adapter,
		Plugins: []limen.Plugin{
			credentialpassword.New(
				credentialpassword.WithUsernameSupport(true),
			),
		},
		HTTP: limen.NewDefaultHTTPConfig(
			limen.WithHTTPBasePath("/auth"),
			// CSRF and origin checks are disabled for this iteration to allow
			// the in-process synthetic requests and localhost HTTP testing.
			// Re-enable for production deployments.
			limen.WithHTTPCSRFProtection(false),
			limen.WithHTTPOriginCheck(false),
			// CookieSecure=false permits cookie issuance over plain HTTP on
			// localhost. Flip to true when serving over TLS in production.
			limen.WithHTTPCookieSecure(false),
		),
	}

	lm, err := limen.New(config)
	if err != nil {
		return nil, fmt.Errorf("limen.New: %w", err)
	}

	return &Module{controller: &controller{handler: lm.Handler()}}, nil
}
