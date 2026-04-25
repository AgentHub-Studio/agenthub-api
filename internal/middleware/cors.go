package middleware

import (
	"net/http"
	"strings"
)

// CORS returns a middleware that sets CORS headers for the given allowed origins.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	originsMap := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		origin := strings.TrimSpace(o)
		if origin != "" && origin != "*" {
			originsMap[origin] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && originsMap[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "86400")

				// Mirror requested headers so any custom header the client sends is allowed.
				// This eliminates the need to maintain a fixed allowlist and handles
				// headers like X-OpenAI-API-Key, X-Tenant-ID, X-Request-ID, etc.
				if requested := r.Header.Get("Access-Control-Request-Headers"); requested != "" {
					w.Header().Set("Access-Control-Allow-Headers", requested)
				} else {
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, X-Request-ID, Cache-Control")
				}
			}
			// BUG-DEPR2 fix: intercept ALL OPTIONS requests here so the router
			// wildcard r.Options("/*") can be removed. Without that wildcard,
			// chi no longer registers a method for unregistered paths, meaning
			// GET/POST to non-existent routes correctly returns 404 (not 405).
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
