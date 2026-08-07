package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"auth-service/src/core"
	"auth-service/src/modules/health"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// setupHealthAPI wires the health module onto a fresh Huma API + mux, mirroring
// the production configuration (CreateHooks disabled so the payload stays
// {"status","time"}).
func setupHealthAPI(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	cfg := huma.DefaultConfig("auth-service", "1.0.0")
	cfg.CreateHooks = nil
	api := humago.New(mux, cfg)

	module := health.New()
	if module.Controller() == nil {
		t.Fatal("health controller is nil")
	}
	// Exercise the module via the shared registrar, covering RegisterModules and
	// the module's RegisterRoutes wiring.
	core.RegisterModules(api, module)
	return mux
}

// TestHealth_HappyPath verifies GET /health returns 200 with status "ok" and a
// recent UTC timestamp.
func TestHealth_HappyPath(t *testing.T) {
	mux := setupHealthAPI(t)

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Status string    `json:"status"`
		Time   time.Time `json:"time"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want %q", body.Status, "ok")
	}
	if body.Time.IsZero() {
		t.Error("time is zero, want a real timestamp")
	}
	if body.Time.Location() != time.UTC {
		t.Errorf("time location = %v, want UTC", body.Time.Location())
	}
	if d := time.Since(body.Time); d < 0 || d > time.Minute {
		t.Errorf("time = %v is not recent (delta %v)", body.Time, d)
	}
}

// TestHealth_ResponseIsByteCompatible ensures no extra fields (e.g. a "$schema"
// link from Huma's CreateHooks) leak into the payload.
func TestHealth_ResponseIsByteCompatible(t *testing.T) {
	mux := setupHealthAPI(t)

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	for k := range raw {
		if k != "status" && k != "time" {
			t.Errorf("unexpected field %q in health payload: %s", k, rec.Body.String())
		}
	}
}
