//go:build e2e

// Package tests is the shared e2e harness imported by the per-module suites
// under tests/modules/*. It is a black-box client: the auth-service runs as a
// real container (docker-compose.yml, profile "e2e") backed by a live Postgres,
// and this package only points an HTTP client at the running instance
// (BASE_URL), waits for it to become healthy, and exposes small request helpers.
package tests

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

// defaultBaseURL matches the port auth-service publishes in docker-compose.yml.
const defaultBaseURL = "http://localhost:8080"

// baseURL is the origin of the running auth-service under test, set by Ready.
var baseURL string

// readyOnce guards the one-time readiness probe so it runs once per test binary.
var readyOnce sync.Once

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

// Ready resolves BASE_URL and blocks until the service reports healthy. Every
// per-module suite calls it from TestMain; the probe runs once per process. It
// does not manage the server or schema — docker-compose owns the auth-service
// container and Postgres applies the migrations on init. On failure it aborts
// the process with a clear message so the whole suite fails fast.
func Ready() {
	readyOnce.Do(func() {
		baseURL = resolveBaseURL()
		if err := waitForHealthy(90, time.Second); err != nil {
			log.Fatalf("e2e: auth-service not healthy at %s: %v", baseURL, err)
		}
	})
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

// UniqueSuffix returns a short random hex string used to build collision-free
// emails and usernames across repeated runs against a persistent DB.
func UniqueSuffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// HTTP client helpers
// ---------------------------------------------------------------------------

// Client is a thin wrapper over http.Client with a cookie jar, so the
// limen_session cookie set on sign-up/sign-in automatically flows onto
// subsequent requests — exactly like a browser.
type Client struct {
	t    *testing.T
	http *http.Client
}

// NewClient returns a fresh client with its own cookie jar (isolated session).
func NewClient(t *testing.T) *Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &Client{t: t, http: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
}

// Response bundles the raw status and body bytes for assertions.
type Response struct {
	Status int
	Raw    []byte
}

// Decode unmarshals the response body into v, failing the test on error.
func (r Response) Decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Raw, v); err != nil {
		t.Fatalf("decode body %q: %v", string(r.Raw), err)
	}
}

// Do issues a JSON request against the running server and returns the response.
// A nil body sends no request body.
func (c *Client) Do(method, path string, body any) Response {
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
	return Response{Status: resp.StatusCode, Raw: raw}
}

// HasSessionCookie reports whether the client's jar holds a limen_session
// cookie for the server origin.
func (c *Client) HasSessionCookie() bool {
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
