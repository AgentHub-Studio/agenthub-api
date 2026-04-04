package middleware

import (
	"net/http"
	"strings"
)

// CORS returns a middleware that sets CORS headers for the given allowed origins.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	originsMap := make(map[string]bool, len(allowedOrigins))
	allowAll := false
	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
			break
		}
		originsMap[strings.TrimSpace(o)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (allowAll || originsMap[origin]) {
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
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
