package core_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"auth-service/src/core"
)

// closerModule is a core.Module that also implements io.Closer, recording that
// Close ran and optionally returning an error, to exercise Serve's
// resource-release step. It registers no routes.
type closerModule struct {
	closed   bool
	closeErr error
}

func (m *closerModule) Controller() core.Controller { return nil }

func (m *closerModule) Close() error {
	m.closed = true
	return m.closeErr
}

// freeAddr reserves a free loopback port and returns its address. The listener
// is closed immediately so Serve can bind it; the tiny reuse window is
// acceptable in a test.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// waitListening blocks until addr accepts a TCP connection, so tests only
// cancel the context once Serve's ListenAndServe is actually bound. This makes
// the graceful-shutdown path deterministic (no ListenAndServe-vs-Shutdown race).
func waitListening(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server never started listening on %s", addr)
}

// TestServe_GracefulShutdownClosesModules verifies that cancelling the context
// drives a clean shutdown: Serve returns nil and every io.Closer module is
// closed after HTTP drains.
func TestServe_GracefulShutdownClosesModules(t *testing.T) {
	addr := freeAddr(t)
	srv := &http.Server{Addr: addr}
	mod := &closerModule{}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- core.Serve(ctx, srv, mod) }()

	waitListening(t, addr)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve returned error on clean shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after context cancellation")
	}

	if !mod.closed {
		t.Error("expected module to be closed during graceful shutdown")
	}
}

// TestServe_ReturnsModuleCloseError verifies that when shutdown is clean but a
// module's Close fails, Serve surfaces that close error.
func TestServe_ReturnsModuleCloseError(t *testing.T) {
	addr := freeAddr(t)
	srv := &http.Server{Addr: addr}
	wantErr := errors.New("pool close failed")
	mod := &closerModule{closeErr: wantErr}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- core.Serve(ctx, srv, mod) }()

	waitListening(t, addr)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected module close error %v, got %v", wantErr, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after context cancellation")
	}
}

// TestServe_ReturnsListenError verifies that a ListenAndServe bind failure is
// returned immediately, before any shutdown is requested (ctx stays live).
func TestServe_ReturnsListenError(t *testing.T) {
	// Port 70000 is outside the valid 0–65535 range, so the bind fails.
	srv := &http.Server{Addr: "127.0.0.1:70000"}

	errCh := make(chan error, 1)
	go func() { errCh <- core.Serve(context.Background(), srv) }()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected Serve to return the ListenAndServe bind error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return the bind error")
	}
}
