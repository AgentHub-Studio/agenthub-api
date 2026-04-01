package middleware

import (
	"context"
	"net/http"
)

// roleChecker is a minimal interface satisfied by agenthub-go-commons/auth.Claims.
// Avoids importing go-commons before PR #12 is merged.
type roleChecker interface {
	HasRole(string) bool
}

// claimsFromContext extracts a roleChecker from ctx, if present.
// Returns nil when the auth middleware is in placeholder mode (dev).
func claimsFromContext(ctx context.Context) roleChecker {
	// TODO: replace with auth.ClaimsFromContext(ctx) once go-commons is wired (PR #12 / #14).
	_ = ctx
	return nil
}

// RequireRole returns a middleware that allows only requests whose JWT contains
// the specified role (checked in both realm_access and resource_access).
//
// Dev mode (auth middleware placeholder): claims are nil → pass-through.
// Production (go-commons auth wired): requests without the role return 403.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFromContext(r.Context())
			if claims != nil && !claims.HasRole(role) {
				http.Error(w, `{"error":"forbidden: missing required role"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
