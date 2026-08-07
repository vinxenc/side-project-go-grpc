package auth_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"auth-service/src/modules/auth"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// stubHandler is a synthetic upstream standing in for limen's http.Handler. It
// returns a fixed status and body so tests can drive the controller's
// upstream-error and body-decode-failure branches deterministically.
type stubHandler struct {
	status int
	body   string
}

func (s stubHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s.status)
	_, _ = io.WriteString(w, s.body)
}

// setupStubAPI wires the auth module (backed by the given stub handler) onto a
// fresh mux via the real RegisterRoutes path.
func setupStubAPI(t *testing.T, h http.Handler) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	cfg := huma.DefaultConfig("auth-service", "1.0.0")
	cfg.CreateHooks = nil
	api := humago.New(mux, cfg)
	auth.NewWithHandler(h).Controller().RegisterRoutes(api)
	return mux
}

// authEndpoint describes one auth operation and a valid request for it (valid so
// huma's input validation passes and the handler actually runs).
type authEndpoint struct {
	name    string
	method  string
	path    string
	body    any
	decodes bool // handler decodes an upstream JSON body (has a decode-error branch)
	// successBody is a schema-valid upstream 2xx body the handler can decode,
	// used to exercise the success path. Empty for 204 (no-body) operations.
	successBody string
}

// allEndpoints enumerates every auth operation with a schema-valid request body.
func allEndpoints() []authEndpoint {
	const userBody = `{"user":{"email":"a@example.com"}}`
	return []authEndpoint{
		{"signup", http.MethodPost, "/auth/signup/credential", map[string]any{"email": "a@example.com", "password": "ValidPassword123"}, true, userBody},
		{"signin", http.MethodPost, "/auth/signin/credential", map[string]any{"credential": "a@example.com", "password": "ValidPassword123"}, true, userBody},
		{"request-reset", http.MethodPost, "/auth/passwords/request-reset", map[string]any{"email": "a@example.com"}, true, `{"message":"ok"}`},
		{"reset", http.MethodPost, "/auth/passwords/reset", map[string]any{"token": "tok", "new_password": "ValidPassword123"}, true, `{"message":"ok"}`},
		{"change", http.MethodPost, "/auth/passwords/change", map[string]any{"current_password": "OldPassword123", "new_password": "ValidPassword123"}, true, userBody},
		{"set", http.MethodPut, "/auth/passwords", map[string]any{"new_password": "ValidPassword123"}, true, userBody},
		{"username-check", http.MethodPost, "/auth/usernames/check", map[string]any{"username": "alice"}, true, `{"available":true}`},
		{"me", http.MethodGet, "/auth/me", nil, true, userBody},
		{"sessions", http.MethodGet, "/auth/sessions", nil, true, `[]`},
		{"signout", http.MethodPost, "/auth/signout", nil, false, ""},
		{"revoke-sessions", http.MethodPost, "/auth/revoke-sessions", nil, false, ""},
	}
}

// TestAuthHandlers_UpstreamSuccess verifies that when the upstream returns a
// decodable 2xx body, every handler completes successfully (2xx), covering the
// decode-success and response-assembly branches — including endpoints whose
// success path the SQLite-backed tests cannot reach (e.g. set-password).
func TestAuthHandlers_UpstreamSuccess(t *testing.T) {
	for _, ep := range allEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			status := http.StatusOK
			if !ep.decodes {
				status = http.StatusNoContent // signout/revoke: 204, no body
			}
			mux := setupStubAPI(t, stubHandler{status: status, body: ep.successBody})
			resp, body := doRequest(t, mux, ep.method, ep.path, nil, ep.body)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode >= 300 {
				t.Errorf("%s %s status = %d, want 2xx. Body: %s", ep.method, ep.path, resp.StatusCode, string(body))
			}
		})
	}
}

// TestAuthHandlers_UpstreamError verifies that when the upstream returns a 4xx,
// every handler forwards that status via errorFromRecorder.
func TestAuthHandlers_UpstreamError(t *testing.T) {
	for _, ep := range allEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			mux := setupStubAPI(t, stubHandler{status: http.StatusForbidden, body: `{"message":"denied"}`})
			resp, body := doRequest(t, mux, ep.method, ep.path, nil, ep.body)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("%s %s status = %d, want 403. Body: %s", ep.method, ep.path, resp.StatusCode, string(body))
			}
		})
	}
}

// TestAuthHandlers_UpstreamServerError verifies that a 5xx from the upstream is
// forwarded (with the internal message suppressed) rather than leaking detail.
func TestAuthHandlers_UpstreamServerError(t *testing.T) {
	for _, ep := range allEndpoints() {
		t.Run(ep.name, func(t *testing.T) {
			mux := setupStubAPI(t, stubHandler{status: http.StatusBadGateway, body: `{"message":"secret internal detail"}`})
			resp, body := doRequest(t, mux, ep.method, ep.path, nil, ep.body)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusBadGateway {
				t.Errorf("%s %s status = %d, want 502. Body: %s", ep.method, ep.path, resp.StatusCode, string(body))
			}
			if strings.Contains(string(body), "secret internal detail") {
				t.Errorf("%s %s leaked upstream 5xx detail: %s", ep.method, ep.path, string(body))
			}
		})
	}
}

// TestAuthHandlers_DecodeError verifies that when the upstream returns 2xx with
// an undecodable body, the decoding handlers surface a 500 (their decode-error
// branch) instead of panicking or returning malformed output.
func TestAuthHandlers_DecodeError(t *testing.T) {
	for _, ep := range allEndpoints() {
		if !ep.decodes {
			continue // signout/revoke return 204 with no body to decode
		}
		t.Run(ep.name, func(t *testing.T) {
			mux := setupStubAPI(t, stubHandler{status: http.StatusOK, body: `{ this is not valid json`})
			resp, body := doRequest(t, mux, ep.method, ep.path, nil, ep.body)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusInternalServerError {
				t.Errorf("%s %s status = %d, want 500 on undecodable body. Body: %s", ep.method, ep.path, resp.StatusCode, string(body))
			}
		})
	}
}
