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

const toolPoolMigration = "000084_seed_tool_pool_assembly_step_default_templates.up.sql"
const toolPoolMigrationDown = "000084_seed_tool_pool_assembly_step_default_templates.down.sql"

func TestIntegration_CoreToolPoolAssemblyStep_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, toolPoolMigration)

	loader := core.NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedToolPoolAssemblyStepRowCount, len(got))
}

func TestIntegration_CoreToolPoolAssemblyStep_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreToolPoolAssemblyStep_FindBySlug_BaseEnumerationIsFirst(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, toolPoolMigration)

	loader := core.NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool)
	step, found, err := loader.FindBySlug(context.Background(), "base_tool_enumeration")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 1, step.StepOrder)
	assert.True(t, step.IsAlwaysActive)
	assert.False(t, step.CanFilterTools)
	assert.True(t, step.AlwaysPrecedesMCP)
}

func TestIntegration_CoreToolPoolAssemblyStep_FindBySlug_MCPIntegrationIsConditional(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, toolPoolMigration)

	loader := core.NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool)
	step, found, err := loader.FindBySlug(context.Background(), "mcp_tool_integration")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 4, step.StepOrder)
	assert.False(t, step.IsAlwaysActive, "mcp_tool_integration skipped when no MCP servers present")
	assert.False(t, step.CanFilterTools, "MCP integration only adds tools, never removes")
	assert.False(t, step.AlwaysPrecedesMCP)
}

func TestIntegration_CoreToolPoolAssemblyStep_FindBySlug_DeduplicationIsLast(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, toolPoolMigration)

	loader := core.NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool)
	step, found, err := loader.FindBySlug(context.Background(), "deduplication")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 5, step.StepOrder)
	assert.True(t, step.IsAlwaysActive)
	assert.True(t, step.CanFilterTools)
}

func TestIntegration_CoreToolPoolAssemblyStep_FindBySlug_UnknownNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, toolPoolMigration)

	loader := core.NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreToolPoolAssemblyStep_LoadFilteringSteps_ThreeSteps(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, toolPoolMigration)

	loader := core.NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool)
	got, err := loader.LoadFilteringSteps(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, len(got))
	for _, s := range got {
		assert.True(t, s.CanFilterTools)
	}
}

func TestIntegration_CoreToolPoolAssemblyStep_LoadPreMCPSteps_ThreeSteps(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, toolPoolMigration)

	loader := core.NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool)
	got, err := loader.LoadPreMCPSteps(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, len(got))
	for _, s := range got {
		assert.True(t, s.AlwaysPrecedesMCP)
	}
}

func TestIntegration_CoreToolPoolAssemblyStep_LoadAlwaysActive_FourSteps(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, toolPoolMigration)

	loader := core.NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool)
	got, err := loader.LoadAlwaysActive(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4, len(got))
	for _, s := range got {
		assert.True(t, s.IsAlwaysActive)
	}
}

func TestIntegration_CoreToolPoolAssemblyStep_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, toolPoolMigration)
	applyMigration(t, pool, migDir, toolPoolMigration)

	loader := core.NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedToolPoolAssemblyStepRowCount, len(got))
}

func TestIntegration_CoreToolPoolAssemblyStep_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, toolPoolMigration)
	applyMigration(t, pool, migDir, toolPoolMigrationDown)

	loader := core.NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreToolPoolAssemblyStep_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, toolPoolMigration)

	loader := core.NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, s := range all {
		dbSlugs = append(dbSlugs, s.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedToolPoolAssemblyStepSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}
