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

// Integration tests for CoreCapabilityAgentToolConfigLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000101 creates capability_agent_tool_config table + 7 rows
//
// The ah_core.capability_agent_tool_config table is created by migration 000101
// itself (no prior migration defines it).

const capabilityAgentToolConfigMigration = "000101_seed_capability_agent_tool_configs.up.sql"
const capabilityAgentToolConfigMigrationDown = "000101_seed_capability_agent_tool_configs.down.sql"

func TestIntegration_Capability_LoadAgentToolConfigs_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentToolConfigMigration)

	loader := core.NewCoreCapabilityAgentToolConfigLoader(pool)
	got, err := loader.LoadCapabilityAgentToolConfigs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentToolConfigCount, len(got),
		"DB row count must match SeedCapabilityAgentToolConfigCount (7) after migration 000101")
}

func TestIntegration_Capability_AgentToolConfigMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentToolConfigMigration)
	// Apply 000101 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityAgentToolConfigMigration)

	loader := core.NewCoreCapabilityAgentToolConfigLoader(pool)
	got, err := loader.LoadCapabilityAgentToolConfigs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentToolConfigCount, len(got),
		"double-apply of migration 000101 must still produce exactly 7 capability agent tool config rows (idempotent)")
}

func TestIntegration_Capability_AgentToolConfigDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentToolConfigMigration)

	loader := core.NewCoreCapabilityAgentToolConfigLoader(pool)
	before, err := loader.LoadCapabilityAgentToolConfigs(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability agent tool config rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityAgentToolConfigMigrationDown)

	after, err := loader.LoadCapabilityAgentToolConfigs(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 7 capability agent tool config rows and drop the table")
}

func TestIntegration_Capability_AgentToolConfigNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityAgentToolConfigLoader(pool)
	got, err := loader.LoadCapabilityAgentToolConfigs(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadToolConfigsForResearcher(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentToolConfigMigration)

	loader := core.NewCoreCapabilityAgentToolConfigLoader(pool)
	got, err := loader.LoadToolConfigsForAgent(context.Background(), "core-researcher")
	require.NoError(t, err)

	assert.Equal(t, core.SeedResearcherToolConfigCount, len(got),
		"LoadToolConfigsForAgent(core-researcher) must return exactly %d rows after migration 000101",
		core.SeedResearcherToolConfigCount)

	// Verify all returned rows belong to core-researcher and have expected param keys.
	paramKeys := map[string]string{}
	for _, c := range got {
		assert.Equal(t, "core-researcher", c.AgentSlug,
			"all rows returned by LoadToolConfigsForAgent(core-researcher) must have agent_slug=core-researcher")
		assert.NotEmpty(t, c.ToolSlug,
			"tool_slug must be non-empty for researcher tool config row")
		assert.NotEmpty(t, c.ParamKey,
			"param_key must be non-empty for researcher tool config row")
		assert.NotEmpty(t, c.ParamValue,
			"param_value must be non-empty for researcher tool config row")
		paramKeys[c.ParamKey] = c.ParamValue
	}
	assert.Contains(t, paramKeys, core.SeedToolConfigMaxResults,
		"researcher tool configs must contain a max_results override")
	assert.Contains(t, paramKeys, core.SeedToolConfigTimeoutSeconds,
		"researcher tool configs must contain a timeout_seconds override")
	assert.Contains(t, paramKeys, core.SeedToolConfigSimilarityThreshold,
		"researcher tool configs must contain a similarity_threshold override")
}
