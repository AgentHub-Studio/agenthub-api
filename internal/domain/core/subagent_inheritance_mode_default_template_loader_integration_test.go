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

const simdMigration = "000050_seed_subagent_inheritance_mode_default_templates.up.sql"
const simdMigrationDown = "000050_seed_subagent_inheritance_mode_default_templates.down.sql"

func TestIntegration_CoreSIMD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSIMDTemplateRowCount, len(got))
}

func TestIntegration_CoreSIMD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSIMD_FindBySlug_ExtendShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "extend-parent-rights")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "inherit_all", tmpl.TargetInheritanceMode)
	assert.Equal(t, "helper_extension", tmpl.TargetUseCase)
	assert.False(t, tmpl.RequiresAdminReview)
	signals, err := tmpl.ExpectedAuditSignals()
	require.NoError(t, err)
	assert.Contains(t, signals, "AddedAllows")
}

func TestIntegration_CoreSIMD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreSIMD_LoadByMode_OneToOneMapping(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	for _, mode := range core.SeedExpectedSIMDTemplateModes {
		matched, err := loader.LoadByInheritanceMode(context.Background(), mode)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "mode %q must have exactly 1 template (1:1)", mode)
	}
}

func TestIntegration_CoreSIMD_LoadByRiskPosture(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	for _, posture := range core.SeedExpectedSIMDTemplateRiskPostures {
		matched, err := loader.LoadByRiskPosture(context.Background(), posture)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "posture %q must have exactly 1 template", posture)
	}
}

func TestIntegration_CoreSIMD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedSIMDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreSIMD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewSIMDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreSIMD_AllModesAreInSUB006Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, m := range core.SeedExpectedSIMDTemplateModes {
		allowed[m] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetInheritanceMode],
			"%s mode %q outside SUB-006 enum", t2.Slug, t2.TargetInheritanceMode)
	}
}

func TestIntegration_CoreSIMD_AllAuditSignalsAreInResolutionStruct(t *testing.T) {
	// Cross-feature invariant: every expected_audit_signal must be a
	// real field on SubagentPermissionResolution (AddedAllows /
	// AddedDenies / DroppedAllows / ReasonSummary).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowedSignals := map[string]bool{
		"AddedAllows": true, "AddedDenies": true,
		"DroppedAllows": true, "ReasonSummary": true,
	}
	for _, t2 := range all {
		signals, err := t2.ExpectedAuditSignals()
		require.NoError(t, err)
		for _, s := range signals {
			assert.True(t, allowedSignals[s],
				"%s signal %q not in SUB-006 Resolution struct", t2.Slug, s)
		}
	}
}

func TestIntegration_CoreSIMD_IntersectModeTracksDroppedAllows(t *testing.T) {
	// Cross-row invariant: merge_intersect mode's audit signals must
	// include DroppedAllows (the unique audit field for that mode).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "audit-strict-intersect")
	require.NoError(t, err)
	signals, err := tmpl.ExpectedAuditSignals()
	require.NoError(t, err)
	assert.Contains(t, signals, "DroppedAllows")
}

func TestIntegration_CoreSIMD_OverrideReplaceTracksReasonSummary(t *testing.T) {
	// Cross-row invariant: override_replace explicitly de-couples
	// rules — auditor needs the ReasonSummary signal to know why.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "isolated-decoupled")
	require.NoError(t, err)
	signals, err := tmpl.ExpectedAuditSignals()
	require.NoError(t, err)
	assert.Contains(t, signals, "ReasonSummary")
}

func TestIntegration_CoreSIMD_ExtendModeTracksBothAddedFields(t *testing.T) {
	// Cross-row invariant: inherit_all is the "union extension" mode —
	// both AddedAllows and AddedDenies are informative.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "extend-parent-rights")
	require.NoError(t, err)
	signals, err := tmpl.ExpectedAuditSignals()
	require.NoError(t, err)
	assert.Contains(t, signals, "AddedAllows")
	assert.Contains(t, signals, "AddedDenies")
}

func TestIntegration_CoreSIMD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CoreSIMD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreSIMD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
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
	assert.Equal(t, "extend-parent-rights", all[0].Slug)
}

func TestIntegration_CoreSIMD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedSIMDTemplateRowCount, len(got))
}

func TestIntegration_CoreSIMD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)
	applyMigration(t, pool, migDir, simdMigrationDown)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSIMD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, simdMigration)

	loader := core.NewCoreSubagentInheritanceModeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedSIMDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
