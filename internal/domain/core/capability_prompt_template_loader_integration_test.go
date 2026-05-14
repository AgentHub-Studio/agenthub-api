//go:build integration

package core_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for CoreCapabilityPromptTemplateLoader against a real
// Postgres (testcontainers pgvector:pg16). Build tag `integration`.
//
// Migration chain applied per test:
//   000001 schema → 000019 prompt_template table + 8 platform rows → 000094 5 capability rows
//
// The ah_core.prompt_template table is created by migration 000019, not 000001.
// Migration 000094 depends on that table existing.

const capabilityPromptMigration = "000094_seed_capability_prompt_templates.up.sql"
const capabilityPromptMigrationDown = "000094_seed_capability_prompt_templates.down.sql"
const platformPromptMigration = "000019_seed_prompt_templates.up.sql"

func TestIntegration_CapabilityPromptTemplate_LoadAfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformPromptMigration)    // creates table + 8 platform rows
	applyMigration(t, pool, migDir, capabilityPromptMigration)  // adds 5 capability rows

	loader := core.NewCoreCapabilityPromptTemplateLoader(pool)
	got, err := loader.LoadCapabilityTemplates(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityPromptTemplateCount, len(got),
		"DB row count must match SeedCapabilityPromptTemplateCount (5) after migration 000094")
}

func TestIntegration_CapabilityPromptTemplate_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	// Do NOT apply any migration — schema and table are missing.

	loader := core.NewCoreCapabilityPromptTemplateLoader(pool)
	got, err := loader.LoadCapabilityTemplates(context.Background())

	require.NoError(t, err, "missing schema must NOT error (non-fatal contract)")
	assert.Empty(t, got, "missing schema must return empty slice, not crash")
}

func TestIntegration_CapabilityPromptTemplate_AllAreCapabilityKind(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformPromptMigration)
	applyMigration(t, pool, migDir, capabilityPromptMigration)

	loader := core.NewCoreCapabilityPromptTemplateLoader(pool)
	got, err := loader.LoadCapabilityTemplates(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	for _, tmpl := range got {
		assert.Equal(t, core.SeedCapabilityPromptTemplateKind, tmpl.TemplateKind,
			"all capability templates must have template_kind='capability', got %q for slug %q",
			tmpl.TemplateKind, tmpl.Slug)
	}
}

func TestIntegration_CapabilityPromptTemplate_AllAreRecommended(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformPromptMigration)
	applyMigration(t, pool, migDir, capabilityPromptMigration)

	loader := core.NewCoreCapabilityPromptTemplateLoader(pool)
	got, err := loader.LoadCapabilityTemplates(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	for _, tmpl := range got {
		assert.True(t, tmpl.IsRecommended,
			"capability template %q must have is_recommended=true — all 5 are curated starters", tmpl.Slug)
	}
}

func TestIntegration_CapabilityPromptTemplate_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformPromptMigration)
	applyMigration(t, pool, migDir, capabilityPromptMigration)
	// Apply 000094 a second time — ON CONFLICT DO NOTHING must keep count stable.
	applyMigration(t, pool, migDir, capabilityPromptMigration)

	loader := core.NewCoreCapabilityPromptTemplateLoader(pool)
	got, err := loader.LoadCapabilityTemplates(context.Background())
	require.NoError(t, err)

	assert.Equal(t, core.SeedCapabilityPromptTemplateCount, len(got),
		"double-apply of migration 000094 must still produce exactly 5 templates (idempotent)")
}

func TestIntegration_CapabilityPromptTemplate_DownMigrationRemovesCapabilityRowsOnly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformPromptMigration)
	applyMigration(t, pool, migDir, capabilityPromptMigration)

	capLoader := core.NewCoreCapabilityPromptTemplateLoader(pool)
	before, err := capLoader.LoadCapabilityTemplates(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: seed produced capability rows")

	// Apply down migration.
	applyMigration(t, pool, migDir, capabilityPromptMigrationDown)

	after, err := capLoader.LoadCapabilityTemplates(context.Background())
	require.NoError(t, err)
	assert.Empty(t, after, "down migration must remove all 5 capability templates")

	// The 8 platform prompt templates must still be intact.
	platformLoader := core.NewCorePromptTemplateLoader(pool)
	platform, err := platformLoader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedPromptTemplateSlugs), len(platform),
		"platform prompt templates must be unaffected by capability prompt down migration")
}

func TestIntegration_CapabilityPromptTemplate_FindBySlug_WebResearchBrief(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformPromptMigration)
	applyMigration(t, pool, migDir, capabilityPromptMigration)

	loader := core.NewCoreCapabilityPromptTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "capability-web-research-brief")
	require.NoError(t, err)
	require.True(t, found, "capability-web-research-brief must be findable after seed")

	assert.Equal(t, "capability-web-research-brief", tmpl.Slug)
	assert.Equal(t, "capability", tmpl.TemplateKind)
	assert.True(t, tmpl.IsRecommended)
	assert.LessOrEqual(t, tmpl.RecommendedTemperature, 0.35,
		"web research brief must use low temperature for factual consistency")
	assert.GreaterOrEqual(t, tmpl.RecommendedMaxTokens, 4096,
		"web research brief must allow enough tokens for a structured brief")
	assert.Contains(t, tmpl.RequiresTools, "core-web-research",
		"web research brief must require the core-web-research skill")
	// Verify placeholder coverage.
	assert.Contains(t, tmpl.Placeholders, "topic")
	assert.Contains(t, tmpl.Placeholders, "depth")
}

func TestIntegration_CapabilityPromptTemplate_SlugsMatchCanonicalConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformPromptMigration)
	applyMigration(t, pool, migDir, capabilityPromptMigration)

	loader := core.NewCoreCapabilityPromptTemplateLoader(pool)
	got, err := loader.LoadCapabilityTemplates(context.Background())
	require.NoError(t, err)

	dbSlugs := make([]string, 0, len(got))
	for _, tmpl := range got {
		dbSlugs = append(dbSlugs, tmpl.Slug)
	}
	sort.Strings(dbSlugs)

	expected := append([]string{}, core.SeedCapabilityPromptTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs,
		"DB slugs must match SeedCapabilityPromptTemplateSlugs exactly")
}

func TestIntegration_CapabilityPromptTemplate_PlaceholdersDocumentedMatchSystemPromptUsage(t *testing.T) {
	// Cross-field consistency: every {{placeholder}} that appears in
	// system_prompt must be listed in the placeholders column.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, platformPromptMigration)
	applyMigration(t, pool, migDir, capabilityPromptMigration)

	loader := core.NewCoreCapabilityPromptTemplateLoader(pool)
	got, err := loader.LoadCapabilityTemplates(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		documented := map[string]bool{}
		for _, p := range tmpl.PlaceholdersList() {
			documented[p] = true
		}
		// Find all {{...}} usages in system_prompt.
		s := tmpl.SystemPrompt
		for {
			start := strings.Index(s, "{{")
			if start < 0 {
				break
			}
			end := strings.Index(s[start:], "}}")
			if end < 0 {
				break
			}
			name := s[start+2 : start+end]
			assert.True(t, documented[name],
				"template %q uses {{%s}} but doesn't document it in placeholders column",
				tmpl.Slug, name)
			s = s[start+end+2:]
		}
	}
}
