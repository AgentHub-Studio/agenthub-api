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

// Integration tests for CoreCapabilityFallbackBehaviorLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000121 creates capability_fallback_behavior table + 9 rows

const capabilityFallbackBehaviorMigration = "000121_seed_capability_fallback_behaviors.up.sql"
const capabilityFallbackBehaviorMigrationDown = "000121_seed_capability_fallback_behaviors.down.sql"

func TestIntegration_CapabilityFallbackBehavior_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityFallbackBehaviorMigration)

	loader := core.NewCoreCapabilityFallbackBehaviorLoader(pool)
	got, err := loader.LoadCapabilityFallbackBehaviors(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedFallbackBehaviorCount, len(got),
		"DB row count must match SeedFallbackBehaviorCount (9) after migration 000121")
}

func TestIntegration_CapabilityFallbackBehavior_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityFallbackBehaviorLoader(pool)
	got, err := loader.LoadCapabilityFallbackBehaviors(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_CapabilityFallbackBehavior_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityFallbackBehaviorMigration)
	// Apply 000121 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityFallbackBehaviorMigration)

	loader := core.NewCoreCapabilityFallbackBehaviorLoader(pool)
	got, err := loader.LoadCapabilityFallbackBehaviors(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedFallbackBehaviorCount, len(got),
		"double-apply of migration 000121 must still produce exactly 9 rows (idempotent)")
}

func TestIntegration_CapabilityFallbackBehavior_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityFallbackBehaviorMigration)

	loader := core.NewCoreCapabilityFallbackBehaviorLoader(pool)
	before, err := loader.LoadCapabilityFallbackBehaviors(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced fallback behavior rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityFallbackBehaviorMigrationDown)

	after, err := loader.LoadCapabilityFallbackBehaviors(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 fallback behavior rows and drop the table")
}

func TestIntegration_CapabilityFallbackBehavior_GetSearchFailureForResearcher(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityFallbackBehaviorMigration)

	loader := core.NewCoreCapabilityFallbackBehaviorLoader(pool)
	got, err := loader.GetFallbackBehavior(context.Background(), "core-researcher", core.SeedBehaviorKeySearchFailure)
	require.NoError(t, err)
	require.NotNil(t, got, "search_failure behavior for core-researcher must be present after migration")
	assert.Equal(t, core.SeedActionGracefulDegrade, got.Action,
		"core-researcher search_failure action must be graceful_degrade")
	assert.Equal(t, "web_search_tool_unavailable", got.Trigger,
		"core-researcher search_failure trigger must be web_search_tool_unavailable")
	assert.Equal(t, 2, got.RetryCount,
		"core-researcher search_failure must retry 2 times before graceful degradation")
}

func TestIntegration_CapabilityFallbackBehavior_GetDocUnavailableForAnalyst(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityFallbackBehaviorMigration)

	loader := core.NewCoreCapabilityFallbackBehaviorLoader(pool)
	got, err := loader.GetFallbackBehavior(context.Background(), "core-analyst", core.SeedBehaviorKeyDocUnavailable)
	require.NoError(t, err)
	require.NotNil(t, got, "doc_unavailable behavior for core-analyst must be present after migration")
	assert.Equal(t, 0, got.RetryCount,
		"core-analyst doc_unavailable must have retry_count=0 (immediate fallback)")
	assert.Equal(t, core.SeedActionGracefulDegrade, got.Action,
		"core-analyst doc_unavailable action must be graceful_degrade")
}
