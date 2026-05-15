//go:build integration

package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for CoreCapabilityToolPermissionLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000118 creates capability_tool_permission table + 12 rows
//
// The ah_core.capability_tool_permission table is created by migration 000118
// itself (no prior migration defines it).

const capabilityToolPermissionMigration = "000118_seed_capability_tool_permissions.up.sql"
const capabilityToolPermissionMigrationDown = "000118_seed_capability_tool_permissions.down.sql"

func TestIntegration_ToolPermission_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityToolPermissionMigration)

	loader := core.NewCoreCapabilityToolPermissionLoader(pool)
	got, err := loader.LoadCapabilityToolPermissions(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedToolPermissionCount, len(got),
		"DB row count must match SeedToolPermissionCount (12) after migration 000118")
}

func TestIntegration_ToolPermission_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityToolPermissionLoader(pool)
	got, err := loader.LoadCapabilityToolPermissions(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_ToolPermission_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityToolPermissionMigration)
	// Apply 000118 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityToolPermissionMigration)

	loader := core.NewCoreCapabilityToolPermissionLoader(pool)
	got, err := loader.LoadCapabilityToolPermissions(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedToolPermissionCount, len(got),
		"double-apply of migration 000118 must still produce exactly 12 tool permission rows (idempotent)")
}

func TestIntegration_ToolPermission_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityToolPermissionMigration)

	loader := core.NewCoreCapabilityToolPermissionLoader(pool)
	before, err := loader.LoadCapabilityToolPermissions(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed must produce tool permission rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityToolPermissionMigrationDown)

	after, err := loader.LoadCapabilityToolPermissions(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 12 tool permission rows and drop the table")
}

func TestIntegration_ToolPermission_GetToolPermissionForResearcherWebSearch(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityToolPermissionMigration)

	loader := core.NewCoreCapabilityToolPermissionLoader(pool)
	mode, found, err := loader.GetToolPermission(context.Background(), "core-researcher", "core-web-search")
	require.NoError(t, err)

	assert.True(t, found,
		"GetToolPermission must find core-researcher/core-web-search after migration 000118")
	assert.Equal(t, core.SeedPermModeAllow, mode,
		"core-researcher core-web-search permission must be \"allow\"")
}

func TestIntegration_ToolPermission_GetToolPermissionForAnalystSubagent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityToolPermissionMigration)

	loader := core.NewCoreCapabilityToolPermissionLoader(pool)
	mode, found, err := loader.GetToolPermission(context.Background(), "core-analyst", "core-subagent-run")
	require.NoError(t, err)

	assert.True(t, found,
		"GetToolPermission must find core-analyst/core-subagent-run after migration 000118")
	assert.Equal(t, core.SeedPermModeDeny, mode,
		"core-analyst core-subagent-run permission must be \"deny\"")
}
