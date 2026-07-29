package main

import (
	"log"
	"net/http"
	"time"

	"auth-service/core"
	"auth-service/src/modules/health"
)

func main() {
	mux := http.NewServeMux()

	// Register all feature modules.
	core.RegisterModules(mux,
		health.New(),
	)

	addr := ":8080"
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("auth-service listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
