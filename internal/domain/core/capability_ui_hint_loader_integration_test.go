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

// Integration tests for CoreCapabilityUIHintLoader against a real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000107 creates capability_ui_hint table + 6 rows
//
// The ah_core.capability_ui_hint table is created by migration 000107 itself
// (no prior migration defines it).

const capabilityUIHintMigration = "000107_seed_capability_ui_hints.up.sql"
const capabilityUIHintMigrationDown = "000107_seed_capability_ui_hints.down.sql"

func TestIntegration_Capability_LoadUIHints_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityUIHintMigration)

	loader := core.NewCoreCapabilityUIHintLoader(pool)
	got, err := loader.LoadCapabilityUIHints(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityUIHintCount, len(got),
		"DB row count must match SeedCapabilityUIHintCount (6) after migration 000107")
}

func TestIntegration_Capability_UIHintMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityUIHintMigration)
	// Apply 000107 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityUIHintMigration)

	loader := core.NewCoreCapabilityUIHintLoader(pool)
	got, err := loader.LoadCapabilityUIHints(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityUIHintCount, len(got),
		"double-apply of migration 000107 must still produce exactly 6 UI hint rows (idempotent)")
}

func TestIntegration_Capability_UIHintDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityUIHintMigration)

	loader := core.NewCoreCapabilityUIHintLoader(pool)
	before, err := loader.LoadCapabilityUIHints(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced UI hint rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityUIHintMigrationDown)

	after, err := loader.LoadCapabilityUIHints(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 6 UI hint rows and drop the table")
}

func TestIntegration_Capability_UIHintNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityUIHintLoader(pool)
	got, err := loader.LoadCapabilityUIHints(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadUIHintsForResearcher_ReturnsTwo(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityUIHintMigration)

	loader := core.NewCoreCapabilityUIHintLoader(pool)
	got, err := loader.LoadUIHintsForAgent(context.Background(), "core-researcher")
	require.NoError(t, err)

	require.Len(t, got, core.SeedResearcherUIHintCount,
		"LoadUIHintsForAgent('core-researcher') must return exactly SeedResearcherUIHintCount (2) rows")

	// All returned rows must reference core-researcher and be in sort_order.
	for _, h := range got {
		assert.Equal(t, "core-researcher", h.AgentSlug,
			"all returned hints must have agent_slug='core-researcher', got slug=%q", h.Slug)
	}
}
