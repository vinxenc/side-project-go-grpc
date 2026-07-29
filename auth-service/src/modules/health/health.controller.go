package health

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// controller holds the health endpoint handlers.
type controller struct{}

// RegisterRoutes wires the health module's routes onto the Huma API.
// huma.Get registers a GET operation; other methods on the same path receive a
// 405 Method Not Allowed, preserving the endpoint's GET-only contract.
func (c *controller) RegisterRoutes(api huma.API) {
	huma.Get(api, "/health", c.health)
}

// health reports whether the service is up.
func (c *controller) health(ctx context.Context, _ *struct{}) (*healthOutput, error) {
	return &healthOutput{Body: response{
		Status: "ok",
		Time:   time.Now().UTC(),
	}}, nil
}
