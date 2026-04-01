package middleware

import (
	"net/http"

	"github.com/AgentHub-Studio/agenthub-go-commons/auth"
	"github.com/AgentHub-Studio/agenthub-go-commons/tenant"
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
func (c *Chain) Protected() []func(http.Handler) http.Handler {
	return append(c.Public(),
		auth.Middleware(auth.Config{KeycloakBaseURL: c.keycloakBaseURL}),
		tenant.Middleware(),
	)
}
