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

const bmpdMigration = "000055_seed_bubble_mode_policy_default_templates.up.sql"
const bmpdMigrationDown = "000055_seed_bubble_mode_policy_default_templates.down.sql"

func TestIntegration_CoreBMPD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedBMPDTemplateRowCount, len(got))
}

func TestIntegration_CoreBMPD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreBMPD_FindBySlug_SingleHopShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "single-hop")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "two_tier", tmpl.TargetUseCase)
	assert.Equal(t, "balanced", tmpl.SafetyPosture)
	assert.Equal(t, 1, tmpl.MaxBubbleDepth)
	assert.True(t, tmpl.RequiresParentResolver)
	assert.False(t, tmpl.RecordsChainHistory)
	assert.False(t, tmpl.RequiresAdminReview)
}

func TestIntegration_CoreBMPD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreBMPD_LoadByUseCase_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedBMPDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "use case %q must have exactly 1 template (1:1)", uc)
	}
}

func TestIntegration_CoreBMPD_LoadBySafetyPosture(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	for _, posture := range core.SeedExpectedBMPDTemplateSafetyPostures {
		matched, err := loader.LoadBySafetyPosture(context.Background(), posture)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(matched), 1, "posture %q must have ≥1 template", posture)
	}
}

func TestIntegration_CoreBMPD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedBMPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreBMPD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewBMPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreBMPD_MaxDepthNonNegative(t *testing.T) {
	// Cross-feature invariant: PERM-003b NewBubbleModeEvaluator requires
	// maxDepth >= 0.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, t2.MaxBubbleDepth, 0, "%s max_bubble_depth", t2.Slug)
	}
}

func TestIntegration_CoreBMPD_ZeroDepthSkipsParentResolver(t *testing.T) {
	// Cross-feature invariant matching PERM-003b NewBubbleModeEvaluator:
	// max_depth=0 → parent CAN be nil; for our seed we encode that as
	// requires_parent_resolver=false. max_depth>0 → MUST require parent.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		if t2.MaxBubbleDepth == 0 {
			assert.False(t, t2.RequiresParentResolver,
				"%s depth=0 must NOT require parent resolver", t2.Slug)
		} else {
			assert.True(t, t2.RequiresParentResolver,
				"%s depth>0 MUST require parent resolver", t2.Slug)
		}
	}
}

func TestIntegration_CoreBMPD_DeepBubblingRequiresHistoryCapture(t *testing.T) {
	// Cross-row invariant: depth >= 2 → records_chain_history=true
	// (chains of 2+ bubbles obscure responsibility without audit).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		if t2.MaxBubbleDepth >= 2 {
			assert.True(t, t2.RecordsChainHistory,
				"%s depth=%d must record chain history",
				t2.Slug, t2.MaxBubbleDepth)
		}
	}
}

func TestIntegration_CoreBMPD_DepthLadderIsMonotonic(t *testing.T) {
	// Cross-row invariant: when sorted by sort_order, max_bubble_depth
	// monotonically non-decreases (curated ladder).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for i := 1; i < len(all); i++ {
		assert.GreaterOrEqual(t, all[i].MaxBubbleDepth, all[i-1].MaxBubbleDepth,
			"depth ladder must be monotonic: %s (%d) < %s (%d)",
			all[i].Slug, all[i].MaxBubbleDepth,
			all[i-1].Slug, all[i-1].MaxBubbleDepth)
	}
}

func TestIntegration_CoreBMPD_AllUseCasesInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedBMPDTemplateUseCases {
		allowed[u] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetUseCase])
	}
}

func TestIntegration_CoreBMPD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CoreBMPD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreBMPD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
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
	assert.Equal(t, "leaf-auto-deny", all[0].Slug)
}

func TestIntegration_CoreBMPD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedBMPDTemplateRowCount, len(got))
}

func TestIntegration_CoreBMPD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)
	applyMigration(t, pool, migDir, bmpdMigrationDown)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreBMPD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bmpdMigration)

	loader := core.NewCoreBubbleModePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedBMPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
