//go:build e2e

// End-to-end tests for the health module, driving the HTTP API over a real
// socket against the running containerized service.
package health_test

import (
	"net/http"
	"os"
	"testing"

	"auth-service/tests"
)

// TestMain blocks until the containerized service is healthy before running the
// health suite (see the shared harness).
func TestMain(m *testing.M) {
	tests.Ready()
	os.Exit(m.Run())
}

// TestHealthEndpoint verifies the service is reachable and reports OK with a
// timestamp.
func TestHealthEndpoint(t *testing.T) {
	c := tests.NewClient(t)
	resp := c.Do(http.MethodGet, "/health", nil)

	if resp.Status != http.StatusOK {
		t.Fatalf("GET /health status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}
	var out struct {
		Status string `json:"status"`
		Time   string `json:"time"`
	}
	resp.Decode(t, &out)
	if out.Status != "ok" {
		t.Errorf("health status = %q, want %q", out.Status, "ok")
	}
	if out.Time == "" {
		t.Error("health response missing time")
	}
}
