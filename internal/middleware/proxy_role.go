package middleware

import (
	"net/http"
)

// ProxyServiceRequired enforces the PROXY_SERVICE role on a route.
func ProxyServiceRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := claimsFromContext(r.Context())
		if claims == nil || !claims.HasRole("PROXY_SERVICE") {
			// Bug 282: http.Error usa text/plain — quebra clientes que parseiam JSON
			// baseado em Content-Type. Body é JSON, então setamos CT explícito.
			writeJSONError(w, http.StatusForbidden, "forbidden: PROXY_SERVICE role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
