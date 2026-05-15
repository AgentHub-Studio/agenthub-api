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

// Integration tests for CoreCapabilityFeatureFlagLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000103 creates capability_feature_flag table + 6 rows
//
// The ah_core.capability_feature_flag table is created by migration 000103
// itself (no prior migration defines it).

const capabilityFeatureFlagMigration = "000103_seed_capability_feature_flags.up.sql"
const capabilityFeatureFlagMigrationDown = "000103_seed_capability_feature_flags.down.sql"

func TestIntegration_Capability_LoadFeatureFlags_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityFeatureFlagMigration)

	loader := core.NewCoreCapabilityFeatureFlagLoader(pool)
	got, err := loader.LoadCapabilityFeatureFlags(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityFeatureFlagCount, len(got),
		"DB row count must match SeedCapabilityFeatureFlagCount (6) after migration 000103")
}

func TestIntegration_Capability_FeatureFlagMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityFeatureFlagMigration)
	// Apply 000103 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityFeatureFlagMigration)

	loader := core.NewCoreCapabilityFeatureFlagLoader(pool)
	got, err := loader.LoadCapabilityFeatureFlags(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityFeatureFlagCount, len(got),
		"double-apply of migration 000103 must still produce exactly 6 capability feature flag rows (idempotent)")
}

func TestIntegration_Capability_FeatureFlagDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityFeatureFlagMigration)

	loader := core.NewCoreCapabilityFeatureFlagLoader(pool)
	before, err := loader.LoadCapabilityFeatureFlags(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability feature flag rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityFeatureFlagMigrationDown)

	after, err := loader.LoadCapabilityFeatureFlags(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 6 capability feature flag rows and drop the table")
}

func TestIntegration_Capability_FeatureFlagNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityFeatureFlagLoader(pool)
	got, err := loader.LoadCapabilityFeatureFlags(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_LoadEnabledFeatureFlags_ReturnsFive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityFeatureFlagMigration)

	loader := core.NewCoreCapabilityFeatureFlagLoader(pool)
	got, err := loader.LoadEnabledCapabilityFeatureFlags(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 5, len(got),
		"LoadEnabledCapabilityFeatureFlags must return exactly 5 rows — subagent-delegation is default_enabled=FALSE")

	// Verify all returned flags have default_enabled=TRUE.
	for _, f := range got {
		assert.True(t, f.DefaultEnabled,
			"all rows returned by LoadEnabledCapabilityFeatureFlags must have default_enabled=TRUE, got slug=%q", f.Slug)
	}

	// Verify the disabled flag is absent.
	for _, f := range got {
		assert.NotEqual(t, core.SeedSubagentDelegationFeatureFlagSlug, f.Slug,
			"LoadEnabledCapabilityFeatureFlags must not include capability-subagent-delegation (default_enabled=FALSE)")
	}
}
