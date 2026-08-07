package auth

import (
	"net/http"

	"gorm.io/gorm"
)

// NewWithDB exposes the internal newLimen builder to the black-box test package
// (auth_test). Tests use it to hand a SQLite *gorm.DB directly to limen,
// bypassing the Postgres connection that LimenModule.New opens.
func NewWithDB(db *gorm.DB, secret []byte, baseURL string) (*Module, error) {
	return newLimen(db, secret, baseURL)
}

// NewWithHandler builds a Module whose controller delegates to the given
// http.Handler instead of a real limen instance. Test-only: it lets black-box
// tests inject synthetic upstream responses (arbitrary status codes and bodies)
// to exercise the controller's success, upstream-error, and body-decode-failure
// branches deterministically, without a database.
func NewWithHandler(h http.Handler) *Module {
	return &Module{controller: &controller{handler: h}}
}
