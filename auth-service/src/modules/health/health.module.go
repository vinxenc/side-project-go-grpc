package health

import "net/http"

// Module is the health-check module.
type Module struct {
	controller *controller
}

// New creates the health module.
func New() *Module {
	return &Module{controller: &controller{}}
}

// RegisterRoutes wires the health module's routes onto the given mux.
func (m *Module) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", m.controller.health)
}
