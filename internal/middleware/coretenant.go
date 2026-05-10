package middleware

import (
	"net/http"

	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// CoreTenantID is the slug of the administrative tenant that owns
// platform-wide management endpoints (cross-tenant operations).
const CoreTenantID = "core"

// RequireCoreTenant rejects requests whose authenticated tenant is not
// the administrative core tenant. Must be chained AFTER the auth+tenant
// middleware so the tenantID is present in the request context.
func RequireCoreTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tenant.FromContext(r.Context()) != CoreTenantID {
			// Bug 282: http.Error usa text/plain mesmo com body JSON
			writeJSONError(w, http.StatusForbidden, "forbidden: core tenant only")
			return
		}
		next.ServeHTTP(w, r)
	})
}
