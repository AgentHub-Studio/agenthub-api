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

const crdMigration = "000035_seed_content_reference_default_templates.up.sql"
const crdMigrationDown = "000035_seed_content_reference_default_templates.down.sql"

func TestIntegration_CoreCRD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCRDTemplateRowCount, len(got))
}

func TestIntegration_CoreCRD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCRD_FindBySlug_BalancedDefaultReferencesThreeKinds(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "balanced-default")
	require.NoError(t, err)
	require.True(t, found)
	kinds := tmpl.EnabledKindsList()
	assert.ElementsMatch(t, []string{"kb_chunk", "tool_result", "file_blob"}, kinds)
	assert.Equal(t, 4096, tmpl.MinBytesToReference)
	assert.Equal(t, 86400, tmpl.IdleGCSeconds)
}

func TestIntegration_CoreCRD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreCRD_LoadByUseCase_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	general, err := loader.LoadByUseCase(context.Background(), "general")
	require.NoError(t, err)
	// balanced + kb-heavy + cost-strict + dev-debug = 4.
	assert.Equal(t, 4, len(general))

	research, _ := loader.LoadByUseCase(context.Background(), "research")
	assert.Equal(t, 1, len(research))

	code, _ := loader.LoadByUseCase(context.Background(), "code")
	assert.Equal(t, 1, len(code))
}

func TestIntegration_CoreCRD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedCRDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreCRD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewCRDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreCRD_AllUseCasesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedCRDTemplateUseCases {
		allowed[u] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetUseCase])
	}
}

func TestIntegration_CoreCRD_AllTenantKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedCRDTemplateTenantKinds {
		allowed[k] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.RecommendedForTenantKind])
	}
}

func TestIntegration_CoreCRD_AllEnabledKindsAreInCTX009Enum(t *testing.T) {
	// Cross-row invariant: every enabled_kind label MUST be in the
	// CTX-009 ContentReferenceKind enum (no orphan labels).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedCRDTemplateKindLabels {
		allowed[k] = true
	}
	for _, tmpl := range all {
		for _, k := range tmpl.EnabledKindsList() {
			assert.True(t, allowed[k],
				"template %q references kind %q outside CTX-009 enum",
				tmpl.Slug, k)
		}
	}
}

func TestIntegration_CoreCRD_ThresholdsLadderIsMonotonic(t *testing.T) {
	// Cross-template ladder invariant:
	//   research(1k) ≤ kb-heavy(2k) ≤ balanced(4k) = code-heavy(4k) ≤ cost-strict(16k)
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	threshold := func(slug string) int {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		return tmpl.MinBytesToReference
	}

	assert.LessOrEqual(t, threshold("research-heavy"), threshold("kb-heavy-only"))
	assert.LessOrEqual(t, threshold("kb-heavy-only"), threshold("balanced-default"))
	assert.LessOrEqual(t, threshold("balanced-default"), threshold("code-heavy"))
	assert.LessOrEqual(t, threshold("code-heavy"), threshold("cost-strict"))
}

func TestIntegration_CoreCRD_ResearchHeavyHasLongestGCWindow(t *testing.T) {
	// Research replay needs week-long window; prod uses 24h.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	research, _, err := loader.FindBySlug(context.Background(), "research-heavy")
	require.NoError(t, err)
	balanced, _, err := loader.FindBySlug(context.Background(), "balanced-default")
	require.NoError(t, err)
	assert.Greater(t, research.IdleGCSeconds, balanced.IdleGCSeconds,
		"research-heavy GC window must exceed balanced (replay-friendly)")
}

func TestIntegration_CoreCRD_DevDebugHasZeroEnabledKindsAndZeroGC(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	dev, _, err := loader.FindBySlug(context.Background(), "dev-debug")
	require.NoError(t, err)
	assert.Empty(t, dev.EnabledKindsList())
	assert.Equal(t, 0, dev.IdleGCSeconds)
}

func TestIntegration_CoreCRD_AllValuesAreNonNegative(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, p.MinBytesToReference, 0)
		assert.GreaterOrEqual(t, p.IdleGCSeconds, 0)
	}
}

func TestIntegration_CoreCRD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreCRD_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CoreCRD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
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

func TestIntegration_CoreCRD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCRDTemplateRowCount, len(got))
}

func TestIntegration_CoreCRD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)
	applyMigration(t, pool, migDir, crdMigrationDown)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCRD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, crdMigration)

	loader := core.NewCoreContentReferenceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedCRDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
