package core

import (
	"encoding/json"
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
func (BaseController) JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes a JSON error response with the given status code.
func (c BaseController) Error(w http.ResponseWriter, status int, message string) {
	c.JSON(w, status, map[string]string{"error": message})
}
