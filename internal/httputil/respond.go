// Package httputil provides helpers for writing JSON HTTP responses.
package httputil

import (
	"encoding/json"
	"net/http"
)

// errorBody is the standard error response envelope.
type errorBody struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// JSON writes v as a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes a standardised error JSON response.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, errorBody{Status: status, Message: message})
}

// NotFound writes a 404 error response.
func NotFound(w http.ResponseWriter, message string) {
	Error(w, http.StatusNotFound, message)
}

// BadRequest writes a 400 error response.
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, message)
}

// Forbidden writes a 403 error response.
func Forbidden(w http.ResponseWriter, message string) {
	Error(w, http.StatusForbidden, message)
}

// InternalServerError writes a 500 error response.
func InternalServerError(w http.ResponseWriter, message string) {
	Error(w, http.StatusInternalServerError, message)
}

// Conflict writes a 409 error response.
func Conflict(w http.ResponseWriter, message string) {
	Error(w, http.StatusConflict, message)
}

// UnprocessableEntity writes a 422 error response. Use for inputs
// que são bem-formados (parse JSON ok) mas falham validação
// semântica (campos obrigatórios vazios, enum value desconhecido,
// limites excedidos, etc) — distingue do 400 que cobre malformed
// requests sem alcançar a camada de service.
func UnprocessableEntity(w http.ResponseWriter, message string) {
	Error(w, http.StatusUnprocessableEntity, message)
}
