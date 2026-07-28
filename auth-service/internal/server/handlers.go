package server

import (
	"encoding/json"
	"net/http"
)

// healthResponse is the JSON body returned by the health endpoint.
type healthResponse struct {
	Status string `json:"status"` // always "ok" for a 200
}

// NewMux constructs and returns the fully-wired http.ServeMux for the service.
// Kept separate from server construction so tests can hit it via httptest.
func NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	return mux
}

// healthHandler writes {"status":"ok"} with 200 and Content-Type application/json.
// Only responds to GET; the "GET /healthz" mux pattern (Go 1.22+) enforces the method,
// returning 405 automatically for non-GET requests.
// JSON encode failure is practically impossible for this fixed struct; the header is
// written before encoding, so any encode error is silently ignored.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // encode error is practically impossible for this fixed struct
	json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
}
