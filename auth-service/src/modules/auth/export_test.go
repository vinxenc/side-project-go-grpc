package auth

import "gorm.io/gorm"

// NewWithDB exposes the internal newLimen builder to the black-box test package
// (auth_test). Tests use it to hand a SQLite *gorm.DB directly to limen,
// bypassing the Postgres connection that LimenModule.New opens.
func NewWithDB(db *gorm.DB, secret []byte, baseURL string) (*Module, error) {
	return newLimen(db, secret, baseURL)
}
