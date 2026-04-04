package auth

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var realmRegex = regexp.MustCompile(`/realms/([^/]+)`)

// Middleware returns an HTTP middleware that validates Keycloak JWTs.
// It caches JWKS per tenant with a 5-minute TTL.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	cache := newJWKSCache(cfg.KeycloakBaseURL, 5*time.Minute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr, err := extractBearerToken(r)
			if err != nil {
				http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			claims, err := parseToken(r.Context(), tokenStr, cfg, cache)
			if err != nil {
				http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			ctx := contextWithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns a middleware that allows only requests whose JWT contains
// the specified role (checked in both realm_access and resource_access).
// Must be placed after Middleware so that claims are present in context.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromContext(r.Context())
			if !claims.HasRole(role) {
				http.Error(w, "forbidden: missing required role", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractBearerToken(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", fmt.Errorf("missing Authorization header")
	}
	if !strings.HasPrefix(h, "Bearer ") {
		return "", fmt.Errorf("Authorization header must start with Bearer")
	}
	return strings.TrimPrefix(h, "Bearer "), nil
}

func parseToken(ctx context.Context, tokenStr string, cfg Config, cache *jwksCache) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		issuer, _ := t.Claims.GetIssuer()
		tenantID := extractTenantID(issuer)
		if tenantID == "" {
			return nil, fmt.Errorf("cannot extract tenantID from issuer %q", issuer)
		}

		// Validate against AllowedIssuers if configured
		if len(cfg.AllowedIssuers) > 0 {
			allowed := false
			for _, ai := range cfg.AllowedIssuers {
				if strings.HasPrefix(issuer, ai) {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil, fmt.Errorf("issuer %q is not in the allowed issuers list", issuer)
			}
		}

		kid, _ := t.Header["kid"].(string)
		return cache.getKey(ctx, tenantID, kid)
	}, jwt.WithExpirationRequired())

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	return claims, nil
}

// extractTenantID parses the tenantID from a Keycloak issuer URL.
// e.g. "https://keycloak.example.com/realms/my-tenant" → "my-tenant"
func extractTenantID(issuer string) string {
	m := realmRegex.FindStringSubmatch(issuer)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
