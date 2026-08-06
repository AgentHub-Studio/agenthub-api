package middleware

import (
	"encoding/json"
	"net/http"
)

// DefaultMaxURLBytes é o cap padrão para o tamanho total da URL (path + query).
// Bug 279: sem limite, atacantes podiam mandar `?q=` com megabytes consumindo
// memória durante parse/log/forward para downstream. nginx default é 8KB,
// Apache 8190; Go net/http aceita até MaxHeaderBytes (1MB) por padrão.
const DefaultMaxURLBytes = 8 << 10 // 8 KB

// MaxURLBytes returns a middleware that rejects requests whose request-target
// exceeds `limit`. URL.RequestURI preserves the escaped path and only adds
// "?" when it is actually present, matching the bytes received by the server.
// Retorna 414 URI Too Long em formato JSON consistente com o resto da API
// ({"error":"..."}).
func MaxURLBytes(limit int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(r.URL.RequestURI()) > limit {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestURITooLong)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "request URI too long"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
