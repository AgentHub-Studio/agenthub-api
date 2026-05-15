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

const tppdMigration = "000044_seed_tool_pool_provider_default_templates.up.sql"
const tppdMigrationDown = "000044_seed_tool_pool_provider_default_templates.down.sql"

func TestIntegration_CoreTPPD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedTPPDTemplateRowCount, len(got))
}

func TestIntegration_CoreTPPD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreTPPD_FindBySlug_BuiltinReadOnlyShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "builtin-readonly-core")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "builtin", tmpl.TargetSource)
	assert.Equal(t, "inspection", tmpl.TargetUseCase)
	assert.False(t, tmpl.RequiresAdminReview)
	exposes, err := tmpl.ExposesToolNames()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Read", "Grep", "Glob"}, exposes)
}

func TestIntegration_CoreTPPD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreTPPD_LoadBySource_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	builtin, err := loader.LoadBySource(context.Background(), "builtin")
	require.NoError(t, err)
	assert.Equal(t, 2, len(builtin))

	skill, _ := loader.LoadBySource(context.Background(), "skill")
	assert.Equal(t, 1, len(skill))

	mcp, _ := loader.LoadBySource(context.Background(), "mcp")
	assert.Equal(t, 1, len(mcp))

	sub, _ := loader.LoadBySource(context.Background(), "subagent")
	assert.Equal(t, 1, len(sub))

	ext, _ := loader.LoadBySource(context.Background(), "extension")
	assert.Equal(t, 1, len(ext))
}

func TestIntegration_CoreTPPD_LoadByUseCase_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedTPPDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(matched), 1, "use case %q must have ≥1 example", uc)
	}
}

func TestIntegration_CoreTPPD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedTPPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreTPPD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewTPPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreTPPD_AllSourcesAreInTOOL003Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedTPPDTemplateSources {
		allowed[s] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetSource],
			"%s source %q outside TOOL-003 enum", p.Slug, p.TargetSource)
	}
}

func TestIntegration_CoreTPPD_AllUseCasesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedTPPDTemplateUseCases {
		allowed[u] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetUseCase])
	}
}

func TestIntegration_CoreTPPD_AllSourcesHaveExample(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	covered := map[string]bool{}
	for _, p := range all {
		covered[p.TargetSource] = true
	}
	for _, src := range core.SeedExpectedTPPDTemplateSources {
		assert.True(t, covered[src],
			"source %q must have at least one example", src)
	}
}

func TestIntegration_CoreTPPD_BuiltinHasLowestPriority(t *testing.T) {
	// Cross-template priority ladder: builtin < others (builtins are
	// registered first in the pool — TOOL-003 toolSourceRank).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	builtinReadonly, _, _ := loader.FindBySlug(context.Background(), "builtin-readonly-core")
	mcp, _, _ := loader.FindBySlug(context.Background(), "mcp-filesystem-default")
	ext, _, _ := loader.FindBySlug(context.Background(), "extension-platform-utilities")

	assert.Less(t, builtinReadonly.DefaultPriority, mcp.DefaultPriority)
	assert.Less(t, builtinReadonly.DefaultPriority, ext.DefaultPriority)
}

func TestIntegration_CoreTPPD_MutatingProviderRequiresAdminReview(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "builtin-mutating-core")
	require.NoError(t, err)
	assert.True(t, tmpl.RequiresAdminReview)
}

func TestIntegration_CoreTPPD_ExposesToolNamesAreNonEmptyArrays(t *testing.T) {
	// Every provider must expose ≥1 tool name (else its presence is moot).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		exposes, err := p.ExposesToolNames()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(exposes), 1, "%s exposes_tool_names empty", p.Slug)
	}
}

func TestIntegration_CoreTPPD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreTPPD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CoreTPPD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
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
	assert.Equal(t, "builtin-readonly-core", all[0].Slug)
}

func TestIntegration_CoreTPPD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedTPPDTemplateRowCount, len(got))
}

func TestIntegration_CoreTPPD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)
	applyMigration(t, pool, migDir, tppdMigrationDown)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreTPPD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tppdMigration)

	loader := core.NewCoreToolPoolProviderDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedTPPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
