package auth

import (
	"fmt"

	gormadapter "github.com/thecodearcher/limen/adapters/gorm"
	credentialpassword "github.com/thecodearcher/limen/plugins/credential-password"

	"github.com/thecodearcher/limen"
	"gorm.io/gorm"
)

// newModule builds a fully configured limen instance from an already-open
// *gorm.DB and returns a Module wrapping the limen handler. It is the shared
// builder used by both the production New (which opens Postgres) and the test
// seam NewWithDB (which hands in a SQLite DB).
//
// Production considerations (iteration-scope choices left as-is):
//   - CSRF and origin checks are disabled; re-enable for production.
//   - CookieSecure is false; flip to true when serving over TLS.
func newModule(db *gorm.DB, secret []byte, baseURL string) (*Module, error) {
	plugin := credentialpassword.New(
		credentialpassword.WithUsernameSupport(true),
	)

	lm, err := limen.New(&limen.Config{
		BaseURL:  baseURL,
		Secret:   secret,
		Database: gormadapter.New(db), // official GORM adapter: New(db *gorm.DB) *Adapter
		Plugins:  []limen.Plugin{plugin},
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
	})
	if err != nil {
		return nil, fmt.Errorf("limen.New: %w", err)
	}

	return &Module{controller: &controller{handler: lm.Handler()}}, nil
}
