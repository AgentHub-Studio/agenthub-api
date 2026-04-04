//go:build integration

package multitenant_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/database/multitenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// TestMigrateAllTenants_NoTenants verifies that when the public.tenants table
// exists but contains no ACTIVE rows, MigrateAllTenants returns nil without error.
func TestMigrateAllTenants_NoTenants(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)

	// Ensure public.tenants table exists with the minimal shape expected.
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.tenants (
			id     TEXT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'ACTIVE'
		)
	`)
	require.NoError(t, err)

	// Table is empty — no tenants to migrate.
	err = multitenant.MigrateAllTenants(ctx, pool, t.TempDir())

	assert.NoError(t, err)
}
