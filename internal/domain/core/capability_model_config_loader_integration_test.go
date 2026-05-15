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

// Integration tests for CoreCapabilityModelConfigLoader against a real Postgres
// (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000120 creates capability_model_config table + 9 rows

const capabilityModelConfigMigration = "000120_seed_capability_model_configs.up.sql"
const capabilityModelConfigMigrationDown = "000120_seed_capability_model_configs.down.sql"

func TestIntegration_CapabilityModelConfig_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityModelConfigMigration)

	loader := core.NewCoreCapabilityModelConfigLoader(pool)
	got, err := loader.LoadCapabilityModelConfigs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedModelConfigCount, len(got),
		"DB row count must match SeedModelConfigCount (9) after migration 000120")
}

func TestIntegration_CapabilityModelConfig_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityModelConfigLoader(pool)
	got, err := loader.LoadCapabilityModelConfigs(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_CapabilityModelConfig_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityModelConfigMigration)
	// Apply 000120 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityModelConfigMigration)

	loader := core.NewCoreCapabilityModelConfigLoader(pool)
	got, err := loader.LoadCapabilityModelConfigs(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedModelConfigCount, len(got),
		"double-apply of migration 000120 must still produce exactly 9 rows (idempotent)")
}

func TestIntegration_CapabilityModelConfig_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityModelConfigMigration)

	loader := core.NewCoreCapabilityModelConfigLoader(pool)
	before, err := loader.LoadCapabilityModelConfigs(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced model config rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityModelConfigMigrationDown)

	after, err := loader.LoadCapabilityModelConfigs(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 model config rows and drop the table")
}

func TestIntegration_CapabilityModelConfig_GetModelForResearcher(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityModelConfigMigration)

	loader := core.NewCoreCapabilityModelConfigLoader(pool)
	value, found, err := loader.GetModelConfigValue(context.Background(), "core-researcher", core.SeedModelConfigKeyModel)
	require.NoError(t, err)
	require.True(t, found, "default_model config for core-researcher must be present after migration")
	assert.Equal(t, core.SeedResearcherModel, value,
		"core-researcher default_model must be claude-haiku-4-5-20251001")
}

func TestIntegration_CapabilityModelConfig_GetTemperatureForAnalyst(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityModelConfigMigration)

	loader := core.NewCoreCapabilityModelConfigLoader(pool)
	value, found, err := loader.GetModelConfigValue(context.Background(), "core-analyst", core.SeedModelConfigKeyTemperature)
	require.NoError(t, err)
	require.True(t, found, "temperature config for core-analyst must be present after migration")
	assert.Equal(t, "0.2", value,
		"core-analyst temperature must be 0.2 (very low for deterministic, reproducible analysis)")
}
