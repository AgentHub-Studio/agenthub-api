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

const queryPipelineMigration = "000081_seed_query_pipeline_step_default_templates.up.sql"
const queryPipelineMigrationDown = "000081_seed_query_pipeline_step_default_templates.down.sql"

func TestIntegration_CoreQueryPipelineStep_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, queryPipelineMigration)

	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedQueryPipelineStepRowCount, len(got))
}

func TestIntegration_CoreQueryPipelineStep_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreQueryPipelineStep_FindBySlug_ModelCall(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, queryPipelineMigration)

	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	step, found, err := loader.FindBySlug(context.Background(), "model_call")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 5, step.StepOrder)
	assert.Equal(t, "reasoning", step.Phase)
	assert.True(t, step.CanBlock)
	assert.True(t, step.IsRetryable)
	assert.True(t, step.IsPerIteration)
}

func TestIntegration_CoreQueryPipelineStep_FindBySlug_SettingsResolution(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, queryPipelineMigration)

	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	step, found, err := loader.FindBySlug(context.Background(), "settings_resolution")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 1, step.StepOrder)
	assert.Equal(t, "setup", step.Phase)
	assert.False(t, step.CanBlock)
	assert.False(t, step.IsRetryable)
	assert.False(t, step.IsPerIteration, "setup steps are one-time per turn, not per iteration")
}

func TestIntegration_CoreQueryPipelineStep_FindBySlug_PermissionGateBlocks(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, queryPipelineMigration)

	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	step, found, err := loader.FindBySlug(context.Background(), "permission_gate")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 7, step.StepOrder)
	assert.Equal(t, "execution", step.Phase)
	assert.True(t, step.CanBlock)
	assert.False(t, step.IsRetryable)
}

func TestIntegration_CoreQueryPipelineStep_FindBySlug_UnknownNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, queryPipelineMigration)

	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreQueryPipelineStep_LoadBlockingSteps(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, queryPipelineMigration)

	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	got, err := loader.LoadBlockingSteps(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4, len(got), "pre_model_shapers, model_call, permission_gate, stop_condition")
	for _, s := range got {
		assert.True(t, s.CanBlock)
	}
}

func TestIntegration_CoreQueryPipelineStep_LoadRetryableSteps(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, queryPipelineMigration)

	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	got, err := loader.LoadRetryableSteps(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, len(got), "model_call and tool_execution")
	for _, s := range got {
		assert.True(t, s.IsRetryable)
	}
}

func TestIntegration_CoreQueryPipelineStep_LoadStepsInPhase_Setup(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, queryPipelineMigration)

	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	got, err := loader.LoadStepsInPhase(context.Background(), "setup")
	require.NoError(t, err)
	assert.Equal(t, 2, len(got))
	for _, s := range got {
		assert.Equal(t, "setup", s.Phase)
		assert.False(t, s.IsPerIteration)
	}
}

func TestIntegration_CoreQueryPipelineStep_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, queryPipelineMigration)
	applyMigration(t, pool, migDir, queryPipelineMigration)

	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedQueryPipelineStepRowCount, len(got))
}

func TestIntegration_CoreQueryPipelineStep_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, queryPipelineMigration)
	applyMigration(t, pool, migDir, queryPipelineMigrationDown)

	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreQueryPipelineStep_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, queryPipelineMigration)

	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, s := range all {
		dbSlugs = append(dbSlugs, s.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedQueryPipelineStepSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}

func TestIntegration_CoreQueryPipelineStep_StepOrderIsStrictlyIncreasing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, queryPipelineMigration)

	loader := core.NewCoreQueryPipelineStepDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 9, len(all))

	for i := 1; i < len(all); i++ {
		assert.Less(t, all[i-1].StepOrder, all[i].StepOrder,
			"step_order must be strictly increasing: %q vs %q", all[i-1].Slug, all[i].Slug)
	}
	assert.Equal(t, 1, all[0].StepOrder, "first step must have order 1")
	assert.Equal(t, 9, all[8].StepOrder, "last step must have order 9")
}
