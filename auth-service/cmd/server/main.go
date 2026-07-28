package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"auth-service/internal/server"
)

func main() {
	if err := run(); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

// run is the testable, error-returning core called by main.
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := server.LoadConfig()

	mux := server.NewMux()
	srv := server.New(cfg, mux)

	log.Printf("starting server on :%s", cfg.Port)

	if err := server.Run(ctx, srv, cfg.ShutdownTimeout); err != nil {
		return fmt.Errorf("run: %w", err)
	}

	log.Printf("server stopped")
	return nil
}
