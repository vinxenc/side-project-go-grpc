package server

import (
	"os"
	"time"
)

const (
	defaultPort            = "8080"
	defaultReadTimeout     = 5 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultIdleTimeout     = 120 * time.Second
	defaultShutdownTimeout = 10 * time.Second
)

// Config holds runtime configuration resolved from the environment.
type Config struct {
	Port            string        // TCP port to listen on, e.g. "8080" (no colon)
	ReadTimeout     time.Duration // header+body read timeout
	WriteTimeout    time.Duration // response write timeout
	IdleTimeout     time.Duration // keep-alive idle timeout
	ShutdownTimeout time.Duration // max time to wait for graceful shutdown
}

// LoadConfig builds a Config from environment variables, applying defaults.
// Reads PORT (default "8080"). Never returns an error for this minimal version;
// invalid PORT handling is documented in the spec Edge cases.
func LoadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	return Config{
		Port:            port,
		ReadTimeout:     defaultReadTimeout,
		WriteTimeout:    defaultWriteTimeout,
		IdleTimeout:     defaultIdleTimeout,
		ShutdownTimeout: defaultShutdownTimeout,
	}
}
