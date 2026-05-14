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

// Integration tests for CoreCapabilityInteractionModeLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000122 creates capability_interaction_mode table + 9 rows

const capabilityInteractionModeMigration = "000122_seed_capability_interaction_modes.up.sql"
const capabilityInteractionModeMigrationDown = "000122_seed_capability_interaction_modes.down.sql"

func TestIntegration_CapabilityInteractionMode_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityInteractionModeMigration)

	loader := core.NewCoreCapabilityInteractionModeLoader(pool)
	got, err := loader.LoadCapabilityInteractionModes(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedInteractionModeCount, len(got),
		"DB row count must match SeedInteractionModeCount (9) after migration 000122")
}

func TestIntegration_CapabilityInteractionMode_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityInteractionModeLoader(pool)
	got, err := loader.LoadCapabilityInteractionModes(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_CapabilityInteractionMode_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityInteractionModeMigration)
	// Apply 000122 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityInteractionModeMigration)

	loader := core.NewCoreCapabilityInteractionModeLoader(pool)
	got, err := loader.LoadCapabilityInteractionModes(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedInteractionModeCount, len(got),
		"double-apply of migration 000122 must still produce exactly 9 rows (idempotent)")
}

func TestIntegration_CapabilityInteractionMode_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityInteractionModeMigration)

	loader := core.NewCoreCapabilityInteractionModeLoader(pool)
	before, err := loader.LoadCapabilityInteractionModes(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced interaction mode rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityInteractionModeMigrationDown)

	after, err := loader.LoadCapabilityInteractionModes(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 interaction mode rows and drop the table")
}

func TestIntegration_CapabilityInteractionMode_GetPrimaryModeForResearcher(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityInteractionModeMigration)

	loader := core.NewCoreCapabilityInteractionModeLoader(pool)
	value, found, err := loader.GetInteractionModeValue(context.Background(), "core-researcher", core.SeedModeKeyPrimary)
	require.NoError(t, err)
	require.True(t, found, "primary_mode for core-researcher must be present after migration")
	assert.Equal(t, core.SeedPrimaryModeSingleTurn, value,
		"core-researcher primary_mode must be single_turn")
}

func TestIntegration_CapabilityInteractionMode_GetProactiveQForAnalyst(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityInteractionModeMigration)

	loader := core.NewCoreCapabilityInteractionModeLoader(pool)
	value, found, err := loader.GetInteractionModeValue(context.Background(), "core-analyst", core.SeedModeKeyProactiveQ)
	require.NoError(t, err)
	require.True(t, found, "proactive_questions for core-analyst must be present after migration")
	assert.Equal(t, core.SeedProactiveQHigh, value,
		"core-analyst proactive_questions must be high")
}
