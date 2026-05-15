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

// Integration tests for CoreCapabilityAgentMemoryConfigLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000102 creates capability_agent_memory_config table + 9 rows
//
// The ah_core.capability_agent_memory_config table is created by migration 000102
// itself (no prior migration defines it).

const capabilityAgentMemoryConfigMigration = "000102_seed_capability_agent_memory_configs.up.sql"
const capabilityAgentMemoryConfigMigrationDown = "000102_seed_capability_agent_memory_configs.down.sql"

func TestIntegration_Capability_LoadAgentMemoryConfigs_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentMemoryConfigMigration)

	loader := core.NewCoreCapabilityAgentMemoryConfigLoader(pool)
	got, err := loader.LoadCapabilityAgentMemoryConfigs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentMemoryConfigCount, len(got),
		"DB row count must match SeedCapabilityAgentMemoryConfigCount (9) after migration 000102")
}

func TestIntegration_Capability_AgentMemoryConfigMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentMemoryConfigMigration)
	// Apply 000102 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityAgentMemoryConfigMigration)

	loader := core.NewCoreCapabilityAgentMemoryConfigLoader(pool)
	got, err := loader.LoadCapabilityAgentMemoryConfigs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentMemoryConfigCount, len(got),
		"double-apply of migration 000102 must still produce exactly 9 capability agent memory config rows (idempotent)")
}

func TestIntegration_Capability_AgentMemoryConfigDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentMemoryConfigMigration)

	loader := core.NewCoreCapabilityAgentMemoryConfigLoader(pool)
	before, err := loader.LoadCapabilityAgentMemoryConfigs(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability agent memory config rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityAgentMemoryConfigMigrationDown)

	after, err := loader.LoadCapabilityAgentMemoryConfigs(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 capability agent memory config rows and drop the table")
}

func TestIntegration_Capability_AgentMemoryConfigNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityAgentMemoryConfigLoader(pool)
	got, err := loader.LoadCapabilityAgentMemoryConfigs(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadMemoryConfigsForAnalyst(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentMemoryConfigMigration)

	loader := core.NewCoreCapabilityAgentMemoryConfigLoader(pool)
	got, err := loader.LoadMemoryConfigsForAgent(context.Background(), "core-analyst")
	require.NoError(t, err)

	assert.Equal(t, core.SeedAnalystMemoryConfigCount, len(got),
		"LoadMemoryConfigsForAgent(core-analyst) must return exactly %d rows after migration 000102",
		core.SeedAnalystMemoryConfigCount)

	// Verify all returned rows belong to core-analyst and have expected config keys.
	configMap := map[string]string{}
	for _, c := range got {
		assert.Equal(t, "core-analyst", c.AgentSlug,
			"all rows returned by LoadMemoryConfigsForAgent(core-analyst) must have agent_slug=core-analyst")
		assert.NotEmpty(t, c.ConfigKey,
			"config_key must be non-empty for analyst memory config row")
		assert.NotEmpty(t, c.ConfigValue,
			"config_value must be non-empty for analyst memory config row")
		configMap[c.ConfigKey] = c.ConfigValue
	}

	assert.Contains(t, configMap, core.SeedMemoryConfigMaxContextTokens,
		"analyst memory configs must contain a max_context_tokens config row")
	assert.Contains(t, configMap, core.SeedMemoryConfigSummaryStrategy,
		"analyst memory configs must contain a summary_strategy config row")
	assert.Contains(t, configMap, core.SeedMemoryConfigPersistenceScope,
		"analyst memory configs must contain a persistence_scope config row")

	// Verify analyst-specific values.
	assert.Equal(t, "150000", configMap[core.SeedMemoryConfigMaxContextTokens],
		"analyst max_context_tokens must be 150000 (largest context window for document processing)")
	assert.Equal(t, core.SeedSummaryStrategySnapshot, configMap[core.SeedMemoryConfigSummaryStrategy],
		"analyst summary_strategy must be snapshot (checkpoint-based summarisation)")
	assert.Equal(t, core.SeedPersistenceScopeGlobal, configMap[core.SeedMemoryConfigPersistenceScope],
		"analyst persistence_scope must be global (cross-session analysis retention)")
}
