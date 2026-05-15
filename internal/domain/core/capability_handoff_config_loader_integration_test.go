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

// Integration tests for CoreCapabilityHandoffConfigLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000123 creates capability_handoff_config table + 9 rows

const capabilityHandoffConfigMigration = "000123_seed_capability_handoff_configs.up.sql"
const capabilityHandoffConfigMigrationDown = "000123_seed_capability_handoff_configs.down.sql"

func TestIntegration_CapabilityHandoffConfig_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityHandoffConfigMigration)

	loader := core.NewCoreCapabilityHandoffConfigLoader(pool)
	got, err := loader.LoadCapabilityHandoffConfigs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedHandoffConfigCount, len(got),
		"DB row count must match SeedHandoffConfigCount (9) after migration 000123")
}

func TestIntegration_CapabilityHandoffConfig_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityHandoffConfigLoader(pool)
	got, err := loader.LoadCapabilityHandoffConfigs(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_CapabilityHandoffConfig_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityHandoffConfigMigration)
	// Apply 000123 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityHandoffConfigMigration)

	loader := core.NewCoreCapabilityHandoffConfigLoader(pool)
	got, err := loader.LoadCapabilityHandoffConfigs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedHandoffConfigCount, len(got),
		"double-apply of migration 000123 must still produce exactly 9 rows (idempotent)")
}

func TestIntegration_CapabilityHandoffConfig_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityHandoffConfigMigration)

	loader := core.NewCoreCapabilityHandoffConfigLoader(pool)
	before, err := loader.LoadCapabilityHandoffConfigs(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced handoff config rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityHandoffConfigMigrationDown)

	after, err := loader.LoadCapabilityHandoffConfigs(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 handoff config rows and drop the table")
}

func TestIntegration_CapabilityHandoffConfig_LoadHumanEscalationsReturnsThree(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityHandoffConfigMigration)

	loader := core.NewCoreCapabilityHandoffConfigLoader(pool)
	escalations, err := loader.LoadHumanEscalations(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedHumanEscalationCount, len(escalations),
		"LoadHumanEscalations must return exactly 3 rows (one per agent)")
}

func TestIntegration_CapabilityHandoffConfig_AllHumanEscalationsHaveEmptyTargetSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityHandoffConfigMigration)

	loader := core.NewCoreCapabilityHandoffConfigLoader(pool)
	escalations, err := loader.LoadHumanEscalations(context.Background())
	require.NoError(t, err)
	require.Equal(t, core.SeedHumanEscalationCount, len(escalations),
		"precondition: 3 human escalation rows must exist")

	for _, e := range escalations {
		assert.Equal(t, "", e.TargetAgentSlug,
			"human escalation row for agent %q must have empty target_agent_slug", e.AgentSlug)
		assert.True(t, e.IsAutomatic,
			"human escalation row for agent %q must have is_automatic=true", e.AgentSlug)
	}
}
