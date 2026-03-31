package middleware

import (
	"net/http"
)

// Chain holds the configured middleware stack.
type Chain struct {
	keycloakBaseURL string
	corsOrigins     []string
}

// New creates a new middleware Chain.
func New(keycloakBaseURL string, corsOrigins []string) *Chain {
	return &Chain{
		keycloakBaseURL: keycloakBaseURL,
		corsOrigins:     corsOrigins,
	}
}

// Public returns middleware for unauthenticated routes: Recovery → RequestID → Logger → CORS.
func (c *Chain) Public() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		Recovery,
		RequestID,
		Logger,
		CORS(c.corsOrigins),
	}
}

// Protected returns middleware for authenticated routes: Public stack + Auth + Tenant.
// Auth and Tenant middlewares are provided by agenthub-go-commons once implemented.
func (c *Chain) Protected() []func(http.Handler) http.Handler {
	return append(c.Public(),
		authMiddleware(c.keycloakBaseURL),
		tenantMiddleware(),
	)
}

// authMiddleware returns a JWT validation middleware backed by Keycloak JWKS.
// TODO: replace with auth.Middleware(auth.Config{KeycloakBaseURL: keycloakBaseURL})
// once agenthub-go-commons/auth exports the Middleware function.
func authMiddleware(keycloakBaseURL string) func(http.Handler) http.Handler {
	_ = keycloakBaseURL // used when commons auth is wired
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Placeholder: pass-through until commons/auth is implemented.
			next.ServeHTTP(w, r)
		})
	}
}

// tenantMiddleware extracts tenantID from JWT issuer and injects into context.
// TODO: replace with tenant.Middleware() once agenthub-go-commons/tenant exports it.
func tenantMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Placeholder: pass-through until commons/tenant is implemented.
			next.ServeHTTP(w, r)
		})
	}
}
