// Package testutil provides test helpers, fixtures, and Testcontainers setup
// for AgentHub Go integration tests.
//
// Usage:
//
//	pool := testutil.NewPostgresContainer(t)
//	testutil.MigratePublic(t, pool, "../../migrations/public")
//	testutil.MigrateTenant(t, pool, "test-tenant", "../../migrations/schemas")
package testutil
