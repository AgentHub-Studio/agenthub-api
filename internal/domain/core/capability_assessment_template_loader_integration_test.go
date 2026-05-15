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

const capTplMigration = "000026_seed_capability_assessment_templates.up.sql"
const capTplMigrationDown = "000026_seed_capability_assessment_templates.down.sql"

func TestIntegration_CoreCapabilityTemplate_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCapabilityTemplateRowCount, len(got),
		"expected 8 templates after seed migration")
}

func TestIntegration_CoreCapabilityTemplate_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCapabilityTemplate_FindBySlug_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "decision-independence-monthly")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "decision_independence", tmpl.TargetDimension)
	assert.Equal(t, "monthly", tmpl.Cadence)
	assert.Equal(t, "behavior_log", tmpl.EvaluatorKind)
	assert.True(t, tmpl.IsRecommended)
	assert.False(t, tmpl.RequiresAdminReview)
	assert.Equal(t, 30, tmpl.RecommendedPeriodDays)
}

func TestIntegration_CoreCapabilityTemplate_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreCapabilityTemplate_LoadByDimension_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	got, err := loader.LoadByDimension(context.Background(), "domain_knowledge")
	require.NoError(t, err)
	assert.Equal(t, 1, len(got),
		"exactly one per-dimension template targets domain_knowledge")
	assert.Equal(t, "domain-knowledge-monthly", got[0].Slug)

	// "multi" should return holistic + onboarding + incident.
	multi, err := loader.LoadByDimension(context.Background(), "multi")
	require.NoError(t, err)
	assert.Equal(t, 3, len(multi),
		"holistic + onboarding + incident are multi-dimensional")
}

func TestIntegration_CoreCapabilityTemplate_LoadByCadence_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	weekly, err := loader.LoadByCadence(context.Background(), "weekly")
	require.NoError(t, err)
	assert.Equal(t, 1, len(weekly), "only task-throughput-weekly is weekly")

	monthly, err := loader.LoadByCadence(context.Background(), "monthly")
	require.NoError(t, err)
	assert.Equal(t, 3, len(monthly),
		"three monthly per-dimension templates: domain/decision/quality")

	quarterly, err := loader.LoadByCadence(context.Background(), "quarterly")
	require.NoError(t, err)
	assert.Equal(t, 2, len(quarterly),
		"collaboration + holistic are quarterly")

	oneShot, err := loader.LoadByCadence(context.Background(), "one_shot")
	require.NoError(t, err)
	assert.Equal(t, 1, len(oneShot), "onboarding-baseline is one_shot")

	eventDriven, err := loader.LoadByCadence(context.Background(), "event_driven")
	require.NoError(t, err)
	assert.Equal(t, 1, len(eventDriven), "incident-postmortem is event_driven")
}

func TestIntegration_CoreCapabilityTemplate_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	got := []string{}
	for _, t := range rec {
		got = append(got, t.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedCapabilityTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got,
		"DB recommended set must equal Go SeedRecommendedCapabilityTemplateSlugs")
}

func TestIntegration_CoreCapabilityTemplate_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	got := []string{}
	for _, t := range all {
		if t.RequiresAdminReview {
			got = append(got, t.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewCapabilityTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got,
		"DB admin-review set must equal Go SeedAdminReviewCapabilityTemplateSlugs")
}

func TestIntegration_CoreCapabilityTemplate_AllDimensionsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, d := range core.SeedExpectedCapabilityTemplateDimensions {
		allowed[d] = true
	}
	for _, tmpl := range all {
		assert.True(t, allowed[tmpl.TargetDimension],
			"template %q has dimension %q outside expected set",
			tmpl.Slug, tmpl.TargetDimension)
	}
}

func TestIntegration_CoreCapabilityTemplate_AllCadencesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, c := range core.SeedExpectedCapabilityTemplateCadences {
		allowed[c] = true
	}
	for _, tmpl := range all {
		assert.True(t, allowed[tmpl.Cadence],
			"template %q has cadence %q outside expected set",
			tmpl.Slug, tmpl.Cadence)
	}
}

func TestIntegration_CoreCapabilityTemplate_AllEvaluatorKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedCapabilityTemplateEvaluatorKinds {
		allowed[k] = true
	}
	for _, tmpl := range all {
		assert.True(t, allowed[tmpl.EvaluatorKind],
			"template %q has evaluator_kind %q outside expected set",
			tmpl.Slug, tmpl.EvaluatorKind)
	}
}

