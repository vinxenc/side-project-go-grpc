// Package core provides the shared building blocks for the auth-service,
// including the module abstraction used to wire feature modules to the server.
package core

import (
	"fmt"

	"github.com/danielgtaylor/huma/v2"
)

// Controller is anything that can register its HTTP routes onto a Huma API.
// Each feature module's controller (under src/modules) implements this
// interface, keeping route wiring next to the handlers it points at.
type Controller interface {
	RegisterRoutes(api huma.API)
}

// Module is a feature module that exposes the controller owning its routes.
// Each feature module under src/modules implements this interface.
//
// Controller must return a non-nil Controller; RegisterModules panics
// otherwise so a mis-wired module fails loudly at startup.
type Module interface {
	Controller() Controller
}

// RegisterModules registers the endpoints of every given module onto the API
// by delegating to each module's controller. It panics if a module returns a
// nil Controller, turning a wiring mistake into a clear startup failure rather
// than an opaque nil-pointer dereference.
func RegisterModules(api huma.API, modules ...Module) {
	for _, m := range modules {
		c := m.Controller()
		if c == nil {
			panic(fmt.Sprintf("core: module %T returned a nil Controller", m))
		}
		c.RegisterRoutes(api)
	}
}
