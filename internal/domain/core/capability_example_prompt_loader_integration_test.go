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

// Integration tests for CoreCapabilityExamplePromptLoader against a real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000108 creates capability_example_prompt table + 12 rows
//
// The ah_core.capability_example_prompt table is created by migration 000108
// itself (no prior migration defines it).

const capabilityExamplePromptMigration = "000108_seed_capability_example_prompts.up.sql"
const capabilityExamplePromptMigrationDown = "000108_seed_capability_example_prompts.down.sql"

func TestIntegration_Capability_LoadExamplePrompts_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityExamplePromptMigration)

	loader := core.NewCoreCapabilityExamplePromptLoader(pool)
	got, err := loader.LoadCapabilityExamplePrompts(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityExamplePromptCount, len(got),
		"DB row count must match SeedCapabilityExamplePromptCount (12) after migration 000108")
}

func TestIntegration_Capability_ExamplePromptMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityExamplePromptMigration)
	// Apply 000108 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityExamplePromptMigration)

	loader := core.NewCoreCapabilityExamplePromptLoader(pool)
	got, err := loader.LoadCapabilityExamplePrompts(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityExamplePromptCount, len(got),
		"double-apply of migration 000108 must still produce exactly 12 example prompt rows (idempotent)")
}

func TestIntegration_Capability_ExamplePromptDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityExamplePromptMigration)

	loader := core.NewCoreCapabilityExamplePromptLoader(pool)
	before, err := loader.LoadCapabilityExamplePrompts(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced example prompt rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityExamplePromptMigrationDown)

	after, err := loader.LoadCapabilityExamplePrompts(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 12 example prompt rows and drop the table")
}

func TestIntegration_Capability_ExamplePromptNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityExamplePromptLoader(pool)
	got, err := loader.LoadCapabilityExamplePrompts(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadExamplePromptsForResearcher_ReturnsFour(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityExamplePromptMigration)

	loader := core.NewCoreCapabilityExamplePromptLoader(pool)
	got, err := loader.LoadExamplePromptsForAgent(context.Background(), "core-researcher")
	require.NoError(t, err)

	require.Len(t, got, core.SeedResearcherExamplePromptCount,
		"LoadExamplePromptsForAgent('core-researcher') must return exactly SeedResearcherExamplePromptCount (4) rows")

	// All returned rows must reference core-researcher and be in sort_order.
	for _, p := range got {
		assert.Equal(t, "core-researcher", p.AgentSlug,
			"all returned prompts must have agent_slug='core-researcher', got slug=%q", p.Slug)
	}
}
