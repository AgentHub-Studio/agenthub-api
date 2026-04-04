package httputil

import (
	"encoding/json"
	"net/http"
)

// JSON writes v as a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ErrorResponse is the standard error body returned by AgentHub APIs.
type ErrorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// Error writes a standardized JSON error response.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, ErrorResponse{Status: status, Message: message})
}

// NotFound writes a 404 JSON error response.
func NotFound(w http.ResponseWriter, resource string) {
	Error(w, http.StatusNotFound, resource+" not found")
}

// BadRequest writes a 400 JSON error response.
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, message)
}

// InternalServerError writes a 500 JSON error response.
func InternalServerError(w http.ResponseWriter) {
	Error(w, http.StatusInternalServerError, "internal server error")
}

// NoContent writes a 204 response with no body.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
