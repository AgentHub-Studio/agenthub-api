// Package auth provides JWT middleware and Keycloak JWKS validation
// for AgentHub Go services. It caches JWKS per tenant and exposes
// claims via context.Context.
//
// Usage:
//
//	r.Use(auth.JWTMiddleware(auth.Config{
//	    KeycloakBaseURL: cfg.KeycloakBaseURL,
//	    AllowedIssuers:  []string{"https://keycloak.example.com/realms/"},
//	}))
package auth
