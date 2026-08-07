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
	// closeFn releases the module's resources (the Postgres connection pool
	// opened by LimenModule.New). It is nil for a route-less module (failed
	// init) or a handler-injected test module, which own no resources.
	closeFn func() error
}

// Close releases the resources the module owns — the database connection pool
// wired by newLimen — so graceful shutdown does not leak connections. It
// satisfies io.Closer, which core.Serve invokes after HTTP has drained. Close
// is a no-op (returns nil) for a module that owns no resources.
func (m *Module) Close() error {
	if m.closeFn == nil {
		return nil
	}
	return m.closeFn()
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
