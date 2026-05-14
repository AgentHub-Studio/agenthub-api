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

const ppsdMigration = "000046_seed_permission_prefilter_stance_default_templates.up.sql"
const ppsdMigrationDown = "000046_seed_permission_prefilter_stance_default_templates.down.sql"

func TestIntegration_CorePPSD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPPSDTemplateRowCount, len(got))
}

func TestIntegration_CorePPSD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePPSD_FindBySlug_InteractiveShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "interactive-default")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "show_confirm", tmpl.TargetStance)
	assert.Equal(t, "interactive_chat", tmpl.TargetUseCase)
	assert.False(t, tmpl.RequiresAdminReview)
	denies, err := tmpl.BaselineDenyPatterns()
	require.NoError(t, err)
	assert.Contains(t, denies, "Bash(rm -rf)")
	confirms, err := tmpl.BaselineConfirmPatterns()
	require.NoError(t, err)
	assert.Contains(t, confirms, "Write")
}

func TestIntegration_CorePPSD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePPSD_LoadByStance_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	// show_confirm: interactive + audit-strict = 2.
	show, err := loader.LoadByStance(context.Background(), "show_confirm")
	require.NoError(t, err)
	assert.Equal(t, 2, len(show))

	// hide_confirm: unattended + lockdown = 2.
	hide, _ := loader.LoadByStance(context.Background(), "hide_confirm")
	assert.Equal(t, 2, len(hide))
}

func TestIntegration_CorePPSD_LoadByUseCase_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	for _, uc := range core.SeedExpectedPPSDTemplateUseCases {
		matched, err := loader.LoadByUseCase(context.Background(), uc)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "use case %q must have exactly 1 template (1:1 mapping)", uc)
	}
}

func TestIntegration_CorePPSD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedPPSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePPSD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewPPSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePPSD_AllStancesAreInPERM004Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedPPSDTemplateStances {
		allowed[s] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetStance],
			"%s stance %q outside PERM-004 enum", t2.Slug, t2.TargetStance)
	}
}

func TestIntegration_CorePPSD_AllUseCasesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedPPSDTemplateUseCases {
		allowed[u] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetUseCase])
	}
}

func TestIntegration_CorePPSD_AllStancesHaveExample(t *testing.T) {
	// Cross-row invariant: every PERM-004 stance has ≥1 template.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	covered := map[string]bool{}
	for _, t2 := range all {
		covered[t2.TargetStance] = true
	}
	for _, stance := range core.SeedExpectedPPSDTemplateStances {
		assert.True(t, covered[stance], "stance %q must have ≥1 example", stance)
	}
}

func TestIntegration_CorePPSD_LockdownHasEmptyConfirmPatterns(t *testing.T) {
	// Cross-row invariant: lockdown denies every mutating tool, so no
	// tool ever reaches the confirm tier — confirm patterns must be empty.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "lockdown-readonly")
	require.NoError(t, err)
	confirms, err := tmpl.BaselineConfirmPatterns()
	require.NoError(t, err)
	assert.Empty(t, confirms)
}

func TestIntegration_CorePPSD_AllTemplatesHaveBaselineDenyPatterns(t *testing.T) {
	// Cross-row invariant: every template ships with at least one
	// baseline deny (admins do not start from scratch).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		denies, err := t2.BaselineDenyPatterns()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(denies), 1, "%s baseline_deny_patterns empty", t2.Slug)
	}
}

func TestIntegration_CorePPSD_InteractiveAndAuditUseShowConfirm(t *testing.T) {
	// Cross-row invariant: templates intended for human-in-the-loop
	// use cases (interactive, audit) must use show_confirm stance.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	for _, slug := range []string{"interactive-default", "audit-strict-trace"} {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		assert.Equal(t, "show_confirm", tmpl.TargetStance, "%s should be show_confirm", slug)
	}
}

func TestIntegration_CorePPSD_BatchAndLockdownUseHideConfirm(t *testing.T) {
	// Cross-row invariant: templates intended for unattended/restricted
	// use cases must use hide_confirm stance.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	for _, slug := range []string{"unattended-batch", "lockdown-readonly"} {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		assert.Equal(t, "hide_confirm", tmpl.TargetStance, "%s should be hide_confirm", slug)
	}
}

func TestIntegration_CorePPSD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 30, "%s description", t2.Slug)
	}
}

func TestIntegration_CorePPSD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CorePPSD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
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
	assert.Equal(t, "interactive-default", all[0].Slug)
}

func TestIntegration_CorePPSD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPPSDTemplateRowCount, len(got))
}

func TestIntegration_CorePPSD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)
	applyMigration(t, pool, migDir, ppsdMigrationDown)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePPSD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ppsdMigration)

	loader := core.NewCorePermissionPrefilterStanceDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedPPSDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
