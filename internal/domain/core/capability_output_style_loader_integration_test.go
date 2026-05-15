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

// Integration tests for CoreCapabilityOutputStyleLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//   000001 schema → 000011 output_style table + 8 platform rows → 000095 3 capability rows
//
// The ah_core.output_style table is created by migration 000011, not 000001.
// Migration 000095 depends on that table existing.

const capabilityOutputStyleMigration = "000095_seed_capability_output_style_templates.up.sql"
const capabilityOutputStyleMigrationDown = "000095_seed_capability_output_style_templates.down.sql"
const platformOutputStyleMigration = "000011_seed_output_styles.up.sql"

func TestIntegration_CapabilityOutputStyle_LoadThreeAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformOutputStyleMigration)  // creates table + 8 platform rows
	applyMigration(t, pool, migDir, capabilityOutputStyleMigration) // adds 3 capability rows

	loader := core.NewCoreCapabilityOutputStyleLoader(pool)
	got, err := loader.LoadCapabilityStyles(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityOutputStyleCount, len(got),
		"DB row count must match SeedCapabilityOutputStyleCount (3) after migration 000095")
}

func TestIntegration_CapabilityOutputStyle_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityOutputStyleLoader(pool)
	got, err := loader.LoadCapabilityStyles(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_CapabilityOutputStyle_AllAreMarkdownFormat(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformOutputStyleMigration)
	applyMigration(t, pool, migDir, capabilityOutputStyleMigration)

	loader := core.NewCoreCapabilityOutputStyleLoader(pool)
	got, err := loader.LoadCapabilityStyles(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	for _, s := range got {
		assert.Equal(t, core.SeedCapabilityOutputStyleFormat, s.OutputFormat,
			"capability output style %q must have output_format=%q",
			s.Slug, core.SeedCapabilityOutputStyleFormat)
	}
}

func TestIntegration_CapabilityOutputStyle_SortOrders100To102(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformOutputStyleMigration)
	applyMigration(t, pool, migDir, capabilityOutputStyleMigration)

	loader := core.NewCoreCapabilityOutputStyleLoader(pool)
	got, err := loader.LoadCapabilityStyles(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 3, "precondition: 3 capability styles loaded")

	// Sort orders must be 100, 101, 102 (ascending order returned by query).
	sortOrders := make([]int, 0, 3)
	for _, s := range got {
		sortOrders = append(sortOrders, s.SortOrder)
	}
	assert.Equal(t, []int{100, 101, 102}, sortOrders,
		"capability output styles must have sort_order 100, 101, 102 in ascending order")
}

func TestIntegration_CapabilityOutputStyle_IdempotentDoubleApply(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformOutputStyleMigration)
	applyMigration(t, pool, migDir, capabilityOutputStyleMigration)
	// Apply 000095 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityOutputStyleMigration)

	loader := core.NewCoreCapabilityOutputStyleLoader(pool)
	got, err := loader.LoadCapabilityStyles(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityOutputStyleCount, len(got),
		"double-apply of migration 000095 must still produce exactly 3 capability styles (idempotent)")
}

func TestIntegration_CapabilityOutputStyle_DownRemovesOnlyThreePlatformEightRemain(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformOutputStyleMigration)
	applyMigration(t, pool, migDir, capabilityOutputStyleMigration)

	capLoader := core.NewCoreCapabilityOutputStyleLoader(pool)
	before, err := capLoader.LoadCapabilityStyles(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability rows")

	// Apply down migration.
	applyMigration(t, pool, migDir, capabilityOutputStyleMigrationDown)

	after, err := capLoader.LoadCapabilityStyles(context.Background())
	require.NoError(t, err)
	assert.Empty(t, after, "down migration must remove all 3 capability output styles")

	// The 8 platform output styles must still be intact.
	platformLoader := core.NewCoreOutputStyleLoader(pool)
	platform, err := platformLoader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedOutputStyleSlugs), len(platform),
		"platform output styles must be unaffected by capability output style down migration")
}

func TestIntegration_CapabilityOutputStyle_FindBySlug_ResearchReport(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformOutputStyleMigration)
	applyMigration(t, pool, migDir, capabilityOutputStyleMigration)

	loader := core.NewCoreCapabilityOutputStyleLoader(pool)
	style, found, err := loader.FindBySlug(context.Background(), core.SeedResearchOutputStyleSlug)
	require.NoError(t, err)
	require.True(t, found, "%q must be findable after seed", core.SeedResearchOutputStyleSlug)

	assert.Equal(t, "research-report", style.Slug)
	assert.Equal(t, "markdown", style.OutputFormat)
	assert.False(t, style.IsDefault, "research-report must NOT be platform default")
	assert.True(t, style.IsActive, "research-report must be active")
	assert.Equal(t, 100, style.SortOrder, "research-report must have sort_order=100")
	assert.Equal(t, 800, style.MaxWords, "research-report must cap at 800 words")
	assert.NotEmpty(t, style.PromptTemplate, "research-report must have a non-empty prompt_template")
}

func TestIntegration_CapabilityOutputStyle_AllHaveNonZeroMaxWords(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformOutputStyleMigration)
	applyMigration(t, pool, migDir, capabilityOutputStyleMigration)

	loader := core.NewCoreCapabilityOutputStyleLoader(pool)
	got, err := loader.LoadCapabilityStyles(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	for _, s := range got {
		assert.Greater(t, s.MaxWords, 0,
			"capability output style %q must have max_words > 0 (structured reports are bounded)",
			s.Slug)
	}
}
