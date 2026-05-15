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

// Integration tests for CoreCapabilityAgentConfigPresetLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//   000001 schema → 000098 creates agent_config_preset table + 3 capability rows
//
// The ah_core.agent_config_preset table is created by migration 000098 itself
// (no prior migration defines it).

const capabilityAgentConfigPresetMigration = "000098_seed_capability_agent_config_presets.up.sql"
const capabilityAgentConfigPresetMigrationDown = "000098_seed_capability_agent_config_presets.down.sql"

func TestIntegration_Capability_LoadAgentConfigPresets_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentConfigPresetMigration)

	loader := core.NewCoreCapabilityAgentConfigPresetLoader(pool)
	got, err := loader.LoadCapabilityAgentConfigPresets(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentConfigPresetCount, len(got),
		"DB row count must match SeedCapabilityAgentConfigPresetCount (3) after migration 000098")
}

func TestIntegration_Capability_AgentConfigPresetMigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentConfigPresetMigration)
	// Apply 000098 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityAgentConfigPresetMigration)

	loader := core.NewCoreCapabilityAgentConfigPresetLoader(pool)
	got, err := loader.LoadCapabilityAgentConfigPresets(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityAgentConfigPresetCount, len(got),
		"double-apply of migration 000098 must still produce exactly 3 capability agent config presets (idempotent)")
}

func TestIntegration_Capability_AgentConfigPresetDownMigrationCleansUp(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentConfigPresetMigration)

	loader := core.NewCoreCapabilityAgentConfigPresetLoader(pool)
	before, err := loader.LoadCapabilityAgentConfigPresets(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability rows")

	// Apply down migration — removes the 3 capability rows (does NOT drop table).
	applyMigration(t, pool, migDir, capabilityAgentConfigPresetMigrationDown)

	after, err := loader.LoadCapabilityAgentConfigPresets(context.Background())
	require.NoError(t, err, "non-fatal: empty table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 3 capability agent config presets")
}

func TestIntegration_Capability_AgentConfigPresetNonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityAgentConfigPresetLoader(pool)
	got, err := loader.LoadCapabilityAgentConfigPresets(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_Capability_AllAgentConfigPresetSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentConfigPresetMigration)

	loader := core.NewCoreCapabilityAgentConfigPresetLoader(pool)
	got, err := loader.LoadCapabilityAgentConfigPresets(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 3, "precondition: 3 capability agent config presets loaded")

	// Verify every loaded slug appears in the canonical Go list.
	canonicalSet := map[string]bool{}
	for _, s := range core.SeedCapabilityAgentConfigPresetSlugs {
		canonicalSet[s] = true
	}
	for _, preset := range got {
		assert.True(t, canonicalSet[preset.Slug],
			"DB slug %q must be in SeedCapabilityAgentConfigPresetSlugs (Go constant list)",
			preset.Slug)
	}
}
