package auth

// ---------------------------------------------------------------------------
// Shared response fragments
// ---------------------------------------------------------------------------

// userResponse mirrors limen's serialized user (id and password are stripped
// by limen before the response is written).
type userResponse struct {
	Email           string  `json:"email"`
	EmailVerifiedAt *string `json:"email_verified_at,omitempty"`
	Username        *string `json:"username,omitempty"`
	FirstName       *string `json:"first_name,omitempty"`
	LastName        *string `json:"last_name,omitempty"`
	CreatedAt       *string `json:"created_at,omitempty"`
	UpdatedAt       *string `json:"updated_at,omitempty"`
}

type userEnvelope struct {
	User userResponse `json:"user"`
}

type messageEnvelope struct {
	Message string `json:"message"`
}

// sessionItem mirrors the limen.Session JSON (session.go). ID and UserID are
// typed as any because the in-memory adapter uses int64 IDs whereas a real DB
// may use UUID strings; OpenAPI types them loosely.
type sessionItem struct {
	ID         any            `json:"id,omitempty"`
	Token      string         `json:"token"`
	UserID     any            `json:"user_id"`
	CreatedAt  string         `json:"created_at"`
	ExpiresAt  string         `json:"expires_at"`
	LastAccess string         `json:"last_access"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// ---------------------------------------------------------------------------
// Request bodies (input structs wrap body in Body, per huma convention)
//
// IMPORTANT: huma v2.39.0 does NOT bind params from anonymously embedded
// structs (huma.go:550 skips anonymous fields during param resolution). Header
// fields that must reach the handler MUST be declared as direct (non-anonymous)
// fields on each input struct. A shared "authHeaders" embed was the original
// design but it silently discards all cookie/authorization values. Each
// session-protected struct therefore declares Cookie and Authorization inline.
// ---------------------------------------------------------------------------

// Public routes: no inbound session cookie needed.

type signupInput struct {
	Body struct {
		Email    string  `json:"email"              format:"email" required:"true"`
		Password string  `json:"password"           minLength:"8"  required:"true"`
		Username *string `json:"username,omitempty"`
	}
}

type signinInput struct {
	Body struct {
		Credential string `json:"credential" required:"true" doc:"email or username"`
		Password   string `json:"password"   required:"true"`
		RememberMe *bool  `json:"remember_me,omitempty"`
	}
}

type requestResetInput struct {
	Body struct {
		Email string `json:"email" format:"email" required:"true"`
	}
}

type resetPasswordInput struct {
	Body struct {
		Token       string `json:"token"        required:"true"`
		NewPassword string `json:"new_password" minLength:"8" required:"true"`
	}
}

type usernameCheckInput struct {
	Body struct {
		Username string `json:"username" required:"true"`
	}
}

// Session-protected routes: Cookie and Authorization are direct fields so huma
// binds them correctly (see note above).

type changePasswordInput struct {
	Cookie        string `header:"Cookie"        doc:"Session cookie (limen_session=...)" required:"false"`
	Authorization string `header:"Authorization" doc:"Bearer token if enabled"           required:"false"`
	Body          struct {
		CurrentPassword     string `json:"current_password"                 required:"true"`
		NewPassword         string `json:"new_password"       minLength:"8" required:"true"`
		RevokeOtherSessions *bool  `json:"revoke_other_sessions,omitempty"`
	}
}

type setPasswordInput struct {
	Cookie        string `header:"Cookie"        doc:"Session cookie (limen_session=...)" required:"false"`
	Authorization string `header:"Authorization" doc:"Bearer token if enabled"           required:"false"`
	Body          struct {
		NewPassword         string `json:"new_password"       minLength:"8" required:"true"`
		RevokeOtherSessions *bool  `json:"revoke_other_sessions,omitempty"`
	}
}

type meInput struct {
	Cookie        string `header:"Cookie"        doc:"Session cookie (limen_session=...)" required:"false"`
	Authorization string `header:"Authorization" doc:"Bearer token if enabled"           required:"false"`
}

type sessionsInput struct {
	Cookie        string `header:"Cookie"        doc:"Session cookie (limen_session=...)" required:"false"`
	Authorization string `header:"Authorization" doc:"Bearer token if enabled"           required:"false"`
}

type signoutInput struct {
	Cookie        string `header:"Cookie"        doc:"Session cookie (limen_session=...)" required:"false"`
	Authorization string `header:"Authorization" doc:"Bearer token if enabled"           required:"false"`
}

type revokeInput struct {
	Cookie        string `header:"Cookie"        doc:"Session cookie (limen_session=...)" required:"false"`
	Authorization string `header:"Authorization" doc:"Bearer token if enabled"           required:"false"`
}

// ---------------------------------------------------------------------------
// Response outputs (SetCookie passes limen cookies through huma header fields)
// ---------------------------------------------------------------------------

// sessionOutput is used for operations that return a user payload and may set
// a session cookie: signup, signin, password-change, password-set, me.
type sessionOutput struct {
	SetCookie []string `header:"Set-Cookie"`
	Body      userEnvelope
}

// messageOutput is used for operations that return a plain message: password
// request-reset, password-reset.
type messageOutput struct {
	Body messageEnvelope
}

// usernameCheckOutput is used for the username availability check endpoint.
type usernameCheckOutput struct {
	Body struct {
		Available bool `json:"available"`
	}
}

// sessionsOutput is used for the list-sessions endpoint.
type sessionsOutput struct {
	Body []sessionItem
}

// emptyOutput is used for 204 endpoints (signout, revoke-sessions). It has no
// Body field; attempting to decode the empty response body would cause an
// error, so callers must skip body decoding for this type.
type emptyOutput struct {
	SetCookie []string `header:"Set-Cookie"`
}
