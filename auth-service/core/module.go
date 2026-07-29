// Package core provides the shared building blocks for the auth-service,
// including the module abstraction used to wire feature modules to the server.
package core

import "net/http"

// Module is anything that can register its HTTP routes onto a mux.
// Each feature module under src/modules implements this interface.
type Module interface {
	RegisterRoutes(mux *http.ServeMux)
}

// RegisterModules registers the endpoints of every given module onto the mux.
func RegisterModules(mux *http.ServeMux, modules ...Module) {
	for _, m := range modules {
		m.RegisterRoutes(mux)
	}
}
