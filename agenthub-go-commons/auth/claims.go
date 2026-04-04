package auth

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const claimsKey contextKey = "claims"

// Claims holds the parsed JWT claims for a request.
type Claims struct {
	jwt.RegisteredClaims
	RealmAccess    RealmAccess            `json:"realm_access"`
	ResourceAccess map[string]ClientRoles `json:"resource_access"`
	PreferredName  string                 `json:"preferred_username"`
}

// RealmAccess holds the realm-level roles from the JWT.
type RealmAccess struct {
	Roles []string `json:"roles"`
}

// ClientRoles holds the client-level roles from the JWT.
type ClientRoles struct {
	Roles []string `json:"roles"`
}

// ClaimsFromContext extracts the Claims from ctx. Returns nil if not set.
func ClaimsFromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

// contextWithClaims returns a copy of ctx with the claims stored.
func contextWithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

// HasRole reports whether the Claims contain the given role in either
// realm_access.roles or any resource_access.*.roles entry.
func (c *Claims) HasRole(role string) bool {
	if c == nil {
		return false
	}
	for _, r := range c.RealmAccess.Roles {
		if r == role {
			return true
		}
	}
	for _, cr := range c.ResourceAccess {
		for _, r := range cr.Roles {
			if r == role {
				return true
			}
		}
	}
	return false
}
