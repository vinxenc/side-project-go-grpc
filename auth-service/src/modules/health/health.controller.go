package health

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// controller holds the health endpoint handlers.
type controller struct{}

// RegisterRoutes wires the health module's routes onto the Huma API.
//
// Routes are registered with huma.Register and an explicit huma.Operation so
// every field (OperationID, Summary, Method, Path, Tags) is spelled out. Prefer
// this form over the huma.Get/huma.Post shortcuts: the OperationID is pinned by
// hand (stable for generated clients even if the path changes) and the operation
// is self-documenting for anyone adding the next endpoint.
func (c *controller) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-health",
		Summary:     "Get health",
		Description: "Reports whether the service is up.",
		Method:      http.MethodGet,
		Path:        "/health",
		Tags:        []string{"Health"},
	}, c.health)
}

// health reports whether the service is up.
func (c *controller) health(ctx context.Context, _ *struct{}) (*healthOutput, error) {
	return &healthOutput{Body: response{
		Status: "ok",
		Time:   time.Now().UTC(),
	}}, nil
}
