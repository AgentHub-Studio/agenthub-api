// Package tenant provides multi-tenant middleware for AgentHub Go services.
// It extracts the tenantID from the Keycloak JWT issuer claim (regex /realms/([^/]+))
// and injects it into context.Context. The database package uses this to execute
// SET search_path TO ah_{tenantID}, public before each query.
//
// Usage:
//
//	r.Use(auth.JWTMiddleware(...))   // must run before TenantMiddleware
//	r.Use(tenant.TenantMiddleware())
package tenant
