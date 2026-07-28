package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// New builds a configured *http.Server bound to cfg.Port using the given handler.
func New(cfg Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
}

// Run starts srv and blocks until ctx is cancelled (e.g. SIGINT/SIGTERM),
// then performs a graceful shutdown bounded by shutdownTimeout.
// Returns nil on a clean shutdown; a non-nil error if ListenAndServe fails
// for a reason other than http.ErrServerClosed, or if graceful shutdown times out.
func Run(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error {
	serveErr := make(chan error, 1)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		// Server failed to start or exited unexpectedly.
		if err != nil {
			return fmt.Errorf("server listen: %w", err)
		}
		return nil
	case <-ctx.Done():
		// Signal received — initiate graceful shutdown.
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	// Wait for ListenAndServe to return after Shutdown.
	if err := <-serveErr; err != nil {
		return fmt.Errorf("server listen: %w", err)
	}

	return nil
}
