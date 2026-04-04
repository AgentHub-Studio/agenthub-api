// Package multitenant provides MigrateAllTenants which iterates all active
// tenants in public.tenants and applies schema migrations to each ah_{tenantID}.
//
// Usage:
//
//	err := multitenant.MigrateAllTenants(ctx, pool, "migrations/schemas")
package multitenant
