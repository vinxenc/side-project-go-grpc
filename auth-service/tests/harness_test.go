//go:build e2e

// Package tests — e2e harness.
//
// Black-box harness: the auth-service runs as a real container (see
// docker-compose.yml, profile "e2e") backed by a live Postgres. This file does
// NOT start the server in-process — it only points an HTTP client at the
// running instance (BASE_URL), waits for it to become healthy, and exposes
// small request helpers used by the e2e tests.
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"testing"
	"time"
)

// defaultBaseURL matches the port auth-service publishes in docker-compose.yml.
const defaultBaseURL = "http://localhost:8080"

// baseURL is the origin of the running auth-service under test, resolved by
// TestMain from BASE_URL (falling back to defaultBaseURL).
var baseURL string

// resolveBaseURL reads BASE_URL from the environment, defaulting to the
// compose-published address, and strips any trailing slash.
func resolveBaseURL() string {
	v := os.Getenv("BASE_URL")
	if v == "" {
		v = defaultBaseURL
	}
	for len(v) > 0 && v[len(v)-1] == '/' {
		v = v[:len(v)-1]
	}
	return v
}

// TestMain resolves the target, waits for the service to report healthy, then
// runs the suite. It does not manage the server or the schema — docker-compose
// owns the auth-service container and Postgres applies the migrations on init.
func TestMain(m *testing.M) {
	baseURL = resolveBaseURL()

	if err := waitForHealthy(90, time.Second); err != nil {
		log.Fatalf("e2e: auth-service not healthy at %s: %v", baseURL, err)
	}

	os.Exit(m.Run())
}

// waitForHealthy polls GET /health until it returns 200 or attempts run out, so
// the suite tolerates a container that is still starting.
func waitForHealthy(attempts int, wait time.Duration) error {
	httpc := &http.Client{Timeout: 3 * time.Second}
	var lastErr error
	for i := 0; i < attempts; i++ {
		resp, err := httpc.Get(baseURL + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("health status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(wait)
	}
	return fmt.Errorf("after %d attempts: %w", attempts, lastErr)
}

// ---------------------------------------------------------------------------
// HTTP client helpers
// ---------------------------------------------------------------------------

// client is a thin wrapper over http.Client with a cookie jar, so the
// limen_session cookie set on sign-up/sign-in automatically flows onto
// subsequent requests — exactly like a browser.
type client struct {
	t    *testing.T
	http *http.Client
}

// newClient returns a fresh client with its own cookie jar (isolated session).
func newClient(t *testing.T) *client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &client{t: t, http: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
}

// response bundles a decoded JSON body with the raw status and bytes for
// assertions.
type response struct {
	Status int
	Raw    []byte
}

// decode unmarshals the response body into v, failing the test on error.
func (r response) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Raw, v); err != nil {
		t.Fatalf("decode body %q: %v", string(r.Raw), err)
	}
}

// do issues a JSON request against the running server and returns the response.
// A nil body sends no request body.
func (c *client) do(method, path string, body any) response {
	c.t.Helper()

	var reader io.Reader = http.NoBody
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, baseURL+path, reader)
	if err != nil {
		c.t.Fatalf("new request %s %s: %v", method, path, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("read body %s %s: %v", method, path, err)
	}
	return response{Status: resp.StatusCode, Raw: raw}
}

// hasSessionCookie reports whether the client's jar holds a limen_session
// cookie for the server origin.
func (c *client) hasSessionCookie() bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == "limen_session" && ck.Value != "" {
			return true
		}
	}
	return false
}
