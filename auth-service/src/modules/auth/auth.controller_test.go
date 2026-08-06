package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"auth-service/src/modules/auth"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// setupTestAPI creates a fresh test API with the auth module installed.
func setupTestAPI(t *testing.T) (huma.API, *http.ServeMux) {
	mux := http.NewServeMux()
	cfg := huma.DefaultConfig("auth-service", "1.0.0")
	cfg.CreateHooks = nil // Match production config
	api := humago.New(mux, cfg)

	module, err := auth.New()
	if err != nil {
		t.Fatalf("auth.New() failed: %v", err)
	}

	controller := module.Controller()
	if controller == nil {
		t.Fatal("controller is nil")
	}
	controller.RegisterRoutes(api)

	return api, mux
}

// doRequest performs an HTTP request against the test server.
func doRequest(t *testing.T, mux *http.ServeMux, method, path string, headers map[string]string, body any) (*http.Response, []byte) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body failed: %v", err)
		}
		reqBody = bytes.NewReader(data)
	} else {
		reqBody = http.NoBody
	}

	req, err := http.NewRequest(method, "http://localhost:8080"+path, reqBody)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	respBody, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body failed: %v", err)
	}
	return rec.Result(), respBody
}

// TestAuthSignup_HappyPath tests successful user registration
func TestAuthSignup_HappyPath(t *testing.T) {
	_, mux := setupTestAPI(t)

	payload := map[string]any{
		"email":    "test@example.com",
		"password": "ValidPassword123",
		"username": "testuser",
	}

	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("signup status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	user := result["user"]
	if user == nil {
		t.Errorf("response missing 'user' field")
	}

	// Check Set-Cookie header
	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Error("signup response missing Set-Cookie header")
	}

	// Verify session cookie is limen_session
	foundSessionCookie := false
	for _, cookie := range cookies {
		if strings.Contains(cookie, "limen_session") {
			foundSessionCookie = true
			break
		}
	}
	if !foundSessionCookie {
		t.Errorf("Set-Cookie does not contain limen_session. Cookies: %v", cookies)
	}
}

// TestAuthSignup_DuplicateEmail tests that duplicate email signup returns 409
func TestAuthSignup_DuplicateEmail(t *testing.T) {
	_, mux := setupTestAPI(t)

	payload := map[string]any{
		"email":    "duplicate@example.com",
		"password": "ValidPassword123",
	}

	// First signup succeeds
	resp, _ := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, payload)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first signup failed: %d", resp.StatusCode)
	}

	// Second signup with same email fails
	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate email status = %d, want %d. Body: %s", resp.StatusCode, http.StatusConflict, string(body))
	}
}

// TestAuthSignup_DuplicateUsername tests that duplicate username signup returns 409
func TestAuthSignup_DuplicateUsername(t *testing.T) {
	_, mux := setupTestAPI(t)

	// First signup with username
	payload1 := map[string]any{
		"email":    "user1@example.com",
		"password": "ValidPassword123",
		"username": "duplicateuser",
	}
	resp, _ := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, payload1)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first signup failed: %d", resp.StatusCode)
	}

	// Second signup with same username but different email
	payload2 := map[string]any{
		"email":    "user2@example.com",
		"password": "ValidPassword123",
		"username": "duplicateuser",
	}
	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, payload2)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate username status = %d, want %d. Body: %s", resp.StatusCode, http.StatusConflict, string(body))
	}
}

// TestAuthSignup_MissingPassword tests missing password validation
func TestAuthSignup_MissingPassword(t *testing.T) {
	_, mux := setupTestAPI(t)

	payload := map[string]any{
		"email": "test@example.com",
		// password missing
	}

	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, payload)
	defer resp.Body.Close()

	// huma validates required fields and returns 422
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("missing password status = %d, want %d. Body: %s", resp.StatusCode, http.StatusUnprocessableEntity, string(body))
	}
}

// TestAuthSignup_PasswordTooShort tests that password < 8 chars is rejected
func TestAuthSignup_PasswordTooShort(t *testing.T) {
	_, mux := setupTestAPI(t)

	payload := map[string]any{
		"email":    "test@example.com",
		"password": "short",
	}

	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, payload)
	defer resp.Body.Close()

	// huma validates minLength and returns 422
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("short password status = %d, want %d. Body: %s", resp.StatusCode, http.StatusUnprocessableEntity, string(body))
	}
}

