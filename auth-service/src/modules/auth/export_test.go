package auth

import "gorm.io/gorm"

// NewWithDB exposes the unexported newModule builder to the black-box test
// package (auth_test). Tests use it to hand a SQLite *gorm.DB directly to
// limen, bypassing the Postgres connection that New() requires.
func NewWithDB(db *gorm.DB, secret []byte, baseURL string) (*Module, error) {
	return newModule(db, secret, baseURL)
}
