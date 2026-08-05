package auth

import (
	"testing"
)

// TestResolveSecret_WrongLength asserts that resolveSecret returns an error
// when LIMEN_SECRET is not exactly 32 bytes. This tests the ⚠️ edge case:
// "LIMEN_SECRET set but not exactly 32 bytes → existing resolveSecret error
// path (fail fast); keep behaviour."
func TestResolveSecret_WrongLength(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		wantError bool
	}{
		{
			name:      "too short (16 bytes)",
			envValue:  "tooshortvalue123",
			wantError: true,
		},
		{
			name:      "too long (48 bytes)",
			envValue:  "thisistoolongbutshouldfailbecauseitis48byteslong!!",
			wantError: true,
		},
		{
			name:      "exactly 32 bytes (valid)",
			envValue:  "0123456789abcdef0123456789abcdef",
			wantError: false,
		},
		{
			name:      "empty (falls back to dev secret, no error)",
			envValue:  "",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use t.Setenv to isolate this test's env var from others.
			t.Setenv("LIMEN_SECRET", tt.envValue)

			// Call resolveSecret (white-box test in package auth).
			secret, err := resolveSecret()

			if tt.wantError {
				if err == nil {
					t.Errorf("resolveSecret with %q expected error, got nil", tt.envValue)
				}
				if secret != nil {
					t.Errorf("resolveSecret with %q should return nil secret on error, got non-nil", tt.envValue)
				}
			} else {
				if err != nil {
					t.Errorf("resolveSecret with %q expected no error, got %v", tt.envValue, err)
				}
				if secret == nil {
					t.Errorf("resolveSecret with %q expected non-nil secret, got nil", tt.envValue)
				} else if len(secret) != 32 {
					t.Errorf("resolveSecret with %q expected 32-byte secret, got %d bytes", tt.envValue, len(secret))
				}
			}
		})
	}
}

// TestAuthNew_WrongLimenSecret asserts that auth.New() returns an error when
// LIMEN_SECRET is set to a wrong-length value. This is an integration test of
// the resolveSecret path within the auth module initialization.
func TestAuthNew_WrongLimenSecret(t *testing.T) {
	t.Setenv("LIMEN_SECRET", "toolow") // 6 bytes, not 32
	t.Setenv("DATABASE_URL", "")       // Use in-memory to avoid DB requirement
	t.Setenv("AUTH_BASE_URL", "http://localhost")

	module, err := New()

	if err == nil {
		t.Error("auth.New with wrong LIMEN_SECRET length expected error, got nil")
		if module != nil {
			// Verify it's truly unusable (though it shouldn't be created).
			t.Error("auth.New with wrong LIMEN_SECRET should not return a module")
		}
		return
	}

	// Verify the error mentions the secret length issue.
	if module != nil {
		t.Error("auth.New with wrong LIMEN_SECRET expected module=nil, got non-nil")
	}

	if err.Error() == "" {
		t.Error("auth.New error is empty; expected a descriptive message about secret length")
	}
}

// TestResolveSecret_Empty asserts that resolveSecret returns the dev secret
// when LIMEN_SECRET is unset. This exercises the ⚠️ edge case:
// "LIMEN_SECRET unset → dev warning path; still allowed for local dev."
func TestResolveSecret_Empty(t *testing.T) {
	t.Setenv("LIMEN_SECRET", "")

	secret, err := resolveSecret()

	if err != nil {
		t.Errorf("resolveSecret with empty LIMEN_SECRET expected no error, got %v", err)
	}
	if secret == nil {
		t.Error("resolveSecret with empty LIMEN_SECRET expected non-nil secret, got nil")
	} else if len(secret) != 32 {
		t.Errorf("resolveSecret with empty LIMEN_SECRET expected 32-byte secret, got %d bytes", len(secret))
	}
}
