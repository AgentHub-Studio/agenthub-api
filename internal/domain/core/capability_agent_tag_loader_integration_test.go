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

// Integration tests for CoreCapabilityAgentTagLoader against a real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000109 creates capability_agent_tag table + 12 rows
//
// The ah_core.capability_agent_tag table is created by migration 000109
// itself (no prior migration defines it).

const capabilityAgentTagMigration = "000109_seed_capability_agent_tags.up.sql"
const capabilityAgentTagMigrationDown = "000109_seed_capability_agent_tags.down.sql"

func TestIntegration_Capability_LoadAgentTags_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentTagMigration)

	loader := core.NewCoreCapabilityAgentTagLoader(pool)
	got, err := loader.LoadCapabilityAgentTags(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentTagCount, len(got),
		"DB row count must match SeedCapabilityAgentTagCount (12) after migration 000109")
}

func TestIntegration_Capability_AgentTagMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentTagMigration)
	// Apply 000109 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityAgentTagMigration)

	loader := core.NewCoreCapabilityAgentTagLoader(pool)
	got, err := loader.LoadCapabilityAgentTags(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentTagCount, len(got),
		"double-apply of migration 000109 must still produce exactly 12 agent tag rows (idempotent)")
}

func TestIntegration_Capability_AgentTagDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentTagMigration)

	loader := core.NewCoreCapabilityAgentTagLoader(pool)
	before, err := loader.LoadCapabilityAgentTags(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced agent tag rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityAgentTagMigrationDown)

	after, err := loader.LoadCapabilityAgentTags(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 12 agent tag rows and drop the table")
}

func TestIntegration_Capability_AgentTagNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityAgentTagLoader(pool)
	got, err := loader.LoadCapabilityAgentTags(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadTagsForResearcher_ReturnsFourStrings(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentTagMigration)

	loader := core.NewCoreCapabilityAgentTagLoader(pool)
	got, err := loader.LoadTagsForAgent(context.Background(), "core-researcher")
	require.NoError(t, err)

	require.Len(t, got, core.SeedResearcherTagCount,
		"LoadTagsForAgent('core-researcher') must return exactly SeedResearcherTagCount (4) tag strings")

	// Verify the returned strings match the seeded researcher tags.
	for _, tag := range got {
		assert.Contains(t, core.SeedResearcherTags, tag,
			"returned tag %q must be in SeedResearcherTags", tag)
	}
}
