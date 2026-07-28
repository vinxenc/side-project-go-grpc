package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		port        string
		readTimeout time.Duration
		wantAddr    string
	}{
		{
			name:        "builds server with default port",
			port:        "8080",
			readTimeout: 5 * time.Second,
			wantAddr:    ":8080",
		},
		{
			name:        "builds server with custom port",
			port:        "9090",
			readTimeout: 5 * time.Second,
			wantAddr:    ":9090",
		},
		{
			name:        "builds server with non-numeric port",
			port:        "invalid",
			readTimeout: 5 * time.Second,
			wantAddr:    ":invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Port:            tt.port,
				ReadTimeout:     tt.readTimeout,
				WriteTimeout:    10 * time.Second,
				IdleTimeout:     120 * time.Second,
				ShutdownTimeout: 10 * time.Second,
			}

			mux := NewMux()
			srv := New(cfg, mux)

			if srv.Addr != tt.wantAddr {
				t.Errorf("got Addr %q, want %q", srv.Addr, tt.wantAddr)
			}

			if srv.ReadTimeout != tt.readTimeout {
				t.Errorf("got ReadTimeout %v, want %v", srv.ReadTimeout, tt.readTimeout)
			}

			if srv.WriteTimeout != 10*time.Second {
				t.Errorf("got WriteTimeout %v, want %v", srv.WriteTimeout, 10*time.Second)
			}

			if srv.IdleTimeout != 120*time.Second {
				t.Errorf("got IdleTimeout %v, want %v", srv.IdleTimeout, 120*time.Second)
			}

			if srv.Handler != mux {
				t.Error("Handler was not set correctly")
			}
		})
	}
}

func TestRunWithCancelledContext(t *testing.T) {
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Extract just the port
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to parse address: %v", err)
	}

	cfg := Config{
		Port:            port,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}

	mux := NewMux()
	srv := New(cfg, mux)

	// Create a pre-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Run should gracefully shut down cleanly even with a pre-cancelled context
	err = Run(ctx, srv, cfg.ShutdownTimeout)

	// Should not return an error for clean shutdown
	if err != nil {
		t.Errorf("expected no error on clean shutdown with pre-cancelled context, got %v", err)
	}
}

func TestRunWithAvailablePort(t *testing.T) {
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Extract just the port
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to parse address: %v", err)
	}

	cfg := Config{
		Port:            port,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}

	mux := NewMux()
	srv := New(cfg, mux)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start the server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- Run(ctx, srv, cfg.ShutdownTimeout)
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify the server is actually listening
	dialAddr := net.JoinHostPort(host, port)
	conn, err := net.Dial("tcp", dialAddr)
	if err != nil {
		t.Fatalf("failed to connect to server: %v", err)
	}
	conn.Close()

	// Cancel the context to trigger shutdown
	cancel()

	// Wait for the server to shut down
	runErr := <-serverErr

	// Should complete without error (clean shutdown)
	if runErr != nil {
		t.Errorf("expected no error on clean shutdown, got %v", runErr)
	}
}

func TestRunPortInUse(t *testing.T) {
	// Find an available port and start a blocking server there
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}

	// Extract the port number correctly
	addr := listener.Addr().(*net.TCPAddr)
	port := strconv.Itoa(addr.Port)

	// Close the initial listener so the blocking server can bind to that address
	listener.Close()

	// Start a listener on the same address that our server will try to bind to (all interfaces)
	// This will keep the port occupied so our server's bind will fail
	blockingListener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		t.Fatalf("failed to bind blocking listener to port %s: %v", port, err)
	}

	// Now try to start our server on the same port (should fail)
	cfg := Config{
		Port:            port,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}

	mux := NewMux()
	srv := New(cfg, mux)

	// Create a context with a timeout to ensure the test doesn't hang
	// Run should fail immediately when ListenAndServe fails due to port in use
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// This should fail immediately because the port is already in use
	err = Run(ctx, srv, cfg.ShutdownTimeout)

	// Clean up the blocking listener
	blockingListener.Close()

	// We expect an error due to port already in use
	if err == nil {
		t.Error("expected error when port is already in use, got nil")
	}
}

func TestRunShutdownTimeout(t *testing.T) {
	// This test verifies that a very short shutdown timeout can trigger a deadline error
	// We create a server with a very short timeout to simulate a timeout scenario

	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Extract just the port
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to parse address: %v", err)
	}

	cfg := Config{
		Port:            port,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 1 * time.Millisecond, // Very short timeout
	}

	// Create a custom handler that blocks on shutdown
	// This simulates slow-draining requests
	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow handler that holds the connection open
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	srv := New(cfg, mux)

	ctx, cancel := context.WithCancel(context.Background())

	// Start the server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- Run(ctx, srv, cfg.ShutdownTimeout)
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Start a slow request in a goroutine
	dialAddr := net.JoinHostPort(host, port)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		_, _ = client.Get("http://" + dialAddr + "/slow")
	}()

	// Give the request time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel the context to trigger shutdown
	cancel()

	// Wait for the server to attempt shutdown
	// The very short timeout should cause a deadline exceeded error
	runErr := <-serverErr

	// With a 1ms shutdown timeout and a 2s slow handler, we should get a context.DeadlineExceeded error
	if !errors.Is(runErr, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded on shutdown timeout, got: %v", runErr)
	}
}

func TestRunCleanExit(t *testing.T) {
	// Test that Run returns nil (clean exit) when context is cancelled normally
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Extract just the port
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to parse address: %v", err)
	}

	cfg := Config{
		Port:            port,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}

	mux := NewMux()
	srv := New(cfg, mux)

	ctx, cancel := context.WithCancel(context.Background())

	// Start the server
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- Run(ctx, srv, cfg.ShutdownTimeout)
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify the server is listening
	dialAddr := net.JoinHostPort(host, port)
	conn, err := net.Dial("tcp", dialAddr)
	if err != nil {
		t.Fatalf("failed to connect to server: %v", err)
	}
	conn.Close()

	// Cancel the context
	cancel()

	// Wait for the server to shut down
	runErr := <-serverErr

	// Should be nil for clean shutdown
	if runErr != nil {
		t.Errorf("expected nil error on clean shutdown, got %v", runErr)
	}
}

func TestRunErrorServerClosed(t *testing.T) {
	// Verify that http.ErrServerClosed is treated as a clean shutdown (returns nil)
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find available port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Extract just the port
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to parse address: %v", err)
	}

	cfg := Config{
		Port:            port,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}

	mux := NewMux()
	srv := New(cfg, mux)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run the server
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- Run(ctx, srv, cfg.ShutdownTimeout)
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown via context cancellation
	cancel()

	// The server should shut down cleanly
	runErr := <-serverErr

	// http.ErrServerClosed should be treated as nil (clean shutdown)
	if runErr != nil {
		t.Errorf("expected clean shutdown (nil), got %v", runErr)
	}
}
