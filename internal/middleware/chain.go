package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
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

// CORSHandler returns the CORS middleware for use at the root router level.
// Must be applied before any route groups so that OPTIONS preflight requests
// are handled before chi returns 405 Method Not Allowed.
func (c *Chain) CORSHandler() func(http.Handler) http.Handler {
	return CORS(c.corsOrigins)
}

// Public returns middleware for unauthenticated routes: Recovery → RequestID → Logger.
// CORS is applied at root router level via CORSHandler(), not here.
func (c *Chain) Public() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		Recovery,
		RequestID,
		Logger,
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
// Until agenthub-go-commons/auth is published, this performs structural JWT validation:
// requires a Bearer token with three dot-separated base64 segments (header.payload.signature).
// Full JWKS-based signature verification is deferred to the commons library.
func authMiddleware(keycloakBaseURL string) func(http.Handler) http.Handler {
	_ = keycloakBaseURL
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok || token == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing or invalid authorization header"})
				return
			}
			// Structural JWT check: header.payload.signature
			parts := strings.Split(token, ".")
			if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "malformed jwt token"})
				return
			}
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
