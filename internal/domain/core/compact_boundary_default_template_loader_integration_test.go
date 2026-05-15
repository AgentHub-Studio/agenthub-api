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

const cbdMigration = "000036_seed_compact_boundary_default_templates.up.sql"
const cbdMigrationDown = "000036_seed_compact_boundary_default_templates.down.sql"

func TestIntegration_CoreCBD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCBDTemplateRowCount, len(got))
}

func TestIntegration_CoreCBD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCBD_FindBySlug_HybridBalancedShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "hybrid-balanced")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "hybrid_balanced", tmpl.TriggerKind)
	assert.Equal(t, 30, tmpl.MaxTurnsBeforeCompact)
	assert.Equal(t, 70, tmpl.TokenPressurePct)
	assert.InDelta(t, 5.00, tmpl.CostThresholdUSD, 0.001)
	assert.Equal(t, 1800, tmpl.IdleSeconds)
	assert.Equal(t, 8, tmpl.PreserveTailMessages)
}

func TestIntegration_CoreCBD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreCBD_LoadByTriggerKind_OneTemplatePerKind(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	for _, kind := range core.SeedExpectedCBDTemplateTriggerKinds {
		got, err := loader.LoadByTriggerKind(context.Background(), kind)
		require.NoError(t, err)
		assert.Equal(t, 1, len(got),
			"each trigger_kind must have exactly 1 template (got %d for %q)",
			len(got), kind)
	}
}

func TestIntegration_CoreCBD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedCBDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreCBD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewCBDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreCBD_AllTriggerKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedCBDTemplateTriggerKinds {
		allowed[k] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TriggerKind])
	}
}

func TestIntegration_CoreCBD_AllUseCasesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedCBDTemplateUseCases {
		allowed[u] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetUseCase])
	}
}

func TestIntegration_CoreCBD_AllModelFamiliesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, m := range core.SeedExpectedCBDTemplateModelFamilies {
		allowed[m] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.RecommendedForModelFamily])
	}
}

func TestIntegration_CoreCBD_AllValuesAreNonNegative(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, p.MaxTurnsBeforeCompact, 0)
		assert.GreaterOrEqual(t, p.TokenPressurePct, 0)
		assert.LessOrEqual(t, p.TokenPressurePct, 100)
		assert.GreaterOrEqual(t, p.CostThresholdUSD, 0.0)
		assert.GreaterOrEqual(t, p.IdleSeconds, 0)
		assert.GreaterOrEqual(t, p.PreserveTailMessages, 0)
	}
}

func TestIntegration_CoreCBD_HybridAndTokenPressureUseCTX010DefaultThreshold(t *testing.T) {
	// Cross-feature invariant: hybrid-balanced + token-pressure-driven
	// MUST use 70% threshold (CLAUDE.md ContextManager default).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	for _, slug := range []string{"hybrid-balanced", "token-pressure-driven"} {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		assert.Equal(t, 70, tmpl.TokenPressurePct,
			"%s must use 70%% (CTX-010 ContextManager default)", slug)
	}
}

func TestIntegration_CoreCBD_DevDebugDisablesAllTriggers(t *testing.T) {
	// Cross-row invariant: dev-debug-no-compact has all thresholds 0.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "dev-debug-no-compact")
	require.NoError(t, err)
	assert.Equal(t, 0, tmpl.MaxTurnsBeforeCompact)
	assert.Equal(t, 0, tmpl.TokenPressurePct)
	assert.InDelta(t, 0.0, tmpl.CostThresholdUSD, 0.001)
	assert.Equal(t, 0, tmpl.IdleSeconds)
	assert.Equal(t, 0, tmpl.PreserveTailMessages)
}

func TestIntegration_CoreCBD_NonDevPreserveTailIsPositive(t *testing.T) {
	// Cross-row invariant: every non-dev template preserves ≥1 message.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		if p.Slug == "dev-debug-no-compact" {
			continue
		}
		assert.Positive(t, p.PreserveTailMessages,
			"non-dev %s must preserve ≥1 tail message", p.Slug)
	}
}

func TestIntegration_CoreCBD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreCBD_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CoreCBD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
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
	assert.Equal(t, "hybrid-balanced", all[0].Slug,
		"first by sort_order=10 must be hybrid-balanced")
}

func TestIntegration_CoreCBD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCBDTemplateRowCount, len(got))
}

func TestIntegration_CoreCBD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)
	applyMigration(t, pool, migDir, cbdMigrationDown)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCBD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, cbdMigration)

	loader := core.NewCoreCompactBoundaryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedCBDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
