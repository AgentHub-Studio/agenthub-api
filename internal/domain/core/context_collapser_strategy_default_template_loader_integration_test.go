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

const ccsdMigration = "000039_seed_context_collapser_strategy_default_templates.up.sql"
const ccsdMigrationDown = "000039_seed_context_collapser_strategy_default_templates.down.sql"

func TestIntegration_CoreCCSD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCCSDTemplateRowCount, len(got))
}

func TestIntegration_CoreCCSD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCCSD_FindBySlug_LLMRendererBalancedShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "llm-renderer-balanced")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "balanced", tmpl.CollapseStrategy)
	assert.Equal(t, "llm_renderer", tmpl.ConsumerKind)
	assert.True(t, tmpl.PreserveErrorsAlways)
	assert.True(t, tmpl.PreserveCompactSummaryAlways)
}

func TestIntegration_CoreCCSD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreCCSD_LoadByStrategy_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	// minimal: replay-debug + audit-export + dev-debug = 3.
	minimal, err := loader.LoadByStrategy(context.Background(), "minimal")
	require.NoError(t, err)
	assert.Equal(t, 3, len(minimal))

	// balanced: llm-renderer = 1.
	balanced, _ := loader.LoadByStrategy(context.Background(), "balanced")
	assert.Equal(t, 1, len(balanced))

	// aggressive: cost-dashboard + ui-summary = 2.
	aggressive, _ := loader.LoadByStrategy(context.Background(), "aggressive")
	assert.Equal(t, 2, len(aggressive))
}

func TestIntegration_CoreCCSD_LoadByConsumerKind_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	// debugger: replay-debug + dev-debug = 2.
	debugger, err := loader.LoadByConsumerKind(context.Background(), "debugger")
	require.NoError(t, err)
	assert.Equal(t, 2, len(debugger))

	// Others have 1 template each.
	for _, k := range []string{"llm_renderer", "cost_dashboard", "audit_exporter", "ui_summary"} {
		got, _ := loader.LoadByConsumerKind(context.Background(), k)
		assert.Equal(t, 1, len(got), "%s must have 1 template", k)
	}
}

func TestIntegration_CoreCCSD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedCCSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreCCSD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewCCSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreCCSD_AllStrategiesAreInCTX012Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedCCSDTemplateStrategies {
		allowed[s] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.CollapseStrategy])
	}
}

func TestIntegration_CoreCCSD_AllConsumerKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedCCSDTemplateConsumerKinds {
		allowed[k] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.ConsumerKind])
	}
}

func TestIntegration_CoreCCSD_AllUseCasesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedCCSDTemplateUseCases {
		allowed[u] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetUseCase])
	}
}

func TestIntegration_CoreCCSD_AllTenantKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedCCSDTemplateTenantKinds {
		allowed[k] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.RecommendedForTenantKind])
	}
}

func TestIntegration_CoreCCSD_PreserveErrorsAlwaysIsTrueForAllRows(t *testing.T) {
	// Cross-row invariant from CTX-012: errors are ALWAYS preserved.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.True(t, p.PreserveErrorsAlways,
			"%s must declare preserve_errors_always=TRUE (CTX-012 invariant)", p.Slug)
	}
}

func TestIntegration_CoreCCSD_PreserveCompactSummaryAlwaysIsTrueForAllRows(t *testing.T) {
	// Cross-row invariant from PDF §9.2: compact_summary preserved.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.True(t, p.PreserveCompactSummaryAlways,
			"%s must declare preserve_compact_summary_always=TRUE (PDF 9.2)", p.Slug)
	}
}

func TestIntegration_CoreCCSD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreCCSD_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CoreCCSD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
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
	assert.Equal(t, "llm-renderer-balanced", all[0].Slug)
}

func TestIntegration_CoreCCSD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCCSDTemplateRowCount, len(got))
}

func TestIntegration_CoreCCSD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)
	applyMigration(t, pool, migDir, ccsdMigrationDown)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCCSD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ccsdMigration)

	loader := core.NewCoreContextCollapserStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedCCSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
