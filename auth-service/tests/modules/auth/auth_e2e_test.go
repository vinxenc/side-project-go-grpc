//go:build e2e

// End-to-end tests for the auth module, driving the HTTP API over a real socket
// against a live Postgres. Each test uses unique credentials so runs are
// independent of each other and of prior runs against a persistent database.
package auth_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"auth-service/tests"
)

// TestMain blocks until the containerized service is healthy before running the
// auth suite (see the shared harness).
func TestMain(m *testing.M) {
	tests.Ready()
	os.Exit(m.Run())
}

// signupPayload builds a valid, unique signup body and returns it plus the
// email/username/password used, so tests can reuse them for later steps.
func signupPayload(t *testing.T) (body map[string]any, email, username, password string) {
	t.Helper()
	s := tests.UniqueSuffix(t)
	email = fmt.Sprintf("user_%s@example.com", s)
	username = "user_" + s
	password = "ValidPassword123"
	body = map[string]any{"email": email, "password": password, "username": username}
	return body, email, username, password
}

// TestSignupMeSignoutFlow walks the core authenticated journey:
// sign up -> session cookie issued -> /auth/me returns the user ->
// /auth/sessions lists the session -> sign out -> /auth/me now unauthorized.
func TestSignupMeSignoutFlow(t *testing.T) {
	c := tests.NewClient(t)
	body, email, username, _ := signupPayload(t)

	// Sign up.
	resp := c.Do(http.MethodPost, "/auth/signup/credential", body)
	if resp.Status != http.StatusOK {
		t.Fatalf("signup status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}
	if !c.HasSessionCookie() {
		t.Fatal("signup did not set a limen_session cookie")
	}
	var signup struct {
		User struct {
			Email    string  `json:"email"`
			Username *string `json:"username"`
		} `json:"user"`
	}
	resp.Decode(t, &signup)
	if signup.User.Email != email {
		t.Errorf("signup user email = %q, want %q", signup.User.Email, email)
	}

	// /auth/me — cookie jar carries the session automatically.
	resp = c.Do(http.MethodGet, "/auth/me", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("me status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}
	var me struct {
		User struct {
			Email    string  `json:"email"`
			Username *string `json:"username"`
		} `json:"user"`
	}
	resp.Decode(t, &me)
	if me.User.Email != email {
		t.Errorf("me email = %q, want %q", me.User.Email, email)
	}
	if me.User.Username == nil || *me.User.Username != username {
		t.Errorf("me username = %v, want %q", me.User.Username, username)
	}

	// /auth/sessions lists at least this session.
	resp = c.Do(http.MethodGet, "/auth/sessions", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("sessions status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}
	var sessions []map[string]any
	resp.Decode(t, &sessions)
	if len(sessions) == 0 {
		t.Error("sessions returned empty list, want at least one active session")
	}

	// Sign out (204, no body).
	resp = c.Do(http.MethodPost, "/auth/signout", nil)
	if resp.Status != http.StatusNoContent {
		t.Fatalf("signout status = %d, want 204. Body: %s", resp.Status, resp.Raw)
	}

	// After sign-out the session is invalid; /auth/me must be unauthorized.
	resp = c.Do(http.MethodGet, "/auth/me", nil)
	if resp.Status != http.StatusUnauthorized {
		t.Errorf("me after signout status = %d, want 401. Body: %s", resp.Status, resp.Raw)
	}
}

// TestSigninFlow verifies signing in with valid credentials after signing up,
// and that a wrong password is rejected.
func TestSigninFlow(t *testing.T) {
	// Register with one client.
	registrar := tests.NewClient(t)
	body, email, _, password := signupPayload(t)
	if resp := registrar.Do(http.MethodPost, "/auth/signup/credential", body); resp.Status != http.StatusOK {
		t.Fatalf("signup status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}

	// Fresh client (no cookies) signs in with the right password.
	c := tests.NewClient(t)
	resp := c.Do(http.MethodPost, "/auth/signin/credential", map[string]any{
		"credential": email,
		"password":   password,
	})
	if resp.Status != http.StatusOK {
		t.Fatalf("signin status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}
	if !c.HasSessionCookie() {
		t.Fatal("signin did not set a limen_session cookie")
	}
	if resp := c.Do(http.MethodGet, "/auth/me", nil); resp.Status != http.StatusOK {
		t.Fatalf("me after signin status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}

	// Wrong password is rejected and issues no session.
	bad := tests.NewClient(t)
	resp = bad.Do(http.MethodPost, "/auth/signin/credential", map[string]any{
		"credential": email,
		"password":   "WrongPassword999",
	})
	if resp.Status < 400 {
		t.Errorf("signin with wrong password status = %d, want >= 400. Body: %s", resp.Status, resp.Raw)
	}
	if bad.HasSessionCookie() {
		t.Error("failed signin unexpectedly set a session cookie")
	}
}

// TestMeRequiresSession verifies protected endpoints reject anonymous callers.
func TestMeRequiresSession(t *testing.T) {
	c := tests.NewClient(t)
	for _, path := range []string{"/auth/me", "/auth/sessions"} {
		resp := c.Do(http.MethodGet, path, nil)
		if resp.Status != http.StatusUnauthorized {
			t.Errorf("anonymous GET %s status = %d, want 401. Body: %s", path, resp.Status, resp.Raw)
		}
	}
}

// TestUsernameAvailability verifies the availability check flips from available
// to unavailable once a username is taken.
func TestUsernameAvailability(t *testing.T) {
	c := tests.NewClient(t)
	body, _, username, _ := signupPayload(t)

	// Before registration the username is available.
	resp := c.Do(http.MethodPost, "/auth/usernames/check", map[string]any{"username": username})
	if resp.Status != http.StatusOK {
		t.Fatalf("username check status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}
	var before struct {
		Available bool `json:"available"`
	}
	resp.Decode(t, &before)
	if !before.Available {
		t.Errorf("username %q reported unavailable before signup", username)
	}

	// Register the username.
	if resp := c.Do(http.MethodPost, "/auth/signup/credential", body); resp.Status != http.StatusOK {
		t.Fatalf("signup status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}

	// Now it must be unavailable.
	resp = c.Do(http.MethodPost, "/auth/usernames/check", map[string]any{"username": username})
	if resp.Status != http.StatusOK {
		t.Fatalf("username re-check status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}
	var after struct {
		Available bool `json:"available"`
	}
	resp.Decode(t, &after)
	if after.Available {
		t.Errorf("username %q reported available after signup", username)
	}
}

// TestChangePasswordFlow signs up, changes the password while authenticated,
// then confirms the new password works and the old one does not.
func TestChangePasswordFlow(t *testing.T) {
	c := tests.NewClient(t)
	body, email, _, oldPassword := signupPayload(t)
	const newPassword = "BrandNewPassword456"

	if resp := c.Do(http.MethodPost, "/auth/signup/credential", body); resp.Status != http.StatusOK {
		t.Fatalf("signup status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}

	// Change password using the authenticated session.
	resp := c.Do(http.MethodPost, "/auth/passwords/change", map[string]any{
		"current_password": oldPassword,
		"new_password":     newPassword,
	})
	if resp.Status != http.StatusOK {
		t.Fatalf("change-password status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}

	// Old password no longer signs in.
	old := tests.NewClient(t)
	if resp := old.Do(http.MethodPost, "/auth/signin/credential", map[string]any{
		"credential": email, "password": oldPassword,
	}); resp.Status < 400 {
		t.Errorf("signin with old password status = %d, want >= 400. Body: %s", resp.Status, resp.Raw)
	}

	// New password signs in successfully.
	fresh := tests.NewClient(t)
	resp = fresh.Do(http.MethodPost, "/auth/signin/credential", map[string]any{
		"credential": email, "password": newPassword,
	})
	if resp.Status != http.StatusOK {
		t.Fatalf("signin with new password status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}
	if !fresh.HasSessionCookie() {
		t.Error("signin with new password did not set a session cookie")
	}
}

// TestRequestPasswordReset verifies the request-reset endpoint responds 200
// (it always returns 200 to avoid user enumeration, per the API contract),
// both for a registered address and an unknown one.
func TestRequestPasswordReset(t *testing.T) {
	c := tests.NewClient(t)
	body, email, _, _ := signupPayload(t)
	if resp := c.Do(http.MethodPost, "/auth/signup/credential", body); resp.Status != http.StatusOK {
		t.Fatalf("signup status = %d, want 200. Body: %s", resp.Status, resp.Raw)
	}

	for _, target := range []string{email, "nobody_" + tests.UniqueSuffix(t) + "@example.com"} {
		resp := c.Do(http.MethodPost, "/auth/passwords/request-reset", map[string]any{"email": target})
		if resp.Status != http.StatusOK {
			t.Errorf("request-reset(%q) status = %d, want 200. Body: %s", target, resp.Status, resp.Raw)
		}
	}
}

// TestSignupValidation verifies invalid signup bodies are rejected by the
// schema layer (short password, missing/invalid email).
func TestSignupValidation(t *testing.T) {
	c := tests.NewClient(t)
	cases := []struct {
		name string
		body map[string]any
	}{
		{"short password", map[string]any{"email": "a@example.com", "password": "short"}},
		{"missing email", map[string]any{"password": "ValidPassword123"}},
		{"invalid email", map[string]any{"email": "not-an-email", "password": "ValidPassword123"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := c.Do(http.MethodPost, "/auth/signup/credential", tc.body)
			if resp.Status != http.StatusUnprocessableEntity {
				t.Errorf("signup(%s) status = %d, want 422. Body: %s", tc.name, resp.Status, resp.Raw)
			}
		})
	}
}
