package respond

import (
	"encoding/json"
	"net/http"
)

// JSON writes a JSON response with the given status code and body.
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Error writes a JSON error response.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

// NoContent writes a 204 No Content response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
