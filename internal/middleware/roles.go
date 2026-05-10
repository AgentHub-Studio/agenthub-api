package middleware

import (
	"context"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

type roleContextKey struct{}

// roleChecker is the minimal authorization contract used by role middlewares.
type roleChecker interface {
	HasRole(string) bool
}

type accessRoles struct {
	Roles []string `json:"roles"`
}

type roleClaims struct {
	jwt.RegisteredClaims
	RealmAccess    accessRoles            `json:"realm_access"`
	ResourceAccess map[string]accessRoles `json:"resource_access"`
}

func (c *roleClaims) HasRole(role string) bool {
	if c == nil || role == "" {
		return false
	}
	for _, r := range c.RealmAccess.Roles {
		if r == role {
			return true
		}
	}
	for _, access := range c.ResourceAccess {
		for _, r := range access.Roles {
			if r == role {
				return true
			}
		}
	}
	return false
}

func contextWithClaims(ctx context.Context, claims roleChecker) context.Context {
	return context.WithValue(ctx, roleContextKey{}, claims)
}

// claimsFromContext extracts a roleChecker from ctx, if present.
func claimsFromContext(ctx context.Context) roleChecker {
	claims, _ := ctx.Value(roleContextKey{}).(roleChecker)
	return claims
}

// SubjectFromContext returns the JWT subject (user identifier) when available,
// or "" if no claims are present. Bug 296: capabilities endpoint needs this.
func SubjectFromContext(ctx context.Context) string {
	claims, ok := ctx.Value(roleContextKey{}).(*roleClaims)
	if !ok || claims == nil {
		return ""
	}
	return claims.Subject
}

// RolesFromContext returns the deduplicated union of realm + resource roles
// in the JWT, or nil if no claims are present. Bug 296.
func RolesFromContext(ctx context.Context) []string {
	claims, ok := ctx.Value(roleContextKey{}).(*roleClaims)
	if !ok || claims == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var roles []string
	for _, r := range claims.RealmAccess.Roles {
		if _, dup := seen[r]; dup {
			continue
		}
		seen[r] = struct{}{}
		roles = append(roles, r)
	}
	for _, access := range claims.ResourceAccess {
		for _, r := range access.Roles {
			if _, dup := seen[r]; dup {
				continue
			}
			seen[r] = struct{}{}
			roles = append(roles, r)
		}
	}
	return roles
}

// RequireRole returns a middleware that allows only requests whose JWT contains
// the specified role (checked in both realm_access and resource_access).
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFromContext(r.Context())
			if claims == nil || !claims.HasRole(role) {
				// Bug 282: http.Error usa text/plain mesmo com body JSON
				writeJSONError(w, http.StatusForbidden, "forbidden: missing required role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
