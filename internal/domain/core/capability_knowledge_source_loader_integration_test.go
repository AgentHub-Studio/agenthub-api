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

// Integration tests for CoreCapabilityKnowledgeSourceLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000117 creates capability_knowledge_source table + 9 rows
//
// The ah_core.capability_knowledge_source table is created by migration 000117
// itself (no prior migration defines it).

const capabilityKnowledgeSourceMigration = "000117_seed_capability_knowledge_sources.up.sql"
const capabilityKnowledgeSourceMigrationDown = "000117_seed_capability_knowledge_sources.down.sql"

func TestIntegration_KnowledgeSource_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityKnowledgeSourceMigration)

	loader := core.NewCoreCapabilityKnowledgeSourceLoader(pool)
	got, err := loader.LoadCapabilityKnowledgeSources(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedKnowledgeSourceCount, len(got),
		"DB row count must match SeedKnowledgeSourceCount (9) after migration 000117")
}

func TestIntegration_KnowledgeSource_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityKnowledgeSourceLoader(pool)
	got, err := loader.LoadCapabilityKnowledgeSources(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_KnowledgeSource_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityKnowledgeSourceMigration)
	// Apply 000117 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityKnowledgeSourceMigration)

	loader := core.NewCoreCapabilityKnowledgeSourceLoader(pool)
	got, err := loader.LoadCapabilityKnowledgeSources(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedKnowledgeSourceCount, len(got),
		"double-apply of migration 000117 must still produce exactly 9 knowledge source rows (idempotent)")
}

func TestIntegration_KnowledgeSource_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityKnowledgeSourceMigration)

	loader := core.NewCoreCapabilityKnowledgeSourceLoader(pool)
	before, err := loader.LoadCapabilityKnowledgeSources(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed must produce knowledge source rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityKnowledgeSourceMigrationDown)

	after, err := loader.LoadCapabilityKnowledgeSources(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 knowledge source rows and drop the table")
}

func TestIntegration_KnowledgeSource_GetPrimaryForResearcherIsWebSearch(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityKnowledgeSourceMigration)

	loader := core.NewCoreCapabilityKnowledgeSourceLoader(pool)
	src, err := loader.GetPrimaryKnowledgeSource(context.Background(), "core-researcher")
	require.NoError(t, err)

	require.NotNil(t, src,
		"GetPrimaryKnowledgeSource must return non-nil for core-researcher after migration 000117")
	assert.Equal(t, "core-researcher", src.AgentSlug,
		"returned source must belong to core-researcher")
	assert.Equal(t, core.SeedSourceKeyPrimary, src.SourceKey,
		"returned source must have source_key 'primary'")
	assert.Equal(t, 1, src.Priority,
		"primary knowledge source must have priority=1")
	assert.Equal(t, core.SeedSourceTypeWebSearch, src.SourceType,
		"core-researcher primary source type must be web_search")
	assert.Equal(t, "web_search_index", src.SourceRef,
		"core-researcher primary source ref must be web_search_index")
	assert.True(t, src.IsActive,
		"primary knowledge source must be active by default")
}

func TestIntegration_KnowledgeSource_LoadForAgentReturnsPriorityOrder(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityKnowledgeSourceMigration)

	loader := core.NewCoreCapabilityKnowledgeSourceLoader(pool)

	for _, agentSlug := range []string{"core-researcher", "core-analyst", "core-planner"} {
		sources, err := loader.LoadKnowledgeSourcesForAgent(context.Background(), agentSlug)
		require.NoError(t, err, "LoadKnowledgeSourcesForAgent must not error for %s", agentSlug)
		require.Len(t, sources, 3,
			"each agent must have exactly 3 knowledge source rows, got %d for %s", len(sources), agentSlug)

		// Verify ascending priority order: 1, 2, 3.
		for i := 1; i < len(sources); i++ {
			assert.Less(t, sources[i-1].Priority, sources[i].Priority,
				"%s: source at position %d (priority=%d) must have lower priority number than position %d (priority=%d)",
				agentSlug, i-1, sources[i-1].Priority, i, sources[i].Priority)
		}

		// Verify source keys match expected slots.
		assert.Equal(t, core.SeedSourceKeyPrimary, sources[0].SourceKey,
			"%s: first source must be 'primary'", agentSlug)
		assert.Equal(t, core.SeedSourceKeySecondary, sources[1].SourceKey,
			"%s: second source must be 'secondary'", agentSlug)
		assert.Equal(t, core.SeedSourceKeyFallback, sources[2].SourceKey,
			"%s: third source must be 'fallback'", agentSlug)
	}
}
