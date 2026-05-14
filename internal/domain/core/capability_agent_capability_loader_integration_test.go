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

// Integration tests for CoreCapabilityAgentCapabilityLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000119 creates capability_agent_capability table + 12 rows
//
// The ah_core.capability_agent_capability table is created by migration 000119
// itself (no prior migration defines it).

const agentCapabilityMigration = "000119_seed_capability_agent_capabilities.up.sql"
const agentCapabilityMigrationDown = "000119_seed_capability_agent_capabilities.down.sql"

func TestIntegration_AgentCapability_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentCapabilityMigration)

	loader := core.NewCoreCapabilityAgentCapabilityLoader(pool)
	got, err := loader.LoadCapabilityAgentCapabilities(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedAgentCapabilityCount, len(got),
		"DB row count must match SeedAgentCapabilityCount (12) after migration 000119")
}

func TestIntegration_AgentCapability_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityAgentCapabilityLoader(pool)
	got, err := loader.LoadCapabilityAgentCapabilities(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_AgentCapability_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentCapabilityMigration)
	// Apply 000119 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, agentCapabilityMigration)

	loader := core.NewCoreCapabilityAgentCapabilityLoader(pool)
	got, err := loader.LoadCapabilityAgentCapabilities(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedAgentCapabilityCount, len(got),
		"double-apply of migration 000119 must still produce exactly 12 capability rows (idempotent)")
}

func TestIntegration_AgentCapability_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentCapabilityMigration)

	loader := core.NewCoreCapabilityAgentCapabilityLoader(pool)
	before, err := loader.LoadCapabilityAgentCapabilities(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed must produce capability rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, agentCapabilityMigrationDown)

	after, err := loader.LoadCapabilityAgentCapabilities(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 12 capability rows and drop the table")
}

func TestIntegration_AgentCapability_ResearcherHasTwoSupportedCapabilities(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentCapabilityMigration)

	loader := core.NewCoreCapabilityAgentCapabilityLoader(pool)
	got, err := loader.LoadSupportedCapabilitiesForAgent(context.Background(), "core-researcher")
	require.NoError(t, err)

	assert.Equal(t, core.SeedResearcherSupportedCount, len(got),
		"core-researcher must have exactly %d supported capabilities after migration 000119",
		core.SeedResearcherSupportedCount)

	// Verify the two supported capabilities are web_search and document_analysis.
	keys := make(map[string]bool, len(got))
	for _, c := range got {
		assert.True(t, c.IsSupported,
			"LoadSupportedCapabilitiesForAgent must only return is_supported=true rows")
		keys[c.CapabilityKey] = true
	}
	assert.True(t, keys[core.SeedCapKeyWebSearch],
		"core-researcher must support web_search")
	assert.True(t, keys[core.SeedCapKeyDocAnalysis],
		"core-researcher must support document_analysis")
}

func TestIntegration_AgentCapability_NoAgentSupportsCodeGeneration(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentCapabilityMigration)

	loader := core.NewCoreCapabilityAgentCapabilityLoader(pool)
	all, err := loader.LoadCapabilityAgentCapabilities(context.Background())
	require.NoError(t, err)

	// Find code_generation rows — all must have is_supported=false.
	codeGenRows := 0
	for _, c := range all {
		if c.CapabilityKey == core.SeedCapKeyCodeGen {
			codeGenRows++
			assert.False(t, c.IsSupported,
				"code_generation must be is_supported=false for agent %q", c.AgentSlug)
		}
	}
	// All 3 agents have a code_generation row.
	assert.Equal(t, core.SeedAgentCapabilityAgentCount, codeGenRows,
		"every capability agent must have a code_generation row (all unsupported)")
}
