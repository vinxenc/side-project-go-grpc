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

	"auth-service/configs"
	"auth-service/core"
	"auth-service/src/modules/auth"
	"auth-service/src/modules/health"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

func main() {
	// Load and validate configuration (.env + env vars). Fails fast on missing
	// DATABASE_URL, bad LIMEN_SECRET, etc. godotenv.Load is called inside configs.Load.
	cfg, err := configs.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	mux := http.NewServeMux()

	// Wrap the mux with Huma. Operations are registered onto the API, but the
	// underlying mux is what the server serves (see srv.Handler below).
	config := huma.DefaultConfig("auth-service", "1.0.0")
	// DefaultConfig installs a SchemaLinkTransformer via CreateHooks, which
	// decorates response bodies with a "$schema" field and adds Link headers.
	// Drop it to keep the health payload byte-compatible: {"status","time"}.
	config.CreateHooks = nil
	// Declare the cookieAuth security scheme referenced by the protected auth
	// operations so the generated OpenAPI document is valid (an operation may
	// only reference a scheme defined under components.securitySchemes).
	if config.Components == nil {
		config.Components = &huma.Components{}
	}
	if config.Components.SecuritySchemes == nil {
		config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
	}
	config.Components.SecuritySchemes["cookieAuth"] = &huma.SecurityScheme{
		Type: "apiKey",
		In:   "cookie",
		Name: "limen_session",
	}
	api := humago.New(mux, config)

	// Build the auth module; fail fast if limen cannot initialise (e.g. unreachable
	// DB, bad secret). auth.New opens Postgres and pings within 5 seconds.
	authModule, err := auth.New(cfg)
	if err != nil {
		log.Fatalf("auth module init failed: %v", err)
	}

	// Register all feature modules.
	core.RegisterModules(api,
		health.New(),
		authModule,
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
