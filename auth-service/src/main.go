package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auth-service/src/core"
	"auth-service/src/setting"
	"auth-service/src/modules/auth"
	"auth-service/src/modules/health"
)

func main() {
	// Load and validate configuration (.env + env vars). Fails fast on missing
	// DATABASE_URL, bad LIMEN_SECRET, etc. godotenv.Load is called inside setting.Load.
	cfg, err := setting.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	mux := http.NewServeMux()

	// Wrap the mux with Huma. Operations are registered onto the API, but the
	// underlying mux is what the server serves (see srv.Handler below). The API
	// configuration lives in core.NewAPI so production and tests share it.
	api := core.NewAPI(mux)

	// Register all feature modules. auth.New opens Postgres and wires limen; if
	// that fails it logs the cause and registers no auth routes (the service
	// still starts).
	core.RegisterModules(api,
		health.New(),
		auth.New(auth.Config{
			DatabaseURL: cfg.DatabaseURL,
			Secret:      cfg.Secret,
			BaseURL:     cfg.BaseURL,
		}),
	)

	addr := ":8080"
	log.Printf("OpenAPI docs available at http://localhost%s/docs", addr)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start the server in the background so main can wait for a shutdown signal.
	go func() {
		log.Printf("auth-service listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Block until an interrupt or terminate signal is received.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	// Gracefully shut down, allowing in-flight requests to finish.
	log.Println("shutting down auth-service...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Println("auth-service stopped")
}
