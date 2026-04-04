package tenant

import (
	"net/http"
	"regexp"

	"github.com/AgentHub-Studio/agenthub-go-commons/auth"
)

var realmRegex = regexp.MustCompile(`/realms/([^/]+)`)

// Middleware extracts the tenantID from the Keycloak JWT issuer (already parsed by auth.Middleware)
// and stores it in context. Must run AFTER auth.Middleware.
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.ClaimsFromContext(r.Context())
			if claims == nil {
				http.Error(w, "tenant: missing JWT claims", http.StatusUnauthorized)
				return
			}

			issuer, _ := claims.GetIssuer()
			tenantID := extractTenantID(issuer)
			if tenantID == "" {
				http.Error(w, "tenant: cannot extract tenantID from issuer", http.StatusUnauthorized)
				return
			}

			ctx := NewContext(r.Context(), tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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
