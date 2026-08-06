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

func TestIntegration_CorePromptTemplate_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedPromptTemplateSlugs), len(got))
}

func TestIntegration_CorePromptTemplate_SeedKeepsTenantTableIntact(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")

	_, err := pool.Exec(context.Background(), `
		CREATE TABLE ah_core.prompt_template (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			slug TEXT NOT NULL
		);
		INSERT INTO ah_core.prompt_template (slug) VALUES ('tenant-template');
	`)
	require.NoError(t, err)

	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	platform, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, platform, len(core.SeedExpectedPromptTemplateSlugs))

	var tenantCount int
	err = pool.QueryRow(context.Background(), `SELECT count(*) FROM ah_core.prompt_template`).Scan(&tenantCount)
	require.NoError(t, err)
	assert.Equal(t, 1, tenantCount)

	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.down.sql")
	err = pool.QueryRow(context.Background(), `SELECT count(*) FROM ah_core.prompt_template`).Scan(&tenantCount)
	require.NoError(t, err)
	assert.Equal(t, 1, tenantCount)
}

func TestIntegration_CorePromptTemplate_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePromptTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePromptTemplate_FindBySlug_ReturnsKnownTemplate(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "code-reviewer")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "coder", tmpl.TemplateKind)
	assert.Less(t, tmpl.RecommendedTemperature, 0.5,
		"code-reviewer must use low temperature for deterministic output")
	assert.True(t, tmpl.IsRecommended)
	assert.Contains(t, tmpl.SystemPrompt, "code reviewer")
}

func TestIntegration_CorePromptTemplate_LoadByKind_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	got, err := loader.LoadByKind(context.Background(), "coder")
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
}

func TestIntegration_CorePromptTemplate_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	got, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	dbRec := []string{}
	for _, tmpl := range got {
		dbRec = append(dbRec, tmpl.Slug)
	}
	sort.Strings(dbRec)
	expected := append([]string{}, core.SeedRecommendedPromptTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbRec)
}

func TestIntegration_CorePromptTemplate_AllKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedPromptTemplateKinds {
		allowed[k] = true
	}
	for _, tmpl := range got {
		assert.True(t, allowed[tmpl.TemplateKind])
	}
}

func TestIntegration_CorePromptTemplate_AllSystemPromptsAreSubstantial(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, len(tmpl.SystemPrompt), 100,
			"template %q system_prompt must be substantial (≥100 chars)", tmpl.Slug)
	}
}

func TestIntegration_CorePromptTemplate_TemperaturesInValidRange(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, tmpl.RecommendedTemperature, 0.0)
		assert.LessOrEqual(t, tmpl.RecommendedTemperature, 2.0)
	}
}

func TestIntegration_CorePromptTemplate_DataExtractorUsesNearZeroTemperature(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "data-extractor")
	require.NoError(t, err)
	require.True(t, found)
	assert.LessOrEqual(t, tmpl.RecommendedTemperature, 0.2,
		"data-extractor needs near-zero temperature for JSON consistency")
}

func TestIntegration_CorePromptTemplate_PlaceholdersDocumentedMatchSystemPromptUsage(t *testing.T) {
	// Cross-field consistency: every {{placeholder}} that appears in
	// system_prompt must be listed in the placeholders column.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		documented := map[string]bool{}
		for _, p := range tmpl.PlaceholdersList() {
			documented[p] = true
		}
		// Find {{...}} in system_prompt.
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

func TestIntegration_CorePromptTemplate_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, tmpl := range got {
		assert.False(t, seen[tmpl.Slug])
		seen[tmpl.Slug] = true
	}
}

func TestIntegration_CorePromptTemplate_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, len(tmpl.Description), 30)
	}
}

func TestIntegration_CorePromptTemplate_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CorePromptTemplate_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, tmpl := range got {
		dbSlugs = append(dbSlugs, tmpl.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedPromptTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}

func TestIntegration_CorePromptTemplate_RenderSubstitutesActualSeedPlaceholders(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000019_seed_prompt_templates.up.sql")

	loader := core.NewCorePromptTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "general-assistant")
	require.NoError(t, err)
	require.True(t, found)

	rendered := tmpl.RenderSystemPrompt(map[string]string{"tenantName": "AcmeCorp"})
	assert.Contains(t, rendered, "AcmeCorp")
	assert.NotContains(t, rendered, "{{tenantName}}")
}