func TestIntegration_CoreCapabilityTemplate_DeltaAlertThresholdDefaultMatchesFutureSix(t *testing.T) {
	// FUTURE-006 classifyTrend uses ±0.05. Per-dimension monthly templates
	// keep that default to align alerts with the trend classification.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	for _, slug := range []string{
		"domain-knowledge-monthly",
		"decision-independence-monthly",
		"quality-output-monthly",
		"collaboration-quarterly",
		"holistic-capability-quarterly",
	} {
		tmpl, found, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		require.True(t, found, "missing %s", slug)
		assert.InDelta(t, 0.05, tmpl.DeltaAlertThreshold, 1e-9,
			"%s must use ±0.05 to align with FUTURE-006 classifyTrend", slug)
	}
}

func TestIntegration_CoreCapabilityTemplate_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, tmpl := range all {
		assert.False(t, seen[tmpl.Slug], "duplicate slug %q in DB", tmpl.Slug)
		seen[tmpl.Slug] = true
	}
}

func TestIntegration_CoreCapabilityTemplate_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, tmpl := range all {
		assert.GreaterOrEqual(t, len(tmpl.Description), 30,
			"template %q description must be informative (≥30 chars)", tmpl.Slug)
	}
}

func TestIntegration_CoreCapabilityTemplate_MinSampleSizeIsPositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, tmpl := range all {
		assert.Positive(t, tmpl.MinSampleSize,
			"template %q must require ≥1 observation", tmpl.Slug)
	}
}

func TestIntegration_CoreCapabilityTemplate_RecommendedPeriodMatchesCadence(t *testing.T) {
	// Sanity invariant: cadence and recommended_period_days must be
	// consistent — weekly ≤ 7, monthly ~30, quarterly ~90.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, tmpl := range all {
		switch tmpl.Cadence {
		case "weekly":
			assert.LessOrEqual(t, tmpl.RecommendedPeriodDays, 7,
				"%s weekly period must be ≤7 days", tmpl.Slug)
		case "monthly":
			assert.GreaterOrEqual(t, tmpl.RecommendedPeriodDays, 28,
				"%s monthly period must be ≥28 days", tmpl.Slug)
			assert.LessOrEqual(t, tmpl.RecommendedPeriodDays, 31,
				"%s monthly period must be ≤31 days", tmpl.Slug)
		case "quarterly":
			assert.GreaterOrEqual(t, tmpl.RecommendedPeriodDays, 89,
				"%s quarterly period must be ~90 days", tmpl.Slug)
			assert.LessOrEqual(t, tmpl.RecommendedPeriodDays, 92,
				"%s quarterly period must be ~90 days", tmpl.Slug)
		case "one_shot":
			// Onboarding window — at most a couple weeks.
			assert.LessOrEqual(t, tmpl.RecommendedPeriodDays, 30,
				"%s one_shot must observe within reasonable window", tmpl.Slug)
		case "event_driven":
			// Snapshot at incident time — short window.
			assert.LessOrEqual(t, tmpl.RecommendedPeriodDays, 7,
				"%s event_driven must observe within incident window", tmpl.Slug)
		}
	}
}

func TestIntegration_CoreCapabilityTemplate_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, all)
	for i := 1; i < len(all); i++ {
		if all[i-1].SortOrder == all[i].SortOrder {
			assert.LessOrEqual(t, all[i-1].Slug, all[i].Slug,
				"tie-broken by slug ascending")
		} else {
			assert.Less(t, all[i-1].SortOrder, all[i].SortOrder,
				"sort_order must be monotonic")
		}
	}
	assert.Equal(t, "domain-knowledge-monthly", all[0].Slug,
		"first by sort_order=10 must be domain-knowledge-monthly")
}

func TestIntegration_CoreCapabilityTemplate_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)
	// Apply twice — ON CONFLICT DO NOTHING.
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCapabilityTemplateRowCount, len(got),
		"second apply must not create duplicates")
}

func TestIntegration_CoreCapabilityTemplate_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)
	applyMigration(t, pool, migDir, capTplMigrationDown)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err, "loader must remain non-fatal after table drop")
	assert.Empty(t, got)
}

func TestIntegration_CoreCapabilityTemplate_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, capTplMigration)

	loader := core.NewCoreCapabilityAssessmentTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, tmpl := range all {
		got = append(got, tmpl.Slug)
	}
	expected := append([]string{}, core.SeedExpectedCapabilityTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got,
		"DB slug set must equal canonical SeedExpectedCapabilityTemplateSlugs")
}
