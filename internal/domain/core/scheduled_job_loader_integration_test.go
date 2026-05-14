//go:build integration

package core_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

func TestIntegration_CoreSchedJob_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedExpectedScheduledJobTemplateSlugs), len(got))
}

func TestIntegration_CoreSchedJob_LoadAll_ReturnsEmptyWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreSchedJob_FindBySlug_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "daily-morning-briefing")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "reporting", tmpl.JobKind)
	assert.Equal(t, "0 9 * * *", tmpl.CronExpression)
	assert.True(t, tmpl.IsRecommended)
}

func TestIntegration_CoreSchedJob_LoadByJobKind_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadByJobKind(context.Background(), "compliance")
	require.NoError(t, err)
	assert.Equal(t, 2, len(got))
}

func TestIntegration_CoreSchedJob_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)

	dbRec := []string{}
	for _, tmpl := range got {
		dbRec = append(dbRec, tmpl.Slug)
	}
	sort.Strings(dbRec)
	expected := append([]string{}, core.SeedRecommendedScheduledJobTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbRec)
}

func TestIntegration_CoreSchedJob_AdminApprovalSetMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbAdmin := []string{}
	for _, tmpl := range got {
		if tmpl.RequiresAdminApproval {
			dbAdmin = append(dbAdmin, tmpl.Slug)
		}
	}
	sort.Strings(dbAdmin)
	expected := append([]string{}, core.SeedAdminApprovalScheduledJobTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbAdmin)
}

func TestIntegration_CoreSchedJob_AllJobKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedScheduledJobTemplateKinds {
		allowed[k] = true
	}
	for _, tmpl := range got {
		assert.True(t, allowed[tmpl.JobKind])
	}
}

func TestIntegration_CoreSchedJob_TargetWorkflowSlugReferencesWorkflowCatalog(t *testing.T) {
	// Cross-table integrity: every target_workflow_slug must reference
	// an existing ah_core.workflow_template.slug.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000020_seed_workflow_templates.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	jobLoader := core.NewCoreScheduledJobTemplateLoader(pool)
	workflowLoader := core.NewCoreWorkflowTemplateLoader(pool)

	jobs, _ := jobLoader.LoadAll(context.Background())
	workflows, _ := workflowLoader.LoadAll(context.Background())

	workflowSlugs := map[string]bool{}
	for _, w := range workflows {
		workflowSlugs[w.Slug] = true
	}
	for _, j := range jobs {
		assert.True(t, workflowSlugs[j.TargetWorkflowSlug],
			"job %q references workflow %q that does not exist",
			j.Slug, j.TargetWorkflowSlug)
	}
}

func TestIntegration_CoreSchedJob_AllCronExpressionsAreFiveOrSixFields(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		fields := strings.Fields(tmpl.CronExpression)
		// Standard cron is 5 fields; some implementations support 6 (with seconds).
		assert.True(t, len(fields) == 5 || len(fields) == 6,
			"job %q cron %q must be 5 or 6 fields, got %d",
			tmpl.Slug, tmpl.CronExpression, len(fields))
	}
}

func TestIntegration_CoreSchedJob_AllTimezonesArePopulated(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.NotEmpty(t, tmpl.Timezone, "timezone required (default UTC)")
	}
}

func TestIntegration_CoreSchedJob_AllCostsArePositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, tmpl.EstimatedCostPerRunUSD, 0.0)
	}
}

func TestIntegration_CoreSchedJob_MaxConcurrentRunsDefaultsToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, tmpl.MaxConcurrentRuns, 1,
			"job %q max_concurrent_runs must be ≥1 (serial by default)", tmpl.Slug)
	}
}

func TestIntegration_CoreSchedJob_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	seen := map[string]bool{}
	for _, tmpl := range got {
		assert.False(t, seen[tmpl.Slug])
		seen[tmpl.Slug] = true
	}
}

func TestIntegration_CoreSchedJob_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range got {
		assert.GreaterOrEqual(t, len(tmpl.Description), 30)
	}
}

func TestIntegration_CoreSchedJob_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, got)

	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.down.sql")
	gotAfterDown, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, gotAfterDown)
}

func TestIntegration_CoreSchedJob_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, "000023_seed_scheduled_job_templates.up.sql")

	loader := core.NewCoreScheduledJobTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, tmpl := range got {
		dbSlugs = append(dbSlugs, tmpl.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedScheduledJobTemplateSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}
