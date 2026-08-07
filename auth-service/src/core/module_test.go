package core_test

import (
	"net/http"
	"testing"

	"auth-service/src/core"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// recordingController records whether RegisterRoutes was invoked.
type recordingController struct{ registered bool }

func (c *recordingController) RegisterRoutes(huma.API) { c.registered = true }

// fakeModule returns a fixed controller (possibly nil) to exercise both
// branches of RegisterModules.
type fakeModule struct{ ctrl core.Controller }

func (m fakeModule) Controller() core.Controller { return m.ctrl }

// TestRegisterModules_RegistersNonNilControllers verifies that modules with a
// controller get their routes registered and modules returning a nil controller
// are skipped without panicking.
func TestRegisterModules_RegistersNonNilControllers(t *testing.T) {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("test", "1.0.0"))

	withRoutes := &recordingController{}
	modules := []core.Module{
		fakeModule{ctrl: withRoutes},
		fakeModule{ctrl: nil}, // must be skipped, not dereferenced
	}

	core.RegisterModules(api, modules...)

	if !withRoutes.registered {
		t.Error("expected RegisterRoutes to be called for the module with a controller")
	}
}

// TestRegisterModules_NoModules verifies the no-op case (empty variadic).
func TestRegisterModules_NoModules(t *testing.T) {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("test", "1.0.0"))
	core.RegisterModules(api) // must not panic
}
