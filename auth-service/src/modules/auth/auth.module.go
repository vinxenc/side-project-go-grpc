package auth

import "auth-service/core"

// Module is the authentication feature module. It owns the limen instance and
// the controller that registers all auth routes.
type Module struct {
	controller *controller
}

// New creates the auth module. It initialises limen (Core + credential-password
// plugin + in-memory adapter) and returns an error if construction fails (e.g.
// invalid LIMEN_SECRET or missing adapter). The caller — main.go — must handle
// the error with log.Fatalf so the service does not start with a broken auth
// layer.
func New() (*Module, error) {
	limenInstance, err := newLimen()
	if err != nil {
		return nil, err
	}

	return &Module{
		controller: &controller{
			handler: limenInstance.Handler(),
		},
	}, nil
}

// Controller returns the controller that owns this module's routes, satisfying
// core.Module. Route registration lives on the controller (RegisterRoutes),
// next to the handlers it points at.
func (m *Module) Controller() core.Controller {
	return m.controller
}
