package auth

import "auth-service/core"

// Config holds the inputs required to construct the auth module. It is the auth
// package's own contract, decoupled from the configs package; main.go maps
// configs.Setting onto it.
type Config struct {
	DatabaseURL string // Postgres DSN/URL
	Secret      []byte // 32-byte signing secret
	BaseURL     string // base URL used for cookies/links
}

// Module is the authentication feature module. It owns the limen instance and
// the controller that registers all auth routes. Construct it via New
// (production) or NewWithDB (tests).
type Module struct {
	controller *controller
}

// New constructs the auth module from cfg by delegating to the limen layer
// (LimenModule.New), which opens Postgres and wires limen. The caller — main.go
// — must handle the returned error with log.Fatalf so the service does not
// start with a broken auth layer.
func New(cfg Config) (*Module, error) {
	return LimenModule.New(LimenConfig{
		DatabaseURL: cfg.DatabaseURL,
		Secret:      cfg.Secret,
		BaseURL:     cfg.BaseURL,
	})
}

// Controller returns the controller that owns this module's routes, satisfying
// core.Module. Route registration lives on the controller (RegisterRoutes),
// next to the handlers it points at.
func (m *Module) Controller() core.Controller {
	return m.controller
}
