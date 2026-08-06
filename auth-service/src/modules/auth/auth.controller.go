package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/danielgtaylor/huma/v2"
)

// controller holds a reference to limen's http.Handler and exposes all auth
// endpoints as typed huma operations. The limen handler is invoked in-process
// (never mounted on the public mux) so limen retains full control of sessions,
// cookies, CSRF, rate-limiting, and validation while huma produces OpenAPI.
type controller struct {
	handler http.Handler
}

// RegisterRoutes registers all 11 auth operations onto the huma API.
// The explicit huma.Operation form is used (as in the health module) so every
// OperationID, Method, Path, and Tag is pinned and stable for generated clients.
func (c *controller) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "auth-signup",
		Summary:       "Sign up",
		Description:   "Create a new account with email, password, and optional username.",
		Method:        http.MethodPost,
		Path:          "/auth/signup/credential",
		Tags:          []string{"Auth"},
		DefaultStatus: http.StatusOK,
		Errors:        []int{400, 409, 422, 429},
	}, c.signup)

	huma.Register(api, huma.Operation{
		OperationID:   "auth-signin",
		Summary:       "Sign in",
		Description:   "Authenticate with email or username and password.",
		Method:        http.MethodPost,
		Path:          "/auth/signin/credential",
		Tags:          []string{"Auth"},
		DefaultStatus: http.StatusOK,
		Errors:        []int{400, 401, 422, 429},
	}, c.signin)

	huma.Register(api, huma.Operation{
		OperationID:   "auth-password-request-reset",
		Summary:       "Request password reset",
		Description:   "Send a password-reset token to the given email address. Always returns 200 to avoid user enumeration.",
		Method:        http.MethodPost,
		Path:          "/auth/passwords/request-reset",
		Tags:          []string{"Auth"},
		DefaultStatus: http.StatusOK,
		Errors:        []int{400, 422, 429},
	}, c.requestReset)

	huma.Register(api, huma.Operation{
		OperationID:   "auth-password-reset",
		Summary:       "Reset password",
		Description:   "Set a new password using a valid password-reset token.",
		Method:        http.MethodPost,
		Path:          "/auth/passwords/reset",
		Tags:          []string{"Auth"},
		DefaultStatus: http.StatusOK,
		Errors:        []int{400, 422, 429},
	}, c.resetPassword)

	huma.Register(api, huma.Operation{
		OperationID:   "auth-password-change",
		Summary:       "Change password",
		Description:   "Change the current password for an authenticated user.",
		Method:        http.MethodPost,
		Path:          "/auth/passwords/change",
		Tags:          []string{"Auth"},
		DefaultStatus: http.StatusOK,
		Errors:        []int{400, 401, 422, 429},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, c.changePassword)

	huma.Register(api, huma.Operation{
		OperationID:   "auth-password-set",
		Summary:       "Set password",
		Description:   "Set a password for a user who does not have one (e.g. signed up via OAuth).",
		Method:        http.MethodPut,
		Path:          "/auth/passwords",
		Tags:          []string{"Auth"},
		DefaultStatus: http.StatusOK,
		Errors:        []int{400, 401, 403, 422, 429},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, c.setPassword)

	huma.Register(api, huma.Operation{
		OperationID:   "auth-username-check",
		Summary:       "Check username availability",
		Description:   "Check whether a username is already taken.",
		Method:        http.MethodPost,
		Path:          "/auth/usernames/check",
		Tags:          []string{"Auth"},
		DefaultStatus: http.StatusOK,
		Errors:        []int{400, 422, 429},
	}, c.usernameCheck)

	huma.Register(api, huma.Operation{
		OperationID:   "auth-me",
		Summary:       "Get current user",
		Description:   "Return the authenticated user's profile.",
		Method:        http.MethodGet,
		Path:          "/auth/me",
		Tags:          []string{"Auth"},
		DefaultStatus: http.StatusOK,
		Errors:        []int{401, 429},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, c.me)

	huma.Register(api, huma.Operation{
		OperationID:   "auth-list-sessions",
		Summary:       "List sessions",
		Description:   "List all active sessions for the authenticated user.",
		Method:        http.MethodGet,
		Path:          "/auth/sessions",
		Tags:          []string{"Auth"},
		DefaultStatus: http.StatusOK,
		Errors:        []int{401, 429},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, c.listSessions)

	huma.Register(api, huma.Operation{
		OperationID:   "auth-signout",
		Summary:       "Sign out",
		Description:   "Revoke the current session and clear the session cookie.",
		Method:        http.MethodPost,
		Path:          "/auth/signout",
		Tags:          []string{"Auth"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 429},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, c.signout)

	huma.Register(api, huma.Operation{
		OperationID:   "auth-revoke-sessions",
		Summary:       "Revoke all sessions",
		Description:   "Revoke all active sessions for the authenticated user.",
		Method:        http.MethodPost,
		Path:          "/auth/revoke-sessions",
		Tags:          []string{"Auth"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{401, 429},
		Security: []map[string][]string{
			{"cookieAuth": {}},
		},
	}, c.revokeSessions)
}

// ---------------------------------------------------------------------------
// delegate — in-process proxy to limen's handler
// ---------------------------------------------------------------------------

// delegate marshals body to JSON (or uses an empty reader for nil), builds a
// synthetic *http.Request aimed at limenPath, forwards the cookie and
// authorization header values, invokes limen's handler, and returns the
// recorder. The caller is responsible for translating the recorder into the
// typed huma output.
//
// cookie and authorization are plain strings taken directly from the input
// struct's own header fields. They must NOT be sourced from an anonymously
// embedded struct (huma v2.39.0 skips anonymous fields during param binding).
func (c *controller) delegate(ctx context.Context, method, limenPath string, body any, cookie, authorization string) (*httptest.ResponseRecorder, error) {
	var reader io.Reader
	if body != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, err
		}
		reader = &buf
	} else {
		reader = http.NoBody
	}

	req, err := http.NewRequestWithContext(ctx, method, limenPath, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	return rec, nil
}

// ---------------------------------------------------------------------------
// errorFromRecorder extracts the error message from a non-2xx limen response
// and wraps it in a huma.StatusError. Callers use this when rec.Code >= 400.
//
// For 5xx responses the upstream message is NOT forwarded to the client (it may
// contain internal detail); it is logged for operators and the client receives
// the generic status text instead. Known 4xx messages are passed through so
// callers see actionable validation/auth errors.
// ---------------------------------------------------------------------------

func errorFromRecorder(rec *httptest.ResponseRecorder) error {
	var payload struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&payload)

	msg := payload.Message
	if rec.Code >= 500 {
		if payload.Message != "" {
			log.Printf("auth: upstream %d from limen: %s", rec.Code, payload.Message)
		}
		msg = "" // fall through to status text; do not leak internals
	}
	if msg == "" {
		msg = http.StatusText(rec.Code)
	}
	return huma.NewError(rec.Code, msg)
}

// ---------------------------------------------------------------------------
// Per-operation handlers
// ---------------------------------------------------------------------------

func (c *controller) signup(ctx context.Context, in *signupInput) (*sessionOutput, error) {
	rec, err := c.delegate(ctx, http.MethodPost, "/auth/signup/credential", in.Body, "", "")
	if err != nil {
		return nil, err
	}
	if rec.Code >= 400 {
		return nil, errorFromRecorder(rec)
	}

	var out sessionOutput
	if err := json.NewDecoder(rec.Body).Decode(&out.Body); err != nil {
		return nil, err
	}
	out.SetCookie = rec.Result().Header["Set-Cookie"]
	return &out, nil
}

func (c *controller) signin(ctx context.Context, in *signinInput) (*sessionOutput, error) {
	rec, err := c.delegate(ctx, http.MethodPost, "/auth/signin/credential", in.Body, "", "")
	if err != nil {
		return nil, err
	}
	if rec.Code >= 400 {
		return nil, errorFromRecorder(rec)
	}

	var out sessionOutput
	if err := json.NewDecoder(rec.Body).Decode(&out.Body); err != nil {
		return nil, err
	}
	out.SetCookie = rec.Result().Header["Set-Cookie"]
	return &out, nil
}

func (c *controller) requestReset(ctx context.Context, in *requestResetInput) (*messageOutput, error) {
	rec, err := c.delegate(ctx, http.MethodPost, "/auth/passwords/request-reset", in.Body, "", "")
	if err != nil {
		return nil, err
	}
	if rec.Code >= 400 {
		return nil, errorFromRecorder(rec)
	}

	var out messageOutput
	if err := json.NewDecoder(rec.Body).Decode(&out.Body); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *controller) resetPassword(ctx context.Context, in *resetPasswordInput) (*messageOutput, error) {
	rec, err := c.delegate(ctx, http.MethodPost, "/auth/passwords/reset", in.Body, "", "")
	if err != nil {
		return nil, err
	}
	if rec.Code >= 400 {
		return nil, errorFromRecorder(rec)
	}

	var out messageOutput
	if err := json.NewDecoder(rec.Body).Decode(&out.Body); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *controller) changePassword(ctx context.Context, in *changePasswordInput) (*sessionOutput, error) {
	rec, err := c.delegate(ctx, http.MethodPost, "/auth/passwords/change", in.Body, in.Cookie, in.Authorization)
	if err != nil {
		return nil, err
	}
	if rec.Code >= 400 {
		return nil, errorFromRecorder(rec)
	}

	var out sessionOutput
	if err := json.NewDecoder(rec.Body).Decode(&out.Body); err != nil {
		return nil, err
	}
	out.SetCookie = rec.Result().Header["Set-Cookie"]
	return &out, nil
}

func (c *controller) setPassword(ctx context.Context, in *setPasswordInput) (*sessionOutput, error) {
	rec, err := c.delegate(ctx, http.MethodPut, "/auth/passwords", in.Body, in.Cookie, in.Authorization)
	if err != nil {
		return nil, err
	}
	if rec.Code >= 400 {
		return nil, errorFromRecorder(rec)
	}

	var out sessionOutput
	if err := json.NewDecoder(rec.Body).Decode(&out.Body); err != nil {
		return nil, err
	}
	out.SetCookie = rec.Result().Header["Set-Cookie"]
	return &out, nil
}

func (c *controller) usernameCheck(ctx context.Context, in *usernameCheckInput) (*usernameCheckOutput, error) {
	rec, err := c.delegate(ctx, http.MethodPost, "/auth/usernames/check", in.Body, "", "")
	if err != nil {
		return nil, err
	}
	if rec.Code >= 400 {
		return nil, errorFromRecorder(rec)
	}

	var out usernameCheckOutput
	if err := json.NewDecoder(rec.Body).Decode(&out.Body); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *controller) me(ctx context.Context, in *meInput) (*sessionOutput, error) {
	rec, err := c.delegate(ctx, http.MethodGet, "/auth/me", nil, in.Cookie, in.Authorization)
	if err != nil {
		return nil, err
	}
	if rec.Code >= 400 {
		return nil, errorFromRecorder(rec)
	}

	var out sessionOutput
	if err := json.NewDecoder(rec.Body).Decode(&out.Body); err != nil {
		return nil, err
	}
	out.SetCookie = rec.Result().Header["Set-Cookie"]
	return &out, nil
}

func (c *controller) listSessions(ctx context.Context, in *sessionsInput) (*sessionsOutput, error) {
	rec, err := c.delegate(ctx, http.MethodGet, "/auth/sessions", nil, in.Cookie, in.Authorization)
	if err != nil {
		return nil, err
	}
	if rec.Code >= 400 {
		return nil, errorFromRecorder(rec)
	}

	var out sessionsOutput
	if err := json.NewDecoder(rec.Body).Decode(&out.Body); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *controller) signout(ctx context.Context, in *signoutInput) (*emptyOutput, error) {
	rec, err := c.delegate(ctx, http.MethodPost, "/auth/signout", nil, in.Cookie, in.Authorization)
	if err != nil {
		return nil, err
	}
	if rec.Code >= 400 {
		return nil, errorFromRecorder(rec)
	}

	// 204: no body to decode.
	return &emptyOutput{SetCookie: rec.Result().Header["Set-Cookie"]}, nil
}

func (c *controller) revokeSessions(ctx context.Context, in *revokeInput) (*emptyOutput, error) {
	rec, err := c.delegate(ctx, http.MethodPost, "/auth/revoke-sessions", nil, in.Cookie, in.Authorization)
	if err != nil {
		return nil, err
	}
	if rec.Code >= 400 {
		return nil, errorFromRecorder(rec)
	}

	// 204: no body to decode.
	return &emptyOutput{SetCookie: rec.Result().Header["Set-Cookie"]}, nil
}
