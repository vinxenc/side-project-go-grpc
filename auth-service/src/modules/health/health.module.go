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
// The route is restricted to GET (Go 1.22+ method patterns); other methods
// receive a 405 Method Not Allowed. HEAD is served alongside GET automatically.
func (m *Module) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", m.controller.health)
}