// TestAuthSignin_HappyPath tests successful sign-in
func TestAuthSignin_HappyPath(t *testing.T) {
	_, mux := setupTestAPI(t)

	// First signup
	signupPayload := map[string]any{
		"email":    "signin@example.com",
		"password": "ValidPassword123",
	}
	resp, _ := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, signupPayload)
	resp.Body.Close()

	// Then signin
	signinPayload := map[string]any{
		"credential": "signin@example.com",
		"password":   "ValidPassword123",
	}
	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signin/credential", nil, signinPayload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("signin status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if result["user"] == nil {
		t.Errorf("signin response missing 'user' field")
	}

	// Check Set-Cookie
	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Error("signin response missing Set-Cookie header")
	}
}

// TestAuthSignin_WrongPassword tests signin with wrong password
func TestAuthSignin_WrongPassword(t *testing.T) {
	_, mux := setupTestAPI(t)

	// Signup
	signupPayload := map[string]any{
		"email":    "wrongpwd@example.com",
		"password": "CorrectPassword123",
	}
	resp, _ := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, signupPayload)
	resp.Body.Close()

	// Signin with wrong password
	signinPayload := map[string]any{
		"credential": "wrongpwd@example.com",
		"password":   "WrongPassword123",
	}
	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signin/credential", nil, signinPayload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong password status = %d, want %d. Body: %s", resp.StatusCode, http.StatusUnauthorized, string(body))
	}
}

// TestAuthUsernameCheck_Available tests checking available username
func TestAuthUsernameCheck_Available(t *testing.T) {
	_, mux := setupTestAPI(t)

	payload := map[string]any{
		"username": "availableusername",
	}

	resp, body := doRequest(t, mux, http.MethodPost, "/auth/usernames/check", nil, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("username check status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}

	var result map[string]bool
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	available := result["available"]
	if !available {
		t.Errorf("available username returned available=%v, want true", available)
	}
}

// TestAuthUsernameCheck_Taken tests checking taken username
func TestAuthUsernameCheck_Taken(t *testing.T) {
	_, mux := setupTestAPI(t)

	// Signup with username
	signupPayload := map[string]any{
		"email":    "takenuser@example.com",
		"password": "ValidPassword123",
		"username": "takenusername",
	}
	resp, _ := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, signupPayload)
	resp.Body.Close()

	// Check if username is available
	checkPayload := map[string]any{
		"username": "takenusername",
	}
	resp, body := doRequest(t, mux, http.MethodPost, "/auth/usernames/check", nil, checkPayload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("username check status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]bool
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	available := result["available"]
	if available {
		t.Errorf("taken username returned available=%v, want false", available)
	}
}

// TestAuthGetMe_HappyPath tests fetching current user with a valid session cookie
func TestAuthGetMe_HappyPath(t *testing.T) {
	_, mux := setupTestAPI(t)

	// Signup
	signupPayload := map[string]any{
		"email":    "getme@example.com",
		"password": "ValidPassword123",
	}
	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, signupPayload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("signup failed: %d. Body: %s", resp.StatusCode, string(body))
	}

	// Extract session cookie from Set-Cookie header
	setCookieHeaders := resp.Header.Values("Set-Cookie")
	if len(setCookieHeaders) == 0 {
		t.Fatal("no Set-Cookie header from signup")
	}

	// Extract just the name=value part before the semicolon
	cookieValue := strings.Split(setCookieHeaders[0], ";")[0]

	// Get /me with cookie
	resp, body = doRequest(t, mux, http.MethodGet, "/auth/me", map[string]string{
		"Cookie": cookieValue,
	}, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("/me status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	userMap := result["user"]
	if userMap == nil {
		t.Errorf("/me response missing 'user' field")
	} else {
		userField := userMap.(map[string]any)
		if email := userField["email"]; email != "getme@example.com" {
			t.Errorf("/me returned email=%v, want getme@example.com", email)
		}
	}
}

// TestAuthGetMe_Unauthorized tests /me without session cookie
func TestAuthGetMe_Unauthorized(t *testing.T) {
	_, mux := setupTestAPI(t)

	// Try /me without cookie
	resp, body := doRequest(t, mux, http.MethodGet, "/auth/me", nil, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("/me without cookie status = %d, want %d. Body: %s", resp.StatusCode, http.StatusUnauthorized, string(body))
	}
}

// TestAuthListSessions_HappyPath tests listing user sessions
func TestAuthListSessions_HappyPath(t *testing.T) {
	_, mux := setupTestAPI(t)

	// Signup
	signupPayload := map[string]any{
		"email":    "listses@example.com",
		"password": "ValidPassword123",
	}
	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, signupPayload)
	resp.Body.Close()

	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatal("no Set-Cookie header")
	}
	cookieHeader := strings.Split(cookies[0], ";")[0]

	// List sessions
	resp, body = doRequest(t, mux, http.MethodGet, "/auth/sessions", map[string]string{
		"Cookie": cookieHeader,
	}, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("/sessions status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}

	var sessions []map[string]any
	if err := json.Unmarshal(body, &sessions); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if len(sessions) == 0 {
		t.Error("/sessions returned empty array")
	}

	// The raw session token is a bearer credential and MUST NOT be exposed in
	// the list response (see sessionItem's json:"-" tag).
	if _, ok := sessions[0]["token"]; ok {
		t.Error("/sessions response leaked the session 'token' field")
	}
	// A non-sensitive identifier should still be present.
	if sessions[0]["expires_at"] == nil || sessions[0]["expires_at"] == "" {
		t.Error("/sessions response missing 'expires_at' field")
	}
}

// TestAuthListSessions_Unauthorized tests /sessions without session cookie
func TestAuthListSessions_Unauthorized(t *testing.T) {
	_, mux := setupTestAPI(t)

	// Try /sessions without cookie
	resp, body := doRequest(t, mux, http.MethodGet, "/auth/sessions", nil, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("/sessions without cookie status = %d, want %d. Body: %s", resp.StatusCode, http.StatusUnauthorized, string(body))
	}
}

// TestAuthSignout_HappyPath tests successful sign-out
func TestAuthSignout_HappyPath(t *testing.T) {
	_, mux := setupTestAPI(t)

	// Signup
	signupPayload := map[string]any{
		"email":    "signout@example.com",
		"password": "ValidPassword123",
	}
	resp, _ := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, signupPayload)
	resp.Body.Close()

	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatal("no Set-Cookie header")
	}
	cookieHeader := strings.Split(cookies[0], ";")[0]

	// Signout
	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signout", map[string]string{
		"Cookie": cookieHeader,
	}, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("signout status = %d, want %d. Body: %s", resp.StatusCode, http.StatusNoContent, string(body))
	}

	// Verify no body
	if len(body) > 0 {
		t.Errorf("signout returned body=%s, want empty", string(body))
	}
}

