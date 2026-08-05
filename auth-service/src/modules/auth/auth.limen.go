package auth

import (
	"context"
	"fmt"
	"log"
	"os"

	"auth-service/src/modules/auth/limenstore"

	credentialpassword "github.com/thecodearcher/limen/plugins/credential-password"

	"github.com/thecodearcher/limen"
)

// devSecret is a fixed 32-byte key used only when LIMEN_SECRET is unset.
// It is publicly known (committed in source) and MUST NOT be used in any
// shared or production environment. resolveSecret logs a prominent warning
// whenever this fallback is active.
var devSecret = []byte("0123456789abcdef0123456789abcdef")

// newDatabaseAdapter selects the limen DatabaseAdapter based on the presence of
// the DATABASE_URL environment variable:
//   - DATABASE_URL set → opens a Postgres connection via the GORM adapter, runs
//     idempotent schema migrations, and returns the adapter.
//   - DATABASE_URL unset/empty → returns the in-memory adapter (current behaviour).
//
// The chosen backend is logged at startup so it is never silent.
func newDatabaseAdapter() (limen.DatabaseAdapter, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Println("INFO: DATABASE_URL not set — using in-memory store (data lost on restart)")
		return limenstore.NewMemoryAdapter(), nil
	}

	log.Println("INFO: DATABASE_URL set — connecting to Postgres")
	adapter, _, err := limenstore.NewGormAdapter(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres adapter: %w", err)
	}
	log.Println("INFO: Postgres adapter ready, migrations applied")
	return adapter, nil
}

// newLimen builds a fully configured limen instance (Core + credential-password
// plugin). The database adapter is chosen by newDatabaseAdapter: Postgres when
// DATABASE_URL is set, in-memory otherwise.
//
// Production considerations (iteration-scope choices left as-is):
//   - CSRF and origin checks are disabled; re-enable (WithHTTPCSRFProtection(true),
//     WithHTTPOriginCheck(true), WithHTTPTrustedOrigins) for production.
//   - CookieSecure is false; flip to true when serving over TLS.
func newLimen() (*limen.Limen, error) {
	secret, err := resolveSecret()
	if err != nil {
		return nil, fmt.Errorf("limen secret: %w", err)
	}

	baseURL := os.Getenv("AUTH_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	adapter, err := newDatabaseAdapter()
	if err != nil {
		return nil, fmt.Errorf("limen database adapter: %w", err)
	}

	plugin := credentialpassword.New(
		credentialpassword.WithUsernameSupport(true),
	)

	auth, err := limen.New(&limen.Config{
		BaseURL:  baseURL,
		Secret:   secret,
		Database: adapter,
		Plugins:  []limen.Plugin{plugin},
		HTTP: limen.NewDefaultHTTPConfig(
			limen.WithHTTPBasePath("/auth"),
			// CSRF and origin checks are disabled for this iteration to allow
			// the in-process synthetic requests (no real Origin header) and
			// localhost HTTP testing. Re-enable for production deployments.
			limen.WithHTTPCSRFProtection(false),
			limen.WithHTTPOriginCheck(false),
			// CookieSecure=false permits cookie issuance over plain HTTP on
			// localhost. Flip to true when serving over TLS in production.
			limen.WithHTTPCookieSecure(false),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("limen.New: %w", err)
	}

	return auth, nil
}

// resolveSecret returns a validated 32-byte signing secret. It reads
// LIMEN_SECRET from the environment; if the variable is empty the dev fallback
// is returned WITH A PROMINENT WARNING — the dev secret is publicly known and
// provides no security. If LIMEN_SECRET is set but is not exactly 32 bytes an
// error is returned so the caller can fail fast.
func resolveSecret() ([]byte, error) {
	raw := os.Getenv("LIMEN_SECRET")
	if raw == "" {
		log.Println("WARNING: LIMEN_SECRET is not set. Using the hardcoded dev secret " +
			"(publicly known, zero security). Set LIMEN_SECRET to a random 32-byte value " +
			"in any non-local environment.")
		return devSecret, nil
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("LIMEN_SECRET must be exactly 32 bytes, got %d", len(raw))
	}
	return []byte(raw), nil
}
