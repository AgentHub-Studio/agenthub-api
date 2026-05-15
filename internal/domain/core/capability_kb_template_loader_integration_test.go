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

// Integration tests for CoreCapabilityKBTemplateLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//   000001 schema → 000017 knowledge_base_template table + 7 platform rows → 000096 3 capability rows
//
// The ah_core.knowledge_base_template table is created by migration 000017, not 000001.
// Migration 000096 depends on that table existing.

const capabilityKBMigration = "000096_seed_capability_kb_templates.up.sql"
const capabilityKBMigrationDown = "000096_seed_capability_kb_templates.down.sql"
const platformKBMigration = "000017_seed_knowledge_base_templates.up.sql"

func TestIntegration_CapabilityKB_LoadThreeAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformKBMigration)    // creates table + 7 platform rows
	applyMigration(t, pool, migDir, capabilityKBMigration)  // adds 3 capability rows

	loader := core.NewCoreCapabilityKBTemplateLoader(pool)
	got, err := loader.LoadCapabilityKBTemplates(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityKBTemplateCount, len(got),
		"DB row count must match SeedCapabilityKBTemplateCount (3) after migration 000096")
}

func TestIntegration_CapabilityKB_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityKBTemplateLoader(pool)
	got, err := loader.LoadCapabilityKBTemplates(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_CapabilityKB_AllAreRecommended(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformKBMigration)
	applyMigration(t, pool, migDir, capabilityKBMigration)

	loader := core.NewCoreCapabilityKBTemplateLoader(pool)
	got, err := loader.LoadCapabilityKBTemplates(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	for _, tmpl := range got {
		assert.True(t, tmpl.IsRecommended,
			"capability KB template %q must have is_recommended=TRUE (surfaces in Suggested section)",
			tmpl.Slug)
	}
}

func TestIntegration_CapabilityKB_IdempotentDoubleApply(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformKBMigration)
	applyMigration(t, pool, migDir, capabilityKBMigration)
	// Apply 000096 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityKBMigration)

	loader := core.NewCoreCapabilityKBTemplateLoader(pool)
	got, err := loader.LoadCapabilityKBTemplates(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityKBTemplateCount, len(got),
		"double-apply of migration 000096 must still produce exactly 3 capability KB templates (idempotent)")
}

func TestIntegration_CapabilityKB_DownRemovesOnlyThreePlatformSevenRemain(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformKBMigration)
	applyMigration(t, pool, migDir, capabilityKBMigration)

	capLoader := core.NewCoreCapabilityKBTemplateLoader(pool)
	before, err := capLoader.LoadCapabilityKBTemplates(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability rows")

	// Apply down migration.
	applyMigration(t, pool, migDir, capabilityKBMigrationDown)

	after, err := capLoader.LoadCapabilityKBTemplates(context.Background())
	require.NoError(t, err)
	assert.Empty(t, after, "down migration must remove all 3 capability KB templates")

	// The 7 platform KB templates must still be intact.
	platformLoader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	platform, err := platformLoader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedKBTemplateSlugs), len(platform),
		"platform KB templates must be unaffected by capability KB template down migration")
}

func TestIntegration_CapabilityKB_FindBySlug_ResearchKBTemplate(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformKBMigration)
	applyMigration(t, pool, migDir, capabilityKBMigration)

	loader := core.NewCoreCapabilityKBTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), core.SeedResearchKBTemplateSlug)
	require.NoError(t, err)
	require.True(t, found, "%q must be findable after seed", core.SeedResearchKBTemplateSlug)

	assert.Equal(t, "research-collection-template", tmpl.Slug)
	assert.Equal(t, "research_collection", tmpl.TemplateKind)
	assert.Equal(t, "semantic", tmpl.ChunkStrategy)
	assert.True(t, tmpl.IsRecommended, "research-collection-template must be recommended")
	assert.False(t, tmpl.RequiresAdminReview, "research-collection-template must NOT require admin review")
	assert.Equal(t, 100, tmpl.SortOrder, "research-collection-template must have sort_order=100")
	assert.Equal(t, "local/e5-large", tmpl.DefaultEmbeddingProviderSlug,
		"research-collection-template must use local/e5-large embedding")
}

func TestIntegration_CapabilityKB_KindsAreNewNotInPlatformSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformKBMigration)
	applyMigration(t, pool, migDir, capabilityKBMigration)

	capLoader := core.NewCoreCapabilityKBTemplateLoader(pool)
	capTemplates, err := capLoader.LoadCapabilityKBTemplates(context.Background())
	require.NoError(t, err)
	require.Len(t, capTemplates, 3, "precondition: 3 capability KB templates loaded")

	// Load all 10 templates (7 platform + 3 capability) to get all kinds.
	platformLoader := core.NewCoreKnowledgeBaseTemplateLoader(pool)
	allTemplates, err := platformLoader.LoadAll(context.Background())
	require.NoError(t, err)

	// Build platform kind set (from templates NOT in capability set).
	capSlugs := map[string]bool{}
	for _, s := range core.SeedCapabilityKBTemplateSlugs {
		capSlugs[s] = true
	}
	platformKinds := map[string]bool{}
	for _, tmpl := range allTemplates {
		if !capSlugs[tmpl.Slug] {
			platformKinds[tmpl.TemplateKind] = true
		}
	}

	// Verify capability kinds do NOT appear in the platform kind set.
	for _, capTmpl := range capTemplates {
		assert.False(t, platformKinds[capTmpl.TemplateKind],
			"capability KB kind %q must NOT exist in platform KB kinds (migration 000017)",
			capTmpl.TemplateKind)
	}
}

func TestIntegration_CapabilityKB_SortOrdersAre100To102(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformKBMigration)
	applyMigration(t, pool, migDir, capabilityKBMigration)

	loader := core.NewCoreCapabilityKBTemplateLoader(pool)
	got, err := loader.LoadCapabilityKBTemplates(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 3, "precondition: 3 capability KB templates loaded")

	// Sort orders must be 100, 101, 102 (ascending order returned by query).
	sortOrders := make([]int, 0, 3)
	for _, tmpl := range got {
		sortOrders = append(sortOrders, tmpl.SortOrder)
	}
	assert.Equal(t, []int{100, 101, 102}, sortOrders,
		"capability KB templates must have sort_order 100, 101, 102 in ascending order")
}
