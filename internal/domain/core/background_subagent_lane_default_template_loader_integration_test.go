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

const bsldMigration = "000063_seed_background_subagent_lane_default_templates.up.sql"
const bsldMigrationDown = "000063_seed_background_subagent_lane_default_templates.down.sql"

func TestIntegration_CoreBSLD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedBSLDTemplateRowCount, len(got))
}

func TestIntegration_CoreBSLD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreBSLD_FindBySlug_QuickGlanceShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "quick-glance")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "exploration", tmpl.TypicalTaskClass)
	assert.Equal(t, 30, tmpl.TimeoutBudgetSeconds)
	assert.Equal(t, "high", tmpl.Priority)
	assert.Equal(t, 20, tmpl.MaxConcurrentPerParent)
	assert.True(t, tmpl.AutoCancelOnParentTerminate)
}

func TestIntegration_CoreBSLD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreBSLD_LoadByTaskClass_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	for _, c := range core.SeedExpectedBSLDTemplateTaskClasses {
		matched, err := loader.LoadByTaskClass(context.Background(), c)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched),
			"task_class %q must have exactly 1 lane (1:1)", c)
	}
}

func TestIntegration_CoreBSLD_LoadByPriority_HighIsQuickGlanceOnly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	high, err := loader.LoadByPriority(context.Background(), "high")
	require.NoError(t, err)
	require.Equal(t, 1, len(high))
	assert.Equal(t, "quick-glance", high[0].Slug)
}

func TestIntegration_CoreBSLD_AllAutoCancelOnParentTerminate(t *testing.T) {
	// Cross-feature invariant: orphaned background jobs waste tenant
	// budget. Every lane must default to auto-cancel.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.True(t, t2.AutoCancelOnParentTerminate,
			"%s must auto-cancel on parent terminate", t2.Slug)
	}
}

func TestIntegration_CoreBSLD_PlannerAndCoderAreSerial(t *testing.T) {
	// Cross-feature invariant: planning + write task classes must run
	// serially per parent.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	for _, slug := range core.SeedSingleConcurrencyBSLDTemplateSlugs {
		tmpl, found, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, 1, tmpl.MaxConcurrentPerParent,
			"%s must be single-concurrency", slug)
	}
}

func TestIntegration_CoreBSLD_ResearchAllowsParallelism(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "research-lane")
	require.NoError(t, err)
	require.True(t, found)
	assert.Greater(t, tmpl.MaxConcurrentPerParent, 1,
		"research-lane must allow parallel topic coverage")
}

func TestIntegration_CoreBSLD_TimeoutBudgetsInBoundedRange(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, t2.TimeoutBudgetSeconds,
			core.SeedMinBSLDTimeoutBudgetSeconds,
			"%s timeout below min", t2.Slug)
		assert.LessOrEqual(t, t2.TimeoutBudgetSeconds,
			core.SeedMaxBSLDTimeoutBudgetSeconds,
			"%s timeout above max", t2.Slug)
	}
}

func TestIntegration_CoreBSLD_WatchdogIntervalLessThanTimeout(t *testing.T) {
	// Operational invariant: watchdog must check at least once before
	// the timeout fires.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.Less(t, t2.WatchdogCheckIntervalSeconds, t2.TimeoutBudgetSeconds,
			"%s watchdog interval must be < timeout budget", t2.Slug)
		assert.Greater(t, t2.WatchdogCheckIntervalSeconds, 0,
			"%s watchdog must be positive", t2.Slug)
	}
}

func TestIntegration_CoreBSLD_PrioritiesInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedBSLDTemplatePriorities {
		allowed[p] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.Priority],
			"%s priority %q outside closed set", t2.Slug, t2.Priority)
	}
}

func TestIntegration_CoreBSLD_RetryPosturesInClosedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedBSLDTemplateRetryPostures {
		allowed[p] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.RetryPosture],
			"%s retry_posture %q outside closed set", t2.Slug, t2.RetryPosture)
	}
}

func TestIntegration_CoreBSLD_AllTaskClassesInExpectedVocabulary(t *testing.T) {
	// Cross-feature invariant: lane task_class must match SUB-002 builtin
	// task_class vocabulary so admin UI can suggest lanes by subagent role.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, c := range core.SeedExpectedBSLDTemplateTaskClasses {
		allowed[c] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TypicalTaskClass],
			"%s task_class %q outside expected vocabulary",
			t2.Slug, t2.TypicalTaskClass)
	}
}

func TestIntegration_CoreBSLD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40,
			"%s description must be substantive", t2.Slug)
	}
}

func TestIntegration_CoreBSLD_AllSlugsUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreBSLD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
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
	assert.Equal(t, "quick-glance", all[0].Slug)
}

func TestIntegration_CoreBSLD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedBSLDTemplateRowCount, len(got))
}

func TestIntegration_CoreBSLD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)
	applyMigration(t, pool, migDir, bsldMigrationDown)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreBSLD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, bsldMigration)

	loader := core.NewCoreBackgroundSubagentLaneDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedBSLDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
