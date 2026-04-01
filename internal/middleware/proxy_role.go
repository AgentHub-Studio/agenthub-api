package middleware

import (
	"context"
	"net/http"
)

// proxyClaimsKey is the context key used by agenthub-go-commons/auth to store JWT claims.
// Kept unexported to avoid coupling — we only need HasRole behaviour here.
type proxyClaimsKey = struct{ pkg string }

// proxyRoleChecker is a minimal interface satisfied by go-commons Claims.
type proxyRoleChecker interface {
	HasRole(string) bool
}

// claimsFromCtx extracts a proxyRoleChecker from ctx, if present.
// Compatible with the claim storage used by agenthub-go-commons/auth.
func claimsFromCtx(ctx context.Context) proxyRoleChecker {
	// go-commons stores claims under a package-internal key of type contextKey("claims").
	// We cannot access that key directly without importing the package.
	// Once go-commons auth middleware is wired (PR #12 / #14), update this function
	// to: return auth.ClaimsFromContext(ctx)
	_ = ctx
	return nil
}

// ProxyServiceRequired enforces the PROXY_SERVICE role on a route.
//
// Dev mode (auth middleware is placeholder): claims are nil → pass-through.
// Production (go-commons auth wired): requests without the role return 403.
//
// TODO: once agenthub-go-commons/auth PR is merged and go-commons is a dependency,
// replace claimsFromCtx with auth.ClaimsFromContext and remove this file.
func ProxyServiceRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := claimsFromCtx(r.Context())
		if claims != nil && !claims.HasRole("PROXY_SERVICE") {
			http.Error(w, `{"error":"forbidden: PROXY_SERVICE role required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
