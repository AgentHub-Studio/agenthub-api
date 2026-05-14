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

const permModeMigration = "000079_seed_permission_mode_default_templates.up.sql"
const permModeMigrationDown = "000079_seed_permission_mode_default_templates.down.sql"

func TestIntegration_CorePermissionMode_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPermissionModeRowCount, len(got))
}

func TestIntegration_CorePermissionMode_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePermissionMode_FindBySlug_DefaultHasSafetyScore80(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	mode, found, err := loader.FindBySlug(context.Background(), "default")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 80, mode.SafetyScore)
	assert.Equal(t, 1, mode.GradientIndex)
	assert.True(t, mode.RequiresConfirmation)
	assert.False(t, mode.AutoAcceptsEdits)
}

func TestIntegration_CorePermissionMode_FindBySlug_PlanIsSafest(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	mode, found, err := loader.FindBySlug(context.Background(), "plan")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 100, mode.SafetyScore)
	assert.Equal(t, 0, mode.GradientIndex)
	assert.True(t, mode.RequiresConfirmation)
	assert.False(t, mode.AllowsBackgroundRun)
	assert.False(t, mode.AllowsToolBypass)
}

func TestIntegration_CorePermissionMode_FindBySlug_BypassIsMostAutonomous(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	mode, found, err := loader.FindBySlug(context.Background(), "bypassPermissions")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 0, mode.SafetyScore)
	assert.Equal(t, 4, mode.GradientIndex)
	assert.True(t, mode.AllowsToolBypass)
	assert.True(t, mode.AllowsBackgroundRun)
	assert.True(t, mode.AutoAcceptsEdits)
}

func TestIntegration_CorePermissionMode_FindBySlug_AutoAllowsBackground(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	mode, found, err := loader.FindBySlug(context.Background(), "auto")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, mode.AllowsBackgroundRun, "auto mode must be KAIROS-compatible")
	assert.False(t, mode.AllowsToolBypass, "auto must not bypass tool permissions")
	assert.Equal(t, 40, mode.SafetyScore)
}

func TestIntegration_CorePermissionMode_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePermissionMode_LoadBackgroundCapable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	got, err := loader.LoadBackgroundCapable(context.Background())
	require.NoError(t, err)
	// auto + bypassPermissions
	assert.Equal(t, 2, len(got), "2 background-capable modes expected")
	for _, m := range got {
		assert.True(t, m.AllowsBackgroundRun)
	}
}

func TestIntegration_CorePermissionMode_LoadModesRequiringConfirmation(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	got, err := loader.LoadModesRequiringConfirmation(context.Background())
	require.NoError(t, err)
	// plan + default
	assert.Equal(t, 2, len(got), "2 confirmation-required modes expected")
	for _, m := range got {
		assert.True(t, m.RequiresConfirmation)
	}
}

func TestIntegration_CorePermissionMode_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)
	applyMigration(t, pool, migDir, permModeMigration)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPermissionModeRowCount, len(got))
}

func TestIntegration_CorePermissionMode_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)
	applyMigration(t, pool, migDir, permModeMigrationDown)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePermissionMode_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, m := range all {
		dbSlugs = append(dbSlugs, m.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedPermissionModeSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}

func TestIntegration_CorePermissionMode_GradientIsMonotonicallyDecreasing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 5, len(all), "must have 5 modes for gradient check")

	for i := 1; i < len(all); i++ {
		assert.Less(t, all[i].SafetyScore, all[i-1].SafetyScore,
			"safety score must be strictly decreasing: mode %q (%d) vs %q (%d)",
			all[i].Slug, all[i].SafetyScore,
			all[i-1].Slug, all[i-1].SafetyScore)
	}
}

func TestIntegration_CorePermissionMode_RecommendedForIsNonEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, permModeMigration)

	loader := core.NewCorePermissionModeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, m := range all {
		assert.NotEmpty(t, m.RecommendedFor,
			"mode %q must have at least one recommended_for entry", m.Slug)
	}
}
