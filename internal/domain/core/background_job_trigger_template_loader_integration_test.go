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

const bgJobMigration = "000028_seed_background_job_trigger_templates.up.sql"
const bgJobMigrationDown = "000028_seed_background_job_trigger_templates.down.sql"

func TestIntegration_CoreBgJobTriggerTemplate_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedBgJobTriggerTemplateRowCount, len(got))
}

func TestIntegration_CoreBgJobTriggerTemplate_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreBgJobTriggerTemplate_FindBySlug_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "on-cost-budget-exceeded")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "threshold_crossed", tmpl.TriggerKind)
	assert.True(t, tmpl.RequiresAdminReview)
	assert.Equal(t, "alert_admin", tmpl.OnFailureAction)
	cfg, err := tmpl.TriggerConfig()
	require.NoError(t, err)
	assert.Equal(t, "llm.cost.daily_usd", cfg["metric"])
}

func TestIntegration_CoreBgJobTriggerTemplate_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreBgJobTriggerTemplate_LoadByTriggerKind_TwoPerKind(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	for _, kind := range core.SeedExpectedBgJobTriggerTemplateKinds {
		got, err := loader.LoadByTriggerKind(context.Background(), kind)
		require.NoError(t, err)
		assert.Equal(t, 2, len(got),
			"kind %q must have exactly 2 templates (8 / 4 kinds)", kind)
	}
}

func TestIntegration_CoreBgJobTriggerTemplate_LoadByOnFailureAction_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	retry, err := loader.LoadByOnFailureAction(context.Background(), "retry_with_backoff")
	require.NoError(t, err)
	assert.Equal(t, 3, len(retry),
		"hourly-health-check + on-kb-document-uploaded + on-user-inactive-30-days")

	alert, err := loader.LoadByOnFailureAction(context.Background(), "alert_admin")
	require.NoError(t, err)
	assert.Equal(t, 5, len(alert))
}

func TestIntegration_CoreBgJobTriggerTemplate_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t := range rec {
		got = append(got, t.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedBgJobTriggerTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreBgJobTriggerTemplate_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t := range all {
		if t.RequiresAdminReview {
			got = append(got, t.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewBgJobTriggerTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreBgJobTriggerTemplate_AllKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedBgJobTriggerTemplateKinds {
		allowed[k] = true
	}
	for _, tmpl := range all {
		assert.True(t, allowed[tmpl.TriggerKind],
			"template %q has kind %q outside expected set",
			tmpl.Slug, tmpl.TriggerKind)
	}
}

func TestIntegration_CoreBgJobTriggerTemplate_AllOnFailureActionsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, a := range core.SeedExpectedBgJobTriggerTemplateOnFailureActions {
		allowed[a] = true
	}
	for _, tmpl := range all {
		assert.True(t, allowed[tmpl.OnFailureAction],
			"template %q has action %q outside expected set",
			tmpl.Slug, tmpl.OnFailureAction)
	}
}

func TestIntegration_CoreBgJobTriggerTemplate_TriggerConfigParsesCleanly(t *testing.T) {
	// Cross-row invariant: every template's trigger_config_json must
	// parse as valid JSON (catches malformed inserts at integration time).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, tmpl := range all {
		cfg, err := tmpl.TriggerConfig()
		require.NoError(t, err, "template %q config must parse", tmpl.Slug)
		require.NotEmpty(t, cfg, "template %q config must not be empty", tmpl.Slug)
	}
}

func TestIntegration_CoreBgJobTriggerTemplate_TriggerConfigShapeMatchesKind(t *testing.T) {
	// Each trigger_kind has a required key in its config:
	//   schedule_cron     → "cron"
	//   event_arrived     → "event_name"
	//   threshold_crossed → "metric" + "threshold"
	//   absence_timeout   → "absence_seconds"
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, tmpl := range all {
		cfg, _ := tmpl.TriggerConfig()
		switch tmpl.TriggerKind {
		case "schedule_cron":
			assert.Contains(t, cfg, "cron",
				"template %q kind=schedule_cron must have cron key", tmpl.Slug)
		case "event_arrived":
			assert.Contains(t, cfg, "event_name",
				"template %q kind=event_arrived must have event_name key", tmpl.Slug)
		case "threshold_crossed":
			assert.Contains(t, cfg, "metric",
				"template %q kind=threshold_crossed must have metric key", tmpl.Slug)
			assert.Contains(t, cfg, "threshold")
		case "absence_timeout":
			assert.Contains(t, cfg, "absence_seconds",
				"template %q kind=absence_timeout must have absence_seconds key", tmpl.Slug)
		}
	}
}

func TestIntegration_CoreBgJobTriggerTemplate_TargetWorkflowReferencesExistingWorkflow(t *testing.T) {
	// Cross-table FK at app level: every template's target_workflow_slug
	// must reference an existing seeded workflow_template.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	bgLoader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	wfLoader := core.NewCoreWorkflowTemplateLoader(pool)

	bgs, err := bgLoader.LoadAll(context.Background())
	require.NoError(t, err)
	wfs, err := wfLoader.LoadAll(context.Background())
	require.NoError(t, err)

	wfSlugs := map[string]bool{}
	for _, wf := range wfs {
		wfSlugs[wf.Slug] = true
	}
	for _, bg := range bgs {
		assert.True(t, wfSlugs[bg.TargetWorkflowSlug],
			"bg-job template %q references missing workflow %q",
			bg.Slug, bg.TargetWorkflowSlug)
	}
}

func TestIntegration_CoreBgJobTriggerTemplate_RateLimitsAreSensible(t *testing.T) {
	// Sanity: max_runs_per_day, claim_timeout_seconds,
	// max_consecutive_failures must all be positive (defensive against
	// migration typos that would result in trigger never firing or never
	// failing-out).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, tmpl := range all {
		assert.Positive(t, tmpl.MaxRunsPerDay, "%s max_runs_per_day", tmpl.Slug)
		assert.Positive(t, tmpl.ClaimTimeoutSeconds, "%s claim_timeout_seconds", tmpl.Slug)
		assert.Positive(t, tmpl.MaxConsecutiveFailures, "%s max_consecutive_failures", tmpl.Slug)
	}
}

func TestIntegration_CoreBgJobTriggerTemplate_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, tmpl := range all {
		assert.False(t, seen[tmpl.Slug])
		seen[tmpl.Slug] = true
	}
}

func TestIntegration_CoreBgJobTriggerTemplate_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, tmpl := range all {
		assert.GreaterOrEqual(t, len(tmpl.Description), 30,
			"template %q description must be informative (≥30 chars)", tmpl.Slug)
	}
}

func TestIntegration_CoreBgJobTriggerTemplate_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
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
	assert.Equal(t, "daily-digest-summary", all[0].Slug)
}

func TestIntegration_CoreBgJobTriggerTemplate_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedBgJobTriggerTemplateRowCount, len(got))
}

func TestIntegration_CoreBgJobTriggerTemplate_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)
	applyMigration(t, pool, migDir, bgJobMigrationDown)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreBgJobTriggerTemplate_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bgJobMigration)

	loader := core.NewCoreBackgroundJobTriggerTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, tmpl := range all {
		got = append(got, tmpl.Slug)
	}
	expected := append([]string{}, core.SeedExpectedBgJobTriggerTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
