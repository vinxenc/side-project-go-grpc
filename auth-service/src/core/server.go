package core

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"time"
)

// shutdownTimeout bounds how long graceful shutdown waits for in-flight
// requests to drain before srv.Shutdown gives up and returns an error.
const shutdownTimeout = 10 * time.Second

// Serve runs srv until ctx is cancelled, then shuts it down gracefully. main
// wires ctx to SIGINT/SIGTERM via signal.NotifyContext, so a termination signal
// triggers the shutdown path here; keeping the signal wiring in main leaves
// Serve fully unit-testable with an ordinary cancellable context.
//
// Shutdown order: in-flight HTTP requests are given shutdownTimeout to finish
// (srv.Shutdown), then every module that owns resources — any that implements
// io.Closer, such as the auth module's Postgres pool — is closed. Draining HTTP
// before closing the pool ensures no request is cut off from the database
// mid-flight.
//
// It blocks until shutdown completes and returns the first error encountered: a
// ListenAndServe bind failure (returned immediately, before any shutdown), a
// shutdown-deadline-exceeded error, or a module close error. The clean
// http.ErrServerClosed returned by ListenAndServe after Shutdown is not an
// error.
func Serve(ctx context.Context, srv *http.Server, modules ...Module) error {
	listenErr := listen(srv)

	select {
	case err := <-listenErr:
		// ListenAndServe returned before any shutdown was requested (e.g. the
		// port is already in use). The process is expected to exit, so module
		// resources are left for the OS to reclaim.
		return err
	case <-ctx.Done():
		return shutdown(srv, modules)
	}
}

// listen runs srv.ListenAndServe on its own goroutine and reports the outcome
// on the returned channel: a bind/serve failure, or nil once the server has
// stopped cleanly (Shutdown makes ListenAndServe return the expected
// http.ErrServerClosed, which is not a failure). The channel is buffered so the
// goroutine never blocks once Serve has moved on to the shutdown path.
func listen(srv *http.Server) <-chan error {
	listenErr := make(chan error, 1)
	go func() {
		log.Printf("auth-service listening on %s", srv.Addr)
		err := srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		listenErr <- err
	}()
	return listenErr
}

// shutdown drains in-flight HTTP requests (up to shutdownTimeout) and then
// releases module-owned resources — in that order, so no request is cut off
// from its database mid-flight. It returns the drain error and every module
// close error joined, so a caller can detect any failure with errors.Is (nil
// when both steps succeed).
func shutdown(srv *http.Server, modules []Module) error {
	log.Println("shutting down auth-service...")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	err := srv.Shutdown(ctx)
	if err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	err = errors.Join(err, closeModules(modules))

	log.Println("auth-service stopped")
	return err
}

// closeModules closes every module that implements io.Closer, skipping those
// without resources (no Close method). It always attempts all of them so one
// failure does not strand the rest, and returns their close errors joined
// (nil if none failed).
func closeModules(modules []Module) error {
	var errs []error
	for _, m := range modules {
		c, ok := m.(io.Closer)
		if !ok {
			continue
		}
		if err := c.Close(); err != nil {
			log.Printf("close module: %v", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
