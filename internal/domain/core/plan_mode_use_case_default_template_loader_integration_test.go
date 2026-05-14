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

const pmudMigration = "000054_seed_plan_mode_use_case_default_templates.up.sql"
const pmudMigrationDown = "000054_seed_plan_mode_use_case_default_templates.down.sql"

func TestIntegration_CorePMUD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPMUDTemplateRowCount, len(got))
}

func TestIntegration_CorePMUD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePMUD_FindBySlug_DryRunShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "dry-run-preview")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "dry_run", tmpl.TargetUseCase)
	assert.Equal(t, "permissive", tmpl.SafetyPosture)
	assert.False(t, tmpl.RequiresRationale)
	assert.Equal(t, 3, tmpl.AutoApproveThreshold)
	tools, err := tmpl.ExtraReadOnlyTools()
	require.NoError(t, err)
	assert.Contains(t, tools, "document_search")
}

func TestIntegration_CorePMUD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePMUD_LoadByUseCase_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedPMUDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "use case %q must have exactly 1 template (1:1)", uc)
	}
}

func TestIntegration_CorePMUD_LoadBySafetyPosture(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	for _, posture := range core.SeedExpectedPMUDTemplateSafetyPostures {
		matched, err := loader.LoadBySafetyPosture(context.Background(), posture)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(matched), 1, "posture %q must have ≥1 template", posture)
	}
}

func TestIntegration_CorePMUD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedPMUDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePMUD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewPMUDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePMUD_AllUseCasesInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedPMUDTemplateUseCases {
		allowed[u] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetUseCase])
	}
}

func TestIntegration_CorePMUD_NonNegativeBoundedFields(t *testing.T) {
	// Cross-feature invariant: max_planned_actions + auto_approve_threshold
	// must be >= 0 (matches PERM-003a PlanSession assumptions).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, t2.MaxPlannedActions, 0, "%s max_planned_actions", t2.Slug)
		assert.GreaterOrEqual(t, t2.AutoApproveThreshold, 0, "%s auto_approve_threshold", t2.Slug)
	}
}

func TestIntegration_CorePMUD_StrictPosturesDisableAutoApprove(t *testing.T) {
	// Cross-row invariant: strict posture → auto_approve_threshold=0
	// (no silent approve for high-risk workflows).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	strict, err := loader.LoadBySafetyPosture(context.Background(), "strict")
	require.NoError(t, err)
	for _, t2 := range strict {
		assert.Equal(t, 0, t2.AutoApproveThreshold,
			"%s (strict) must have auto_approve_threshold=0", t2.Slug)
	}
}

func TestIntegration_CorePMUD_OnlyDryRunSkipsRationale(t *testing.T) {
	// Cross-row invariant: 4 of 5 templates require rationale; only
	// dry_run skips it (exploratory, no audit needed).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	rationaleRequiredCount := 0
	for _, t2 := range all {
		if t2.RequiresRationale {
			rationaleRequiredCount++
			assert.NotEqual(t, "dry_run", t2.TargetUseCase,
				"%s dry_run should NOT require rationale", t2.Slug)
		}
	}
	assert.Equal(t, 4, rationaleRequiredCount)
}

func TestIntegration_CorePMUD_MultiStepRefactorHasLargestMaxActions(t *testing.T) {
	// Cross-row priority: multi_step_refactor has the largest cap.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	refactor, _, err := loader.FindBySlug(context.Background(), "multi-step-refactor")
	require.NoError(t, err)
	for _, t2 := range all {
		if t2.Slug == "multi-step-refactor" {
			continue
		}
		// 0 means unlimited; treat as larger.
		if t2.MaxPlannedActions == 0 {
			continue
		}
		assert.GreaterOrEqual(t, refactor.MaxPlannedActions, t2.MaxPlannedActions,
			"multi-step-refactor (%d) must be >= %s (%d)",
			refactor.MaxPlannedActions, t2.Slug, t2.MaxPlannedActions)
	}
}

func TestIntegration_CorePMUD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CorePMUD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CorePMUD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
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
	assert.Equal(t, "dry-run-preview", all[0].Slug)
}

func TestIntegration_CorePMUD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPMUDTemplateRowCount, len(got))
}

func TestIntegration_CorePMUD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)
	applyMigration(t, pool, migDir, pmudMigrationDown)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePMUD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pmudMigration)

	loader := core.NewCorePlanModeUseCaseDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedPMUDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
