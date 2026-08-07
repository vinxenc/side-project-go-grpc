package core

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// NewAPI builds the auth-service Huma API bound to mux, with the service's
// OpenAPI configuration applied. Both production (main.go) and tests construct
// their API through this function so their API contract can never drift.
func NewAPI(mux *http.ServeMux) huma.API {
	return humago.New(mux, NewConfig())
}

// NewConfig returns the Huma configuration for the auth-service. It is exported
// so tests can build an API with the exact production configuration.
func NewConfig() huma.Config {
	config := huma.DefaultConfig("auth-service", "1.0.0")

	// DefaultConfig installs a SchemaLinkTransformer via CreateHooks, which
	// decorates response bodies with a "$schema" field and adds Link headers.
	// Drop it to keep response payloads byte-compatible: the health payload
	// stays {"status","time"}.
	config.CreateHooks = nil

	// Declare the cookieAuth security scheme referenced by the protected auth
	// operations so the generated OpenAPI document is valid (an operation may
	// only reference a scheme defined under components.securitySchemes).
	if config.Components == nil {
		config.Components = &huma.Components{}
	}
	if config.Components.SecuritySchemes == nil {
		config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
	}
	config.Components.SecuritySchemes["cookieAuth"] = &huma.SecurityScheme{
		Type: "apiKey",
		In:   "cookie",
		Name: "limen_session",
	}

	return config
}
