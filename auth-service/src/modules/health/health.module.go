package health

import "auth-service/core"

// Module is the health-check module.
type Module struct {
	controller *controller
}

// New creates the health module.
func New() *Module {
	return &Module{controller: &controller{}}
}

// Controller returns the controller that owns this module's routes, satisfying
// core.Module. Route registration lives on the controller (RegisterRoutes),
// next to the handlers it points at.
func (m *Module) Controller() core.Controller {
	return m.controller
}
