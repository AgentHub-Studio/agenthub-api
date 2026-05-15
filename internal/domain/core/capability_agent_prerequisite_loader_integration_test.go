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

// Integration tests for CoreCapabilityAgentPrerequisiteLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000110 creates capability_agent_prerequisite table + 6 rows
//
// The ah_core.capability_agent_prerequisite table is created by migration
// 000110 itself (no prior migration defines it).

const capabilityAgentPrerequisiteMigration = "000110_seed_capability_agent_prerequisites.up.sql"
const capabilityAgentPrerequisiteMigrationDown = "000110_seed_capability_agent_prerequisites.down.sql"

func TestIntegration_Capability_LoadAgentPrerequisites_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentPrerequisiteMigration)

	loader := core.NewCoreCapabilityAgentPrerequisiteLoader(pool)
	got, err := loader.LoadCapabilityAgentPrerequisites(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentPrerequisiteCount, len(got),
		"DB row count must match SeedCapabilityAgentPrerequisiteCount (6) after migration 000110")
}

func TestIntegration_Capability_AgentPrerequisiteMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentPrerequisiteMigration)
	// Apply 000110 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityAgentPrerequisiteMigration)

	loader := core.NewCoreCapabilityAgentPrerequisiteLoader(pool)
	got, err := loader.LoadCapabilityAgentPrerequisites(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentPrerequisiteCount, len(got),
		"double-apply of migration 000110 must still produce exactly 6 agent prerequisite rows (idempotent)")
}

func TestIntegration_Capability_AgentPrerequisiteDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentPrerequisiteMigration)

	loader := core.NewCoreCapabilityAgentPrerequisiteLoader(pool)
	before, err := loader.LoadCapabilityAgentPrerequisites(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced agent prerequisite rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityAgentPrerequisiteMigrationDown)

	after, err := loader.LoadCapabilityAgentPrerequisites(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 6 agent prerequisite rows and drop the table")
}

func TestIntegration_Capability_AgentPrerequisiteNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityAgentPrerequisiteLoader(pool)
	got, err := loader.LoadCapabilityAgentPrerequisites(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadPrerequisitesForResearcher_ReturnsTwo(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentPrerequisiteMigration)

	loader := core.NewCoreCapabilityAgentPrerequisiteLoader(pool)
	got, err := loader.LoadPrerequisitesForAgent(context.Background(), "core-researcher")
	require.NoError(t, err)

	require.Len(t, got, core.SeedResearcherPrerequisiteCount,
		"LoadPrerequisitesForAgent('core-researcher') must return exactly SeedResearcherPrerequisiteCount (2) prerequisite rows")

	// Verify each returned prerequisite belongs to the researcher.
	for _, p := range got {
		assert.Equal(t, "core-researcher", p.AgentSlug,
			"all returned prerequisites must belong to core-researcher")
		assert.True(t, p.IsRequired,
			"all seeded researcher prerequisites must be required")
	}
}
