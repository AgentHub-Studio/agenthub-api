package middleware

import (
	"encoding/json"
	"net/http"
)

// writeJSONError escreve uma resposta de erro JSON com Content-Type correto.
// Bug 282: substitui http.Error que usa text/plain mesmo com body JSON.
// Compatível com o shape {"error":"..."} usado em todo o resto da API.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
