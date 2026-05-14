//go:build integration

package core_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const effortMigration = "000075_seed_skill_effort_level_default_templates.up.sql"
const effortMigrationDown = "000075_seed_skill_effort_level_default_templates.down.sql"

func TestIntegration_CoreEffortLevel_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, effortMigration)

	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedEffortLevelRowCount, len(got))
}

func TestIntegration_CoreEffortLevel_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreEffortLevel_FindBySlug_MediumIsDefault(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, effortMigration)

	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "medium")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "Medium", tmpl.Label)
	assert.Equal(t, 8192, tmpl.ThinkingTokens)
	assert.NotEmpty(t, tmpl.RecommendedFor)
}

func TestIntegration_CoreEffortLevel_FindBySlug_LowestHasZeroTokens(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, effortMigration)

	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "lowest")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 0, tmpl.ThinkingTokens, "lowest tier must disable extended thinking")
}

func TestIntegration_CoreEffortLevel_FindBySlug_HighestHasMaxTokens(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, effortMigration)

	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "highest")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, core.SeedEffortLevelMaxTokens, tmpl.ThinkingTokens)
}

func TestIntegration_CoreEffortLevel_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, effortMigration)

	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "ultra")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreEffortLevel_LoadByThinkingEnabled_ExcludesLowest(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, effortMigration)

	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	got, err := loader.LoadByThinkingEnabled(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4, len(got), "low/medium/high/highest have thinking enabled")
	for _, t2 := range got {
		assert.Greater(t, t2.ThinkingTokens, 0)
		assert.NotEqual(t, "lowest", t2.Slug)
	}
}

func TestIntegration_CoreEffortLevel_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, effortMigration)
	applyMigration(t, pool, migDir, effortMigration)

	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedEffortLevelRowCount, len(got))
}

func TestIntegration_CoreEffortLevel_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, effortMigration)
	applyMigration(t, pool, migDir, effortMigrationDown)

	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreEffortLevel_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, effortMigration)

	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedEffortLevelSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreEffortLevel_SortOrderIsAscending(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, effortMigration)

	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, all)
	for i := 1; i < len(all); i++ {
		assert.LessOrEqual(t, all[i-1].SortOrder, all[i].SortOrder)
	}
	assert.Equal(t, "lowest", all[0].Slug, "lowest must sort first")
}

func TestIntegration_CoreEffortLevel_RecommendedForDeserialises(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, effortMigration)

	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "high")
	require.NoError(t, err)
	require.True(t, found)
	assert.NotEmpty(t, tmpl.RecommendedFor, "high tier must have recommended_for entries")
}

func TestIntegration_CoreEffortLevel_AllSlugsMatchKebabRegex(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, effortMigration)

	loader := core.NewCoreSkillEffortLevelDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.True(t, core.SeedEffortLevelSlugRE.MatchString(t2.Slug),
			"slug %q must match kebab regex", t2.Slug)
	}
}