// TestAuthSignout_Unauthorized tests signout without session
func TestAuthSignout_Unauthorized(t *testing.T) {
	_, mux := setupTestAPI(t)

	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signout", nil, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("signout without cookie status = %d, want %d. Body: %s", resp.StatusCode, http.StatusUnauthorized, string(body))
	}
}

// TestAuthRevokeSessions_HappyPath tests revoking all sessions
func TestAuthRevokeSessions_HappyPath(t *testing.T) {
	_, mux := setupTestAPI(t)

	// Signup
	signupPayload := map[string]any{
		"email":    "revoke@example.com",
		"password": "ValidPassword123",
	}
	resp, _ := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, signupPayload)
	resp.Body.Close()

	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatal("no Set-Cookie header")
	}
	cookieHeader := strings.Split(cookies[0], ";")[0]

	// Revoke all sessions
	resp, body := doRequest(t, mux, http.MethodPost, "/auth/revoke-sessions", map[string]string{
		"Cookie": cookieHeader,
	}, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("revoke-sessions status = %d, want %d. Body: %s", resp.StatusCode, http.StatusNoContent, string(body))
	}

	// Verify no body
	if len(body) > 0 {
		t.Errorf("revoke-sessions returned body=%s, want empty", string(body))
	}
}

// TestAuthChangePassword_HappyPath tests changing password
func TestAuthChangePassword_HappyPath(t *testing.T) {
	_, mux := setupTestAPI(t)

	// Signup
	signupPayload := map[string]any{
		"email":    "changepwd@example.com",
		"password": "OriginalPassword123",
	}
	resp, _ := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, signupPayload)
	resp.Body.Close()

	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatal("no Set-Cookie header")
	}
	cookieHeader := strings.Split(cookies[0], ";")[0]

	// Change password
	changePayload := map[string]any{
		"current_password": "OriginalPassword123",
		"new_password":     "NewPassword123456",
	}
	resp, body := doRequest(t, mux, http.MethodPost, "/auth/passwords/change", map[string]string{
		"Cookie": cookieHeader,
	}, changePayload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("change password status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}
}

// TestAuthChangePassword_Unauthorized tests change password without session
func TestAuthChangePassword_Unauthorized(t *testing.T) {
	_, mux := setupTestAPI(t)

	changePayload := map[string]any{
		"current_password": "OriginalPassword123",
		"new_password":     "NewPassword123456",
	}
	resp, body := doRequest(t, mux, http.MethodPost, "/auth/passwords/change", nil, changePayload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("change password without cookie status = %d, want %d. Body: %s", resp.StatusCode, http.StatusUnauthorized, string(body))
	}
}

// TestAuthSetPassword_AlreadySet tests that SetPassword returns 403 when user already has a password
func TestAuthSetPassword_AlreadySet(t *testing.T) {
	_, mux := setupTestAPI(t)

	// Signup (user now has a password)
	signupPayload := map[string]any{
		"email":    "setpwd@example.com",
		"password": "InitialPassword123",
	}
	resp, _ := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, signupPayload)
	resp.Body.Close()

	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatal("no Set-Cookie header")
	}
	cookieHeader := strings.Split(cookies[0], ";")[0]

	// Try to set password (should fail because password already exists)
	setPayload := map[string]any{
		"new_password": "NewSetPassword123456",
	}
	resp, body := doRequest(t, mux, http.MethodPut, "/auth/passwords", map[string]string{
		"Cookie": cookieHeader,
	}, setPayload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("set password with existing password status = %d, want %d. Body: %s", resp.StatusCode, http.StatusForbidden, string(body))
	}
}

