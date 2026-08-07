//go:build e2e

// Package tests — e2e harness.
//
// This file boots a real auth-service instance (health + auth modules) backed
// by a live Postgres, serves it over a loopback socket, and exposes small HTTP
// helpers used by the e2e tests. It mirrors src/main.go's API construction so
// the tests exercise the production wiring rather than a bespoke test double.
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"auth-service/core"
	"auth-service/src/modules/auth"
	"auth-service/src/modules/health"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	// defaultDatabaseURL matches docker-compose.yml and .example.env so the
	// suite runs against `docker compose up -d` locally with no extra config.
	defaultDatabaseURL = "postgres://auth:auth@localhost:5432/authdb?sslmode=disable"
	// testSecret is the publicly known 32-byte dev key from .example.env —
	// acceptable for test-only use.
	testSecret = "0123456789abcdef0123456789abcdef"
	// defaultMigrationsPath points at the committed schema relative to this
	// package's directory (the CWD when `go test` runs it).
	defaultMigrationsPath = "../migrations/0001_init_limen.up.sql"
)

// baseURL is the origin of the running e2e server, set by TestMain.
var baseURL string

// databaseURL resolves the Postgres DSN from the environment, defaulting to the
// docker-compose DSN.
func databaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	return defaultDatabaseURL
}

// TestMain applies the schema, boots the server once for the whole package, and
// tears it down afterwards. Sharing one server across tests keeps the suite
// fast; per-test isolation is achieved with unique emails/usernames instead.
func TestMain(m *testing.M) {
	dsn := databaseURL()

	if err := applyMigrations(dsn); err != nil {
		log.Fatalf("e2e: apply migrations: %v", err)
	}

	shutdown, url, err := startServer(dsn)
	if err != nil {
		log.Fatalf("e2e: start server: %v", err)
	}
	baseURL = url

	code := m.Run()
	shutdown()
	os.Exit(code)
}

// applyMigrations opens a short-lived connection, waits for Postgres to accept
// connections, then applies the committed DDL. The DDL is idempotent
// (CREATE ... IF NOT EXISTS), so re-running against a warm database is safe.
func applyMigrations(dsn string) error {
	path := os.Getenv("AUTH_MIGRATIONS_PATH")
	if path == "" {
		path = defaultMigrationsPath
	}
	schema, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read schema %q: %w", path, err)
	}

	db, err := openWithRetry(dsn, 30, time.Second)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("db handle: %w", err)
	}
	defer sqlDB.Close()

	// Apply statements one at a time (splitting on ';' is safe for this DDL,
	// which contains no procedural bodies or embedded semicolons).
	for _, stmt := range strings.Split(string(schema), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// openWithRetry opens a GORM Postgres connection and pings until it succeeds or
// attempts are exhausted, so the suite tolerates a Postgres container that is
// still starting up.
func openWithRetry(dsn string, attempts int, wait time.Duration) (*gorm.DB, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				pingErr := sqlDB.PingContext(ctx)
				cancel()
				if pingErr == nil {
					return db, nil
				}
				lastErr = pingErr
				_ = sqlDB.Close()
			} else {
				lastErr = dbErr
			}
		} else {
			lastErr = err
		}
		time.Sleep(wait)
	}
	return nil, fmt.Errorf("postgres not reachable after %d attempts: %w", attempts, lastErr)
}

// startServer wires the same Huma API as src/main.go (health + auth modules)
// and serves it on a loopback port. It returns a shutdown func and the origin
// URL. The listener is created first so the auth module's BaseURL matches the
// address cookies are actually issued on.
func startServer(dsn string) (func(), string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", fmt.Errorf("listen: %w", err)
	}
	url := "http://" + ln.Addr().String()

	mux := http.NewServeMux()
	config := huma.DefaultConfig("auth-service", "1.0.0")
	config.CreateHooks = nil // match production: keep the health payload byte-compatible
	if config.Components == nil {
		config.Components = &huma.Components{}
	}
	if config.Components.SecuritySchemes == nil {
		config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
	}
	config.Components.SecuritySchemes["cookieAuth"] = &huma.SecurityScheme{
		Type: "apiKey",
		In:   "cookie",
		Name: "limen_session",
	}
	api := humago.New(mux, config)

	authModule, err := auth.New(auth.Config{
		DatabaseURL: dsn,
		Secret:      []byte(testSecret),
		BaseURL:     url,
	})
	if err != nil {
		_ = ln.Close()
		return nil, "", fmt.Errorf("auth.New: %w", err)
	}

	core.RegisterModules(api, health.New(), authModule)

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("e2e: server error: %v", err)
		}
	}()

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
	return shutdown, url, nil
}

// ---------------------------------------------------------------------------
// HTTP client helpers
// ---------------------------------------------------------------------------

// client is a thin wrapper over http.Client with a cookie jar, so the
// limen_session cookie set on sign-up/sign-in automatically flows onto
// subsequent requests — exactly like a browser.
type client struct {
	t    *testing.T
	http *http.Client
}

// newClient returns a fresh client with its own cookie jar (isolated session).
func newClient(t *testing.T) *client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &client{t: t, http: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
}

// response bundles a decoded JSON body with the raw status and bytes for
// assertions.
type response struct {
	Status int
	Raw    []byte
}

// decode unmarshals the response body into v, failing the test on error.
func (r response) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Raw, v); err != nil {
		t.Fatalf("decode body %q: %v", string(r.Raw), err)
	}
}

// do issues a JSON request against the running server and returns the response.
// A nil body sends no request body.
func (c *client) do(method, path string, body any) response {
	c.t.Helper()

	var reader io.Reader = http.NoBody
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, baseURL+path, reader)
	if err != nil {
		c.t.Fatalf("new request %s %s: %v", method, path, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("read body %s %s: %v", method, path, err)
	}
	return response{Status: resp.StatusCode, Raw: raw}
}

// hasSessionCookie reports whether the client's jar holds a limen_session
// cookie for the server origin.
func (c *client) hasSessionCookie() bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == "limen_session" && ck.Value != "" {
			return true
		}
	}
	return false
}
