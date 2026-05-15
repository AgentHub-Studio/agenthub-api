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

// Integration tests for CoreCapabilityAuditLogConfigLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000113 creates capability_audit_log_config table + 9 rows
//
// The ah_core.capability_audit_log_config table is created by migration
// 000113 itself (no prior migration defines it).

const capabilityAuditLogConfigMigration = "000113_seed_capability_audit_log_configs.up.sql"
const capabilityAuditLogConfigMigrationDown = "000113_seed_capability_audit_log_configs.down.sql"

func TestIntegration_Capability_LoadAuditLogConfigs_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAuditLogConfigMigration)

	loader := core.NewCoreCapabilityAuditLogConfigLoader(pool)
	got, err := loader.LoadCapabilityAuditLogConfigs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAuditLogConfigCount, len(got),
		"DB row count must match SeedCapabilityAuditLogConfigCount (9) after migration 000113")
}

func TestIntegration_Capability_AuditLogConfigMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAuditLogConfigMigration)
	// Apply 000113 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityAuditLogConfigMigration)

	loader := core.NewCoreCapabilityAuditLogConfigLoader(pool)
	got, err := loader.LoadCapabilityAuditLogConfigs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAuditLogConfigCount, len(got),
		"double-apply of migration 000113 must still produce exactly 9 audit log config rows (idempotent)")
}

func TestIntegration_Capability_AuditLogConfigDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAuditLogConfigMigration)

	loader := core.NewCoreCapabilityAuditLogConfigLoader(pool)
	before, err := loader.LoadCapabilityAuditLogConfigs(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced audit log config rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityAuditLogConfigMigrationDown)

	after, err := loader.LoadCapabilityAuditLogConfigs(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 audit log config rows and drop the table")
}

func TestIntegration_Capability_AuditLogConfigNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityAuditLogConfigLoader(pool)
	got, err := loader.LoadCapabilityAuditLogConfigs(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadAuditLogConfigsForPlanner_ReturnsThree(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAuditLogConfigMigration)

	loader := core.NewCoreCapabilityAuditLogConfigLoader(pool)
	got, err := loader.LoadAuditLogConfigsForAgent(context.Background(), "core-planner")
	require.NoError(t, err)

	require.Len(t, got, core.SeedPlannerAuditLogConfigCount,
		"LoadAuditLogConfigsForAgent('core-planner') must return exactly SeedPlannerAuditLogConfigCount (3) audit log config rows")

	// Verify each returned config belongs to the planner and is well-formed.
	for _, c := range got {
		assert.Equal(t, "core-planner", c.AgentSlug,
			"all returned audit log configs must belong to core-planner")
		assert.NotEmpty(t, c.ConfigKey, "config_key must not be empty")
		assert.NotEmpty(t, c.ConfigValue, "config_value must not be empty")
	}
}
