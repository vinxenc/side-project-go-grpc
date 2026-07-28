package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewMux(t *testing.T) {
	mux := NewMux()
	if mux == nil {
		t.Error("NewMux returned nil")
	}
}

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		wantStatus     int
		wantContentLen bool
		wantBody       *healthResponse
	}{
		{
			name:           "GET /healthz returns 200 with ok status",
			method:         "GET",
			path:           "/healthz",
			wantStatus:     http.StatusOK,
			wantContentLen: true,
			wantBody:       &healthResponse{Status: "ok"},
		},
		{
			name:           "POST /healthz returns 405",
			method:         "POST",
			path:           "/healthz",
			wantStatus:     http.StatusMethodNotAllowed,
			wantContentLen: false,
			wantBody:       nil,
		},
		{
			name:           "PUT /healthz returns 405",
			method:         "PUT",
			path:           "/healthz",
			wantStatus:     http.StatusMethodNotAllowed,
			wantContentLen: false,
			wantBody:       nil,
		},
		{
			name:           "DELETE /healthz returns 405",
			method:         "DELETE",
			path:           "/healthz",
			wantStatus:     http.StatusMethodNotAllowed,
			wantContentLen: false,
			wantBody:       nil,
		},
		{
			name:           "GET /nope returns 404",
			method:         "GET",
			path:           "/nope",
			wantStatus:     http.StatusNotFound,
			wantContentLen: false,
			wantBody:       nil,
		},
		{
			name:           "GET /health (not /healthz) returns 404",
			method:         "GET",
			path:           "/health",
			wantStatus:     http.StatusNotFound,
			wantContentLen: false,
			wantBody:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			mux := NewMux()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantBody != nil {
				// For successful responses, check Content-Type and body
				contentType := rec.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("got Content-Type %q, want %q", contentType, "application/json")
				}

				var got healthResponse
				if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
					t.Fatalf("failed to decode body: %v", err)
				}

				if got.Status != tt.wantBody.Status {
					t.Errorf("got status %q, want %q", got.Status, tt.wantBody.Status)
				}
			}
		})
	}
}

func TestHealthHandlerConcurrency(t *testing.T) {
	t.Parallel()

	mux := NewMux()

	// Run multiple concurrent requests to /healthz
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/healthz", nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("got status %d, want 200", rec.Code)
			}

			var got healthResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Errorf("failed to decode body: %v", err)
			}

			if got.Status != "ok" {
				t.Errorf("got status %q, want %q", got.Status, "ok")
			}

			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
