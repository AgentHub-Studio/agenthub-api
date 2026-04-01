package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
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
// Auth validates JWT structure; Tenant extracts tenantID from the Keycloak issuer claim.
func (c *Chain) Protected() []func(http.Handler) http.Handler {
	return append(c.Public(),
		authMiddleware(c.keycloakBaseURL),
		tenantMiddleware(),
	)
}

// authMiddleware validates JWT structure: requires a Bearer token with three
// dot-separated base64 segments (header.payload.signature).
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

var realmRegex = regexp.MustCompile(`/realms/([^/]+)`)

// tenantMiddleware extracts tenantID from the JWT issuer claim and stores it in context.
// Runs after authMiddleware, which has already verified the token is structurally valid.
// Extracts the realm name from the Keycloak issuer URL
// (e.g. "https://keycloak.example.com/realms/my-tenant" → "my-tenant").
func tenantMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenStr, _ := strings.CutPrefix(authHeader, "Bearer ")
			parts := strings.Split(tokenStr, ".")
			if len(parts) != 3 {
				http.Error(w, `{"error":"malformed jwt token"}`, http.StatusUnauthorized)
				return
			}

			// Decode JWT payload (base64url, no padding).
			payload, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				http.Error(w, `{"error":"cannot decode jwt payload"}`, http.StatusUnauthorized)
				return
			}

			var claims struct {
				Issuer string `json:"iss"`
			}
			if err := json.Unmarshal(payload, &claims); err != nil || claims.Issuer == "" {
				http.Error(w, `{"error":"missing iss claim in jwt"}`, http.StatusUnauthorized)
				return
			}

			m := realmRegex.FindStringSubmatch(claims.Issuer)
			if len(m) < 2 {
				http.Error(w, `{"error":"cannot extract tenantID from jwt issuer"}`, http.StatusUnauthorized)
				return
			}

			ctx := tenant.NewContext(r.Context(), m[1])
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
