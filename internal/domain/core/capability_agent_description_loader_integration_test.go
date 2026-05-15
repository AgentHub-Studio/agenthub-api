//go:build integration

package core_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for CoreCapabilityAgentDescriptionLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//
//	000001 schema → 000124 creates capability_agent_description table + 9 rows

const capabilityAgentDescriptionMigration = "000124_seed_capability_agent_descriptions.up.sql"
const capabilityAgentDescriptionMigrationDown = "000124_seed_capability_agent_descriptions.down.sql"

func TestIntegration_CapabilityAgentDescription_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentDescriptionMigration)

	loader := core.NewCoreCapabilityAgentDescriptionLoader(pool)
	got, err := loader.LoadCapabilityAgentDescriptions(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedAgentDescriptionCount, len(got),
		"DB row count must match SeedAgentDescriptionCount (9) after migration 000124")
}

func TestIntegration_CapabilityAgentDescription_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityAgentDescriptionLoader(pool)
	got, err := loader.LoadCapabilityAgentDescriptions(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_CapabilityAgentDescription_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentDescriptionMigration)
	// Apply 000124 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityAgentDescriptionMigration)

	loader := core.NewCoreCapabilityAgentDescriptionLoader(pool)
	got, err := loader.LoadCapabilityAgentDescriptions(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedAgentDescriptionCount, len(got),
		"double-apply of migration 000124 must still produce exactly 9 rows (idempotent)")
}

func TestIntegration_CapabilityAgentDescription_DownMigrationRemovesRows(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentDescriptionMigration)

	loader := core.NewCoreCapabilityAgentDescriptionLoader(pool)
	before, err := loader.LoadCapabilityAgentDescriptions(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced description rows")

	// Apply down migration — removes rows and drops table.
	applyMigration(t, pool, migDir, capabilityAgentDescriptionMigrationDown)

	after, err := loader.LoadCapabilityAgentDescriptions(context.Background())
	require.NoError(t, err, "non-fatal: missing table after down migration must not error")
	assert.Empty(t, after, "down migration must remove all 9 description rows and drop the table")
}

func TestIntegration_CapabilityAgentDescription_GetTaglineForResearcher(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentDescriptionMigration)

	loader := core.NewCoreCapabilityAgentDescriptionLoader(pool)
	value, found, err := loader.GetDescriptionValue(context.Background(), "core-researcher", core.SeedDescKeyTagline)

	require.NoError(t, err)
	assert.True(t, found, "tagline row for core-researcher must exist after seed")
	assert.NotEmpty(t, value, "tagline value for core-researcher must be non-empty")
	assert.Equal(t, core.SeedResearcherTagline, value,
		"DB tagline value must match SeedResearcherTagline constant")
}

func TestIntegration_CapabilityAgentDescription_GetUseCasesForPlanner(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capabilityAgentDescriptionMigration)

	loader := core.NewCoreCapabilityAgentDescriptionLoader(pool)
	value, found, err := loader.GetDescriptionValue(context.Background(), "core-planner", core.SeedDescKeyUseCases)

	require.NoError(t, err)
	assert.True(t, found, "use_cases row for core-planner must exist after seed")
	assert.True(t, strings.Contains(value, core.SeedUseCaseSeparator),
		"core-planner use_cases value must contain pipe separator %q: got %q",
		core.SeedUseCaseSeparator, value)
}
