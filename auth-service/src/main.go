package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auth-service/src/core"
	"auth-service/src/modules/auth"
	"auth-service/src/modules/health"
	"auth-service/src/setting"
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

	// Build the feature modules once, then both register their routes and hand
	// them to core.Serve so their resources (e.g. the auth Postgres pool) are
	// closed on shutdown. auth.New opens Postgres and wires limen; if that fails
	// it logs the cause and registers no auth routes (the service still starts).
	modules := []core.Module{
		health.New(),
		auth.New(auth.Config{
			DatabaseURL: cfg.DatabaseURL,
			Secret:      cfg.Secret,
			BaseURL:     cfg.BaseURL,
		}),
	}
	core.RegisterModules(api, modules...)

	log.Printf("OpenAPI docs available at %s/docs", cfg.BaseURL)
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Cancel the context on interrupt or terminate; core.Serve treats that as a
	// graceful-shutdown request. stop() restores default signal handling so a
	// second Ctrl-C during shutdown terminates immediately.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := core.Serve(ctx, srv, modules...); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
