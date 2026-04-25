package middleware

import (
	"net/http"
)

// ProxyServiceRequired enforces the PROXY_SERVICE role on a route.
func ProxyServiceRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := claimsFromContext(r.Context())
		if claims == nil || !claims.HasRole("PROXY_SERVICE") {
			http.Error(w, `{"error":"forbidden: PROXY_SERVICE role required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
