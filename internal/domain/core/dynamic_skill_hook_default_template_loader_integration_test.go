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

const dshdMigration = "000042_seed_dynamic_skill_hook_default_templates.up.sql"
const dshdMigrationDown = "000042_seed_dynamic_skill_hook_default_templates.down.sql"

func TestIntegration_CoreDSHD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedDSHDTemplateRowCount, len(got))
}

func TestIntegration_CoreDSHD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreDSHD_FindBySlug_ValidateInputShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "validate-input")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "before_invocation", tmpl.TargetPhase)
	assert.Equal(t, 90, tmpl.DefaultPriority)
	assert.Equal(t, "input_validation", tmpl.TargetUseCase)
}

func TestIntegration_CoreDSHD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreDSHD_LoadByPhase_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	// before_invocation: validate-input + redact-pii = 2.
	before, err := loader.LoadByPhase(context.Background(), "before_invocation")
	require.NoError(t, err)
	assert.Equal(t, 2, len(before))

	// Each other phase has 1 template.
	for _, p := range []string{"before_tool_call", "after_tool_call", "after_invocation", "on_error"} {
		got, _ := loader.LoadByPhase(context.Background(), p)
		assert.Equal(t, 1, len(got), "phase %q must have 1 template", p)
	}
}

func TestIntegration_CoreDSHD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedDSHDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreDSHD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewDSHDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreDSHD_AllPhasesAreInEXT007Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedDSHDTemplatePhases {
		allowed[p] = true
	}
	for _, tmpl := range all {
		assert.True(t, allowed[tmpl.TargetPhase],
			"%s phase %q outside EXT-007 enum", tmpl.Slug, tmpl.TargetPhase)
	}
}

func TestIntegration_CoreDSHD_AllUseCasesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedDSHDTemplateUseCases {
		allowed[u] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetUseCase])
	}
}

func TestIntegration_CoreDSHD_AllTenantKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedDSHDTemplateTenantKinds {
		allowed[k] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.RecommendedForTenantKind])
	}
}

func TestIntegration_CoreDSHD_AllPhasesAreCovered(t *testing.T) {
	// Cross-row invariant: seed includes at least one example per
	// EXT-007 phase (vendors see a starting point for any lifecycle moment).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	covered := map[string]bool{}
	for _, p := range all {
		covered[p.TargetPhase] = true
	}
	for _, phase := range core.SeedExpectedDSHDTemplatePhases {
		assert.True(t, covered[phase],
			"phase %q must have at least one example template", phase)
	}
}

func TestIntegration_CoreDSHD_PrivacyHookHasHighPriority(t *testing.T) {
	// Cross-row invariant: redact-pii outranks routine hooks (privacy
	// fires first to scrub before downstream).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	pii, _, _ := loader.FindBySlug(context.Background(), "redact-pii")
	cost, _, _ := loader.FindBySlug(context.Background(), "cost-track")
	assert.Greater(t, pii.DefaultPriority, cost.DefaultPriority,
		"privacy hook must outrank routine cost tracking")
}

func TestIntegration_CoreDSHD_AllPriorityValuesArePositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.Positive(t, p.DefaultPriority)
	}
}

func TestIntegration_CoreDSHD_HandlerPatternsAreNonEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.NotEmpty(t, p.HandlerPattern)
	}
}

func TestIntegration_CoreDSHD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreDSHD_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CoreDSHD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
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
	assert.Equal(t, "validate-input", all[0].Slug)
}

func TestIntegration_CoreDSHD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedDSHDTemplateRowCount, len(got))
}

func TestIntegration_CoreDSHD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)
	applyMigration(t, pool, migDir, dshdMigrationDown)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreDSHD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, dshdMigration)

	loader := core.NewCoreDynamicSkillHookDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedDSHDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
