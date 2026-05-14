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

const phdMigration = "000047_seed_permission_hook_default_templates.up.sql"
const phdMigrationDown = "000047_seed_permission_hook_default_templates.down.sql"

func TestIntegration_CorePHD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPHDTemplateRowCount, len(got))
}

func TestIntegration_CorePHD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePHD_FindBySlug_OncallShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "oncall-bypass-allow")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "before_evaluate", tmpl.TargetPhase)
	assert.Equal(t, "override_allow", tmpl.TargetOutcome)
	assert.Equal(t, "oncall_escalation", tmpl.TargetUseCase)
	assert.True(t, tmpl.RequiresAdminReview)
}

func TestIntegration_CorePHD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePHD_LoadByPhase_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	// before_evaluate: oncall + business + rate-limit = 3
	before, err := loader.LoadByPhase(context.Background(), "before_evaluate")
	require.NoError(t, err)
	assert.Equal(t, 3, len(before))
	// after_evaluate: pii + audit-observer + sensitive-customer = 3
	after, _ := loader.LoadByPhase(context.Background(), "after_evaluate")
	assert.Equal(t, 3, len(after))
}

func TestIntegration_CorePHD_LoadByOutcome_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	allow, err := loader.LoadByOutcome(context.Background(), "override_allow")
	require.NoError(t, err)
	assert.Equal(t, 1, len(allow))
	deny, _ := loader.LoadByOutcome(context.Background(), "override_deny")
	assert.Equal(t, 3, len(deny))
	confirm, _ := loader.LoadByOutcome(context.Background(), "override_confirm")
	assert.Equal(t, 1, len(confirm))
	cont, _ := loader.LoadByOutcome(context.Background(), "continue")
	assert.Equal(t, 1, len(cont))
}

func TestIntegration_CorePHD_LoadByUseCase_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedPHDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "use case %q must have exactly 1 template", uc)
	}
}

func TestIntegration_CorePHD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedPHDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePHD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewPHDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePHD_AllPhasesAreInPERM005Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedPHDTemplatePhases {
		allowed[p] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetPhase],
			"%s phase %q outside PERM-005 enum", t2.Slug, t2.TargetPhase)
	}
}

func TestIntegration_CorePHD_AllOutcomesAreInPERM005Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, o := range core.SeedExpectedPHDTemplateOutcomes {
		allowed[o] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetOutcome],
			"%s outcome %q outside PERM-005 enum", t2.Slug, t2.TargetOutcome)
	}
}

func TestIntegration_CorePHD_AllUseCasesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedPHDTemplateUseCases {
		allowed[u] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetUseCase])
	}
}

func TestIntegration_CorePHD_BothPhasesHaveExamples(t *testing.T) {
	// Cross-row invariant: every PERM-005 phase has ≥1 template.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	covered := map[string]bool{}
	for _, t2 := range all {
		covered[t2.TargetPhase] = true
	}
	for _, phase := range core.SeedExpectedPHDTemplatePhases {
		assert.True(t, covered[phase], "phase %q must have ≥1 example", phase)
	}
}

func TestIntegration_CorePHD_AllFourOutcomesHaveExamples(t *testing.T) {
	// Cross-row invariant: every PERM-005 outcome has ≥1 template.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	covered := map[string]bool{}
	for _, t2 := range all {
		covered[t2.TargetOutcome] = true
	}
	for _, outcome := range core.SeedExpectedPHDTemplateOutcomes {
		assert.True(t, covered[outcome], "outcome %q must have ≥1 example", outcome)
	}
}

func TestIntegration_CorePHD_AuditObserverIsTheOnlyContinueOutcome(t *testing.T) {
	// Cross-row invariant: only audit-trace-observer-continue uses
	// the "continue" outcome (every other template materially overrides).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	matched, err := loader.LoadByOutcome(context.Background(), "continue")
	require.NoError(t, err)
	require.Equal(t, 1, len(matched))
	assert.Equal(t, "audit-trace-observer-continue", matched[0].Slug)
	assert.False(t, matched[0].RequiresAdminReview,
		"the pure observer should not require admin review")
}

func TestIntegration_CorePHD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 30, "%s description", t2.Slug)
	}
}

func TestIntegration_CorePHD_PrioritiesArePositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.Greater(t, t2.DefaultPriority, 0, "%s priority", t2.Slug)
	}
}

func TestIntegration_CorePHD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CorePHD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
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
	assert.Equal(t, "oncall-bypass-allow", all[0].Slug)
}

func TestIntegration_CorePHD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPHDTemplateRowCount, len(got))
}

func TestIntegration_CorePHD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)
	applyMigration(t, pool, migDir, phdMigrationDown)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePHD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, phdMigration)

	loader := core.NewCorePermissionHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedPHDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
