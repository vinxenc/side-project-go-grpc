package setting

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoad_ReadsDotEnvFile verifies the best-effort .env load path: when a .env
// file is present in the working directory, Load reads it (covering the
// "loaded .env file" branch) and its values populate the Setting.
func TestLoad_ReadsDotEnvFile(t *testing.T) {
	dir := t.TempDir()
	env := "DATABASE_URL=postgres://u:p@localhost:5432/db?sslmode=disable\n" +
		"LIMEN_SECRET=0123456789abcdef0123456789abcdef\n" +
		"AUTH_BASE_URL=https://from-dotenv.example.com\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	// godotenv.Load reads ".env" from the current working directory.
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	s, err := Load()
	if err != nil {
		t.Fatalf("Load() with .env present returned error: %v", err)
	}
	if s.BaseURL != "https://from-dotenv.example.com" {
		t.Errorf("BaseURL = %q, want value from .env", s.BaseURL)
	}
}

// TestLoad_HappyPath verifies that Load returns a populated Env with no error
// when DATABASE_URL and a valid 32-byte LIMEN_SECRET are set, and that BaseURL
// defaults to http://localhost:8080 when AUTH_BASE_URL is unset.
func TestLoad_HappyPath(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("LIMEN_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("AUTH_BASE_URL", "")

	env, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if env.DatabaseURL != "postgres://user:pass@localhost:5432/db?sslmode=disable" {
		t.Errorf("DatabaseURL = %q, want postgres://...", env.DatabaseURL)
	}
	if len(env.Secret) != 32 {
		t.Errorf("Secret len = %d, want 32", len(env.Secret))
	}
	if env.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q, want http://localhost:8080 (default)", env.BaseURL)
	}
}

// TestLoad_CustomBaseURL verifies that AUTH_BASE_URL overrides the default.
func TestLoad_CustomBaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("LIMEN_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("AUTH_BASE_URL", "https://auth.example.com")

	env, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if env.BaseURL != "https://auth.example.com" {
		t.Errorf("BaseURL = %q, want https://auth.example.com", env.BaseURL)
	}
}

// TestLoad_EmptyDatabaseURL verifies that Load returns an error when DATABASE_URL
// is unset/empty (no in-memory fallback).
func TestLoad_EmptyDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("LIMEN_SECRET", "0123456789abcdef0123456789abcdef")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() with empty DATABASE_URL expected error, got nil")
	}
}

// TestLoad_SecretTooShort verifies that a LIMEN_SECRET shorter than 32 bytes is rejected.
func TestLoad_SecretTooShort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("LIMEN_SECRET", "tooshortvalue123") // 16 bytes

	_, err := Load()
	if err == nil {
		t.Fatal("Load() with 16-byte LIMEN_SECRET expected error, got nil")
	}
}

// TestLoad_SecretTooLong verifies that a LIMEN_SECRET longer than 32 bytes is rejected.
func TestLoad_SecretTooLong(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("LIMEN_SECRET", "thisistoolongbutshouldfailbecauseitis48byteslong!!") // 50 bytes

	_, err := Load()
	if err == nil {
		t.Fatal("Load() with long LIMEN_SECRET expected error, got nil")
	}
}

// TestLoad_UnsetSecret_FailClosed verifies that Load returns an error when
// LIMEN_SECRET is unset and AUTH_ALLOW_DEV_SECRET is not "true" (CWE-798 guard).
func TestLoad_UnsetSecret_FailClosed(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("LIMEN_SECRET", "")
	t.Setenv("AUTH_ALLOW_DEV_SECRET", "") // not opted in

	_, err := Load()
	if err == nil {
		t.Fatal("Load() with unset LIMEN_SECRET and no dev opt-in expected error, got nil")
	}
}

// TestLoad_InvalidAllowDevSecret verifies that a non-boolean AUTH_ALLOW_DEV_SECRET
// fails env parsing (covering the parse-error branch of Load).
func TestLoad_InvalidAllowDevSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("LIMEN_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("AUTH_ALLOW_DEV_SECRET", "not-a-bool")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() with non-boolean AUTH_ALLOW_DEV_SECRET expected error, got nil")
	}
}

// TestLoad_UnsetSecret_DevOptIn verifies that Load succeeds (returning the 32-byte
// dev secret) when LIMEN_SECRET is unset but AUTH_ALLOW_DEV_SECRET=true.
func TestLoad_UnsetSecret_DevOptIn(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("LIMEN_SECRET", "")
	t.Setenv("AUTH_ALLOW_DEV_SECRET", "true")

	env, err := Load()
	if err != nil {
		t.Fatalf("Load() with AUTH_ALLOW_DEV_SECRET=true expected no error, got %v", err)
	}
	if len(env.Secret) != 32 {
		t.Errorf("Secret len = %d, want 32", len(env.Secret))
	}
}
