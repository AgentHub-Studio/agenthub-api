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

const afptMigration = "000071_seed_agent_frontmatter_preset_default_templates.up.sql"
const afptMigrationDown = "000071_seed_agent_frontmatter_preset_default_templates.down.sql"

func TestIntegration_CoreAFPT_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, afptMigration)

	loader := core.NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedAFPTRowCount, len(got))
}

func TestIntegration_CoreAFPT_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreAFPT_FindBySlug_FastResponderShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, afptMigration)

	loader := core.NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "fast-responder")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "Fast Responder", tmpl.Label)
	assert.NotEmpty(t, tmpl.Description)
	assert.NotEmpty(t, tmpl.FrontmatterYAML)
	assert.Contains(t, tmpl.FrontmatterYAML, "temperature")
}

func TestIntegration_CoreAFPT_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, afptMigration)

	loader := core.NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreAFPT_AllSlugsMatchKebabRegex(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, afptMigration)

	loader := core.NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.True(t, core.SeedAFPTSlugRE.MatchString(t2.Slug),
			"slug %q must match kebab regex", t2.Slug)
	}
}

func TestIntegration_CoreAFPT_AllFrontmatterYAMLsContainDescription(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, afptMigration)

	loader := core.NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.True(t, strings.Contains(t2.FrontmatterYAML, "description:"),
			"%s frontmatter_yaml must contain 'description:' field", t2.Slug)
	}
}

func TestIntegration_CoreAFPT_AllFrontmatterYAMLsContainTemperature(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, afptMigration)

	loader := core.NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.True(t, strings.Contains(t2.FrontmatterYAML, "temperature:"),
			"%s frontmatter_yaml must contain 'temperature:' field", t2.Slug)
	}
}

func TestIntegration_CoreAFPT_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, afptMigration)
	applyMigration(t, pool, migDir, afptMigration)

	loader := core.NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedAFPTRowCount, len(got))
}

func TestIntegration_CoreAFPT_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, afptMigration)
	applyMigration(t, pool, migDir, afptMigrationDown)

	loader := core.NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreAFPT_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, afptMigration)

	loader := core.NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedAFPTSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreAFPT_SortOrderAscending(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, afptMigration)

	loader := core.NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, all)
	for i := 1; i < len(all); i++ {
		if all[i-1].SortOrder == all[i].SortOrder {
			assert.LessOrEqual(t, all[i-1].Slug, all[i].Slug)
		} else {
			assert.Less(t, all[i-1].SortOrder, all[i].SortOrder)
		}
	}
	assert.Equal(t, "fast-responder", all[0].Slug)
}

func TestIntegration_CoreAFPT_AllLabelsNonEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, afptMigration)

	loader := core.NewCoreAgentFrontmatterPresetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.NotEmpty(t, t2.Label, "%s label must not be empty", t2.Slug)
		assert.NotEmpty(t, t2.Description, "%s description must not be empty", t2.Slug)
	}
}
