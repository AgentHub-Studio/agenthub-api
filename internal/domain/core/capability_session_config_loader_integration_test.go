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

// Integration tests for CoreCapabilitySessionConfigLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000111 creates capability_session_config table + 9 rows
//
// The ah_core.capability_session_config table is created by migration
// 000111 itself (no prior migration defines it).

const capabilitySessionConfigMigration = "000111_seed_capability_session_configs.up.sql"
const capabilitySessionConfigMigrationDown = "000111_seed_capability_session_configs.down.sql"

func TestIntegration_Capability_LoadSessionConfigs_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySessionConfigMigration)

	loader := core.NewCoreCapabilitySessionConfigLoader(pool)
	got, err := loader.LoadCapabilitySessionConfigs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilitySessionConfigCount, len(got),
		"DB row count must match SeedCapabilitySessionConfigCount (9) after migration 000111")
}

func TestIntegration_Capability_SessionConfigMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySessionConfigMigration)
	// Apply 000111 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilitySessionConfigMigration)

	loader := core.NewCoreCapabilitySessionConfigLoader(pool)
	got, err := loader.LoadCapabilitySessionConfigs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilitySessionConfigCount, len(got),
		"double-apply of migration 000111 must still produce exactly 9 session config rows (idempotent)")
}

func TestIntegration_Capability_SessionConfigDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySessionConfigMigration)

	loader := core.NewCoreCapabilitySessionConfigLoader(pool)
	before, err := loader.LoadCapabilitySessionConfigs(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced session config rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilitySessionConfigMigrationDown)

	after, err := loader.LoadCapabilitySessionConfigs(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 session config rows and drop the table")
}

func TestIntegration_Capability_SessionConfigNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilitySessionConfigLoader(pool)
	got, err := loader.LoadCapabilitySessionConfigs(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadSessionConfigsForPlanner_ReturnsThree(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilitySessionConfigMigration)

	loader := core.NewCoreCapabilitySessionConfigLoader(pool)
	got, err := loader.LoadSessionConfigsForAgent(context.Background(), "core-planner")
	require.NoError(t, err)

	require.Len(t, got, core.SeedPlannerSessionConfigCount,
		"LoadSessionConfigsForAgent('core-planner') must return exactly SeedPlannerSessionConfigCount (3) session config rows")

	// Verify each returned config belongs to the planner.
	for _, c := range got {
		assert.Equal(t, "core-planner", c.AgentSlug,
			"all returned session configs must belong to core-planner")
		assert.NotEmpty(t, c.ConfigKey, "config_key must not be empty")
		assert.NotEmpty(t, c.ConfigValue, "config_value must not be empty")
	}
}