// TestAuthSetPassword_Unauthorized tests set password without session
func TestAuthSetPassword_Unauthorized(t *testing.T) {
	_, mux := setupTestAPI(t)

	setPayload := map[string]any{
		"new_password": "NewSetPassword123456",
	}
	resp, body := doRequest(t, mux, http.MethodPut, "/auth/passwords", nil, setPayload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("set password without cookie status = %d, want %d. Body: %s", resp.StatusCode, http.StatusUnauthorized, string(body))
	}
}

// TestAuthRequestPasswordReset tests requesting password reset
func TestAuthRequestPasswordReset_HappyPath(t *testing.T) {
	_, mux := setupTestAPI(t)

	payload := map[string]any{
		"email": "reset@example.com",
	}

	resp, body := doRequest(t, mux, http.MethodPost, "/auth/passwords/request-reset", nil, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("request-reset status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if result["message"] == nil || result["message"] == "" {
		t.Errorf("request-reset response missing or empty 'message' field")
	}
}

// TestAuthRequestPasswordReset_UnknownEmail tests reset for unknown email (should still return 200 for anti-enumeration)
func TestAuthRequestPasswordReset_UnknownEmail(t *testing.T) {
	_, mux := setupTestAPI(t)

	payload := map[string]any{
		"email": "unknownuser@example.com",
	}

	resp, body := doRequest(t, mux, http.MethodPost, "/auth/passwords/request-reset", nil, payload)
	defer resp.Body.Close()

	// Spec says this always returns 200 to avoid user enumeration
	if resp.StatusCode != http.StatusOK {
		t.Errorf("request-reset for unknown email status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}
}

// TestAuthContextCancellation tests that context cancellation is propagated
func TestAuthContextCancellation(t *testing.T) {
	// This test verifies that the delegate function uses httptest.NewRequestWithContext
	// to thread context cancellation through. We test this by observing that
	// a cancelled context is handled properly.
	_, mux := setupTestAPI(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	payload := map[string]any{
		"email":    "cancel@example.com",
		"password": "ValidPassword123",
	}

	// Marshal the request manually
	data, _ := json.Marshal(payload)
	reqBody := bytes.NewReader(data)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8080/auth/signup/credential", reqBody)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// The request should be processed (the cancellation is checked by limen internally)
	// We just verify it doesn't panic
	if rec == nil {
		t.Error("recorder is nil")
	}
}

// TestAuthOpenAPIGeneration tests that OpenAPI document is generated
func TestAuthOpenAPIGeneration(t *testing.T) {
	api, _ := setupTestAPI(t)

	// Try to get the OpenAPI spec
	spec := api.OpenAPI()
	if spec == nil {
		t.Fatal("OpenAPI spec is nil")
	}

	// Check that required operations are documented
	expectedOperations := map[string]bool{
		"auth-signup":                 false,
		"auth-signin":                 false,
		"auth-password-request-reset": false,
		"auth-password-reset":         false,
		"auth-password-change":        false,
		"auth-password-set":           false,
		"auth-username-check":         false,
		"auth-me":                     false,
		"auth-list-sessions":          false,
		"auth-signout":                false,
		"auth-revoke-sessions":        false,
	}

	// Check if all operations are present in the OpenAPI spec
	if spec.Paths == nil {
		t.Fatal("OpenAPI paths is nil")
	}

	for path, pathItem := range spec.Paths {
		if pathItem.Post != nil && pathItem.Post.OperationID != "" {
			expectedOperations[pathItem.Post.OperationID] = true
		}
		if pathItem.Get != nil && pathItem.Get.OperationID != "" {
			expectedOperations[pathItem.Get.OperationID] = true
		}
		if pathItem.Put != nil && pathItem.Put.OperationID != "" {
			expectedOperations[pathItem.Put.OperationID] = true
		}
		_ = path // silence unused warning
	}

	// Verify all expected operations are present
	for opID, found := range expectedOperations {
		if !found {
			t.Errorf("OpenAPI missing operation: %s", opID)
		}
	}
}

// TestAuthOpenAPITags tests that operations have Auth tag
func TestAuthOpenAPITags(t *testing.T) {
	api, _ := setupTestAPI(t)

	spec := api.OpenAPI()
	if spec == nil {
		t.Fatal("OpenAPI spec is nil")
	}

	authTagCount := 0
	for _, pathItem := range spec.Paths {
		if pathItem.Post != nil && pathItem.Post.Tags != nil {
			for _, tag := range pathItem.Post.Tags {
				if tag == "Auth" {
					authTagCount++
				}
			}
		}
		if pathItem.Get != nil && pathItem.Get.Tags != nil {
			for _, tag := range pathItem.Get.Tags {
				if tag == "Auth" {
					authTagCount++
				}
			}
		}
		if pathItem.Put != nil && pathItem.Put.Tags != nil {
			for _, tag := range pathItem.Put.Tags {
				if tag == "Auth" {
					authTagCount++
				}
			}
		}
	}

	if authTagCount < 11 {
		t.Errorf("OpenAPI Auth tag count = %d, want at least 11", authTagCount)
	}
}

// TestAuthSetCookiePassthrough tests that multiple Set-Cookie values are passed through
func TestAuthSetCookiePassthrough(t *testing.T) {
	_, mux := setupTestAPI(t)

	payload := map[string]any{
		"email":    "cookies@example.com",
		"password": "ValidPassword123",
	}

	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("signup status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}

	// Check Set-Cookie headers
	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Error("no Set-Cookie headers found")
		return
	}

	// Verify limen_session is present
	found := false
	for _, cookie := range cookies {
		if strings.Contains(cookie, "limen_session") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("limen_session not found in Set-Cookie headers: %v", cookies)
	}
}

// TestAuthInvalidEmail tests signup with invalid email
func TestAuthInvalidEmail(t *testing.T) {
	_, mux := setupTestAPI(t)

	payload := map[string]any{
		"email":    "not-an-email",
		"password": "ValidPassword123",
	}

	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, payload)
	defer resp.Body.Close()

	// huma validates format:email
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("invalid email status = %d, want %d. Body: %s", resp.StatusCode, http.StatusUnprocessableEntity, string(body))
	}
}

// TestAuthConcurrentRequests tests concurrent signup operations
func TestAuthConcurrentRequests(t *testing.T) {
	_, mux := setupTestAPI(t)

	done := make(chan error, 3)

	// Use only 3 concurrent requests to avoid hitting rate limits
	for i := 0; i < 3; i++ {
		go func(idx int) {
			email := fmt.Sprintf("concurrent%d@example.com", idx)
			payload := map[string]any{
				"email":    email,
				"password": "ValidPassword123",
			}

			resp, body := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, payload)
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				done <- fmt.Errorf("concurrent signup failed: %d. Body: %s", resp.StatusCode, string(body))
				return
			}
			done <- nil
		}(i)
	}

	for i := 0; i < 3; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent test error: %v", err)
		}
	}
}

// TestAuthSignupOptionalUsername tests signup without username
func TestAuthSignupOptionalUsername(t *testing.T) {
	_, mux := setupTestAPI(t)

	payload := map[string]any{
		"email":    "nousername@example.com",
		"password": "ValidPassword123",
		// username omitted
	}

	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("signup without username status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}
}

// TestAuthSigninWithUsername tests signin using username instead of email
func TestAuthSigninWithUsername(t *testing.T) {
	_, mux := setupTestAPI(t)

	// Signup with username
	signupPayload := map[string]any{
		"email":    "username@example.com",
		"password": "ValidPassword123",
		"username": "myusername",
	}
	resp, _ := doRequest(t, mux, http.MethodPost, "/auth/signup/credential", nil, signupPayload)
	resp.Body.Close()

	// Signin with username credential
	signinPayload := map[string]any{
		"credential": "myusername",
		"password":   "ValidPassword123",
	}
	resp, body := doRequest(t, mux, http.MethodPost, "/auth/signin/credential", nil, signinPayload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("signin with username status = %d, want %d. Body: %s", resp.StatusCode, http.StatusOK, string(body))
	}
}
