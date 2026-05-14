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

const trbdMigration = "000034_seed_tool_result_budget_default_templates.up.sql"
const trbdMigrationDown = "000034_seed_tool_result_budget_default_templates.down.sql"

func TestIntegration_CoreTRBD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedTRBDTemplateRowCount, len(got))
}

func TestIntegration_CoreTRBD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreTRBD_FindBySlug_BalancedDefaultMatchesCTX008(t *testing.T) {
	// Cross-feature invariant: balanced-default DB row MUST match
	// CTX-008 DefaultToolResultBudgetConfig byte-for-byte.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "balanced-default")
	require.NoError(t, err)
	require.True(t, found)
	// CTX-008 DefaultToolResultBudgetConfig: 4000/12000/50000/8000.
	assert.Equal(t, 4000, tmpl.PerCallMaxTokens)
	assert.Equal(t, 12000, tmpl.PerTurnMaxTokens)
	assert.Equal(t, 50000, tmpl.PerRunMaxTokens)
	assert.Equal(t, 8000, tmpl.ForceSummarizeOverTokens)
}

func TestIntegration_CoreTRBD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreTRBD_LoadByUseCase_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	general, err := loader.LoadByUseCase(context.Background(), "general")
	require.NoError(t, err)
	// balanced-default + small-context-tight + cost-strict + dev-debug.
	assert.Equal(t, 4, len(general))

	research, _ := loader.LoadByUseCase(context.Background(), "research")
	assert.Equal(t, 1, len(research))

	code, _ := loader.LoadByUseCase(context.Background(), "code")
	assert.Equal(t, 1, len(code))
}

func TestIntegration_CoreTRBD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedTRBDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreTRBD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewTRBDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreTRBD_AllUseCasesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedTRBDTemplateUseCases {
		allowed[u] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetUseCase],
			"template %q has use case %q outside expected set",
			p.Slug, p.TargetUseCase)
	}
}

func TestIntegration_CoreTRBD_AllModelFamiliesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, m := range core.SeedExpectedTRBDTemplateModelFamilies {
		allowed[m] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.RecommendedForModelFamily])
	}
}

func TestIntegration_CoreTRBD_AllCapsAreNonNegative(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, p.PerCallMaxTokens, 0, "%s per_call", p.Slug)
		assert.GreaterOrEqual(t, p.PerTurnMaxTokens, 0, "%s per_turn", p.Slug)
		assert.GreaterOrEqual(t, p.PerRunMaxTokens, 0, "%s per_run", p.Slug)
		assert.GreaterOrEqual(t, p.ForceSummarizeOverTokens, 0, "%s force_summarize", p.Slug)
	}
}

func TestIntegration_CoreTRBD_PerCallLadderIsMonotonic(t *testing.T) {
	// Cross-template ladder invariant:
	//   small(1k) ≤ cost_strict(2k) ≤ balanced(4k) ≤ code(8k) ≤ research(16k)
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	perCall := func(slug string) int {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		return tmpl.PerCallMaxTokens
	}

	assert.LessOrEqual(t, perCall("small-context-tight"), perCall("cost-strict"))
	assert.LessOrEqual(t, perCall("cost-strict"), perCall("balanced-default"))
	assert.LessOrEqual(t, perCall("balanced-default"), perCall("code-heavy"))
	assert.LessOrEqual(t, perCall("code-heavy"), perCall("research-heavy"))
}

func TestIntegration_CoreTRBD_PerTurnGrowsWithPerCall(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		if p.PerCallMaxTokens == 0 {
			continue // dev-debug unlimited
		}
		// per_turn should be at least 3x per_call (allows ~3 tool calls per turn).
		assert.GreaterOrEqual(t, p.PerTurnMaxTokens, p.PerCallMaxTokens*3,
			"%s per_turn=%d should be ≥ 3x per_call=%d (multi-tool turns)",
			p.Slug, p.PerTurnMaxTokens, p.PerCallMaxTokens)
	}
}

func TestIntegration_CoreTRBD_PerRunGrowsWithPerTurn(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		if p.PerTurnMaxTokens == 0 {
			continue
		}
		// per_run should be at least 2x per_turn (allows ≥ 2 turns).
		assert.GreaterOrEqual(t, p.PerRunMaxTokens, p.PerTurnMaxTokens*2,
			"%s per_run=%d should be ≥ 2x per_turn=%d (multi-turn runs)",
			p.Slug, p.PerRunMaxTokens, p.PerTurnMaxTokens)
	}
}

func TestIntegration_CoreTRBD_ForceSummarizePositiveExceptDev(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		if p.Slug == "dev-debug" {
			continue
		}
		assert.Positive(t, p.ForceSummarizeOverTokens,
			"non-dev %s must have force_summarize > 0 (else huge results blunt-truncate)", p.Slug)
	}
}

func TestIntegration_CoreTRBD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreTRBD_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CoreTRBD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
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
	assert.Equal(t, "balanced-default", all[0].Slug)
}

func TestIntegration_CoreTRBD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedTRBDTemplateRowCount, len(got))
}

func TestIntegration_CoreTRBD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)
	applyMigration(t, pool, migDir, trbdMigrationDown)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreTRBD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, trbdMigration)

	loader := core.NewCoreToolResultBudgetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedTRBDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
