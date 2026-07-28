package server

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name             string
		portEnv          *string // nil means unset, otherwise set to value
		wantPort         string
		wantReadTimeout  time.Duration
		wantWriteTimeout time.Duration
		wantIdleTimeout  time.Duration
		wantShutdownTime time.Duration
	}{
		{
			name:             "PORT unset uses default 8080",
			portEnv:          nil,
			wantPort:         "8080",
			wantReadTimeout:  5 * time.Second,
			wantWriteTimeout: 10 * time.Second,
			wantIdleTimeout:  120 * time.Second,
			wantShutdownTime: 10 * time.Second,
		},
		{
			name:             "PORT empty string uses default 8080",
			portEnv:          strPtr(""),
			wantPort:         "8080",
			wantReadTimeout:  5 * time.Second,
			wantWriteTimeout: 10 * time.Second,
			wantIdleTimeout:  120 * time.Second,
			wantShutdownTime: 10 * time.Second,
		},
		{
			name:             "PORT set to 9090",
			portEnv:          strPtr("9090"),
			wantPort:         "9090",
			wantReadTimeout:  5 * time.Second,
			wantWriteTimeout: 10 * time.Second,
			wantIdleTimeout:  120 * time.Second,
			wantShutdownTime: 10 * time.Second,
		},
		{
			name:             "PORT set to 3000",
			portEnv:          strPtr("3000"),
			wantPort:         "3000",
			wantReadTimeout:  5 * time.Second,
			wantWriteTimeout: 10 * time.Second,
			wantIdleTimeout:  120 * time.Second,
			wantShutdownTime: 10 * time.Second,
		},
		{
			name:             "PORT with invalid value passes through",
			portEnv:          strPtr("abc"),
			wantPort:         "abc",
			wantReadTimeout:  5 * time.Second,
			wantWriteTimeout: 10 * time.Second,
			wantIdleTimeout:  120 * time.Second,
			wantShutdownTime: 10 * time.Second,
		},
		{
			name:             "PORT with special characters passes through",
			portEnv:          strPtr("not-a-port"),
			wantPort:         "not-a-port",
			wantReadTimeout:  5 * time.Second,
			wantWriteTimeout: 10 * time.Second,
			wantIdleTimeout:  120 * time.Second,
			wantShutdownTime: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original PORT value
			origPort, portExists := os.LookupEnv("PORT")
			defer func() {
				if portExists {
					os.Setenv("PORT", origPort)
				} else {
					os.Unsetenv("PORT")
				}
			}()

			// Set PORT as per test case
			if tt.portEnv == nil {
				os.Unsetenv("PORT")
			} else {
				os.Setenv("PORT", *tt.portEnv)
			}

			cfg := LoadConfig()

			if cfg.Port != tt.wantPort {
				t.Errorf("got Port %q, want %q", cfg.Port, tt.wantPort)
			}

			if cfg.ReadTimeout != tt.wantReadTimeout {
				t.Errorf("got ReadTimeout %v, want %v", cfg.ReadTimeout, tt.wantReadTimeout)
			}

			if cfg.WriteTimeout != tt.wantWriteTimeout {
				t.Errorf("got WriteTimeout %v, want %v", cfg.WriteTimeout, tt.wantWriteTimeout)
			}

			if cfg.IdleTimeout != tt.wantIdleTimeout {
				t.Errorf("got IdleTimeout %v, want %v", cfg.IdleTimeout, tt.wantIdleTimeout)
			}

			if cfg.ShutdownTimeout != tt.wantShutdownTime {
				t.Errorf("got ShutdownTimeout %v, want %v", cfg.ShutdownTimeout, tt.wantShutdownTime)
			}
		})
	}
}

// Helper function to create string pointers for test cases
func strPtr(s string) *string {
	return &s
}
