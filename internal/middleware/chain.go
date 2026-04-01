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
// Auth validates Keycloak JWTs via JWKS; Tenant extracts tenantID from the JWT issuer.
// TODO: replace stubs with agenthub-go-commons auth.Middleware and tenant.Middleware
// once that package is published to the Go module proxy.
func (c *Chain) Protected() []func(http.Handler) http.Handler {
	return append(c.Public(),
		authMiddleware(c.keycloakBaseURL),
		tenantMiddleware(),
	)
}

// authMiddleware returns a JWT validation middleware backed by Keycloak JWKS.
// Stub: pass-through until agenthub-go-commons/auth is published.
func authMiddleware(keycloakBaseURL string) func(http.Handler) http.Handler {
	_ = keycloakBaseURL
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}

// tenantMiddleware extracts tenantID from JWT issuer claim and stores it in context.
// Stub: pass-through until agenthub-go-commons/tenant is published.
func tenantMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}
