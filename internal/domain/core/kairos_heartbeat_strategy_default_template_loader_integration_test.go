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

const kairosStratMigration = "000077_seed_kairos_heartbeat_strategy_default_templates.up.sql"
const kairosStratMigrationDown = "000077_seed_kairos_heartbeat_strategy_default_templates.down.sql"

func TestIntegration_CoreKairosStrategy_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedKairosHeartbeatStrategyRowCount, len(got))
}

func TestIntegration_CoreKairosStrategy_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreKairosStrategy_FindBySlug_OnDemandIsDefault(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "on-demand")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "on-demand", tmpl.ScheduleType)
	assert.False(t, tmpl.IsProactive, "on-demand must not be proactive")
	assert.False(t, tmpl.IsBackground)
	assert.Equal(t, 0, tmpl.TickIntervalSeconds)
}

func TestIntegration_CoreKairosStrategy_FindBySlug_Heartbeat5mIsMinimum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "heartbeat-5m")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "heartbeat", tmpl.ScheduleType)
	assert.Equal(t, 300, tmpl.TickIntervalSeconds,
		"5 minutes is the economic minimum (§11.6 prompt cache TTL)")
	assert.True(t, tmpl.IsProactive)
	assert.True(t, tmpl.IsBackground)
	assert.Equal(t, "kairos-standard", tmpl.SourceKairosPattern)
}

func TestIntegration_CoreKairosStrategy_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nonexistent-slug")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreKairosStrategy_LoadByScheduleType_Heartbeat(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadByScheduleType(context.Background(), "heartbeat")
	require.NoError(t, err)
	// heartbeat-5m, heartbeat-15m, heartbeat-hourly
	assert.Equal(t, 3, len(got), "3 heartbeat presets expected")
	for _, tmpl := range got {
		assert.Equal(t, "heartbeat", tmpl.ScheduleType)
	}
}

func TestIntegration_CoreKairosStrategy_LoadByScheduleType_Cron(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadByScheduleType(context.Background(), "cron")
	require.NoError(t, err)
	// daily-digest, weekly-report
	assert.Equal(t, 2, len(got), "2 cron presets expected")
}

func TestIntegration_CoreKairosStrategy_LoadProactive_ExcludesOnDemand(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadProactive(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 5, len(got), "5 proactive presets (all except on-demand)")
	for _, tmpl := range got {
		assert.True(t, tmpl.IsProactive)
		assert.NotEqual(t, "on-demand", tmpl.Slug,
			"on-demand must not appear in proactive list")
	}
}

func TestIntegration_CoreKairosStrategy_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedKairosHeartbeatStrategyRowCount, len(got))
}

func TestIntegration_CoreKairosStrategy_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)
	applyMigration(t, pool, migDir, kairosStratMigrationDown)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreKairosStrategy_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, t2 := range all {
		dbSlugs = append(dbSlugs, t2.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedKairosHeartbeatStrategySlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}

func TestIntegration_CoreKairosStrategy_SortOrderAscending(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, all)

	for i := 1; i < len(all); i++ {
		assert.LessOrEqual(t, all[i-1].SortOrder, all[i].SortOrder)
	}
	assert.Equal(t, "on-demand", all[0].Slug, "on-demand sorts first")
}

func TestIntegration_CoreKairosStrategy_AllScheduleTypesInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	allowed := map[string]bool{}
	for _, s := range core.SeedKairosScheduleTypes {
		allowed[s] = true
	}
	for _, tmpl := range all {
		assert.True(t, allowed[tmpl.ScheduleType],
			"template %q has unknown schedule_type %q", tmpl.Slug, tmpl.ScheduleType)
	}
}

func TestIntegration_CoreKairosStrategy_RecommendedForIsNonEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range all {
		assert.NotEmpty(t, tmpl.RecommendedFor,
			"template %q must have at least one recommended_for entry", tmpl.Slug)
	}
}

func TestIntegration_CoreKairosStrategy_HeartbeatStrategiesHavePositiveTickInterval(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, tmpl := range all {
		if tmpl.ScheduleType == "heartbeat" {
			assert.Greater(t, tmpl.TickIntervalSeconds, 0,
				"heartbeat template %q must have positive tick interval", tmpl.Slug)
		}
	}
}

func TestIntegration_CoreKairosStrategy_WeeklyReportHaeLongestInterval(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, kairosStratMigration)

	loader := core.NewCoreKairosHeartbeatStrategyDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "weekly-report")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 604800, tmpl.TickIntervalSeconds, "weekly = 7 days = 604800s")
	assert.Equal(t, "scheduled-weekly", tmpl.SourceKairosPattern)
}
