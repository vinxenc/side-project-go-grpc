package core

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
)

// BaseController provides shared HTTP helpers for module controllers.
// Embed it in a module's controller to reuse these helpers:
//
//	type controller struct {
//		core.BaseController
//	}
type BaseController struct{}

// JSON writes v as a JSON response with the given status code.
//
// v is encoded into a buffer first, so an encoding failure results in a clean
// 500 (with nothing written to the client) rather than a partial body sent
// after a 200 status line.
func (BaseController) JSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		log.Printf("core: failed to encode JSON response: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Printf("core: failed to write JSON response: %v", err)
	}
}

// Error writes a JSON error response with the given status code.
func (c BaseController) Error(w http.ResponseWriter, status int, message string) {
	c.JSON(w, status, map[string]string{"error": message})
}
