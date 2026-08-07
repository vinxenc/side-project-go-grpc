package auth

import "auth-service/src/core"

// Config holds the inputs required to construct the auth module. It is the auth
// package's own contract, decoupled from the setting package; main.go maps
// setting.Setting onto it.
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
// (LimenModule.New), which opens Postgres and wires limen. It never returns an
// error: on failure LimenModule.New logs the cause and yields a route-less
// Module, so main.go can inline it into core.RegisterModules.
func New(cfg Config) *Module {
	return LimenModule.New(LimenConfig(cfg))
}

// Controller returns the controller that owns this module's routes, satisfying
// core.Module. Route registration lives on the controller (RegisterRoutes),
// next to the handlers it points at.
//
// A Module whose backend failed to build has a nil controller; this returns an
// untyped nil interface in that case (not a non-nil interface wrapping a nil
// *controller) so RegisterModules correctly skips it and registers no routes.
func (m *Module) Controller() core.Controller {
	if m.controller == nil {
		return nil
	}
	return m.controller
}
