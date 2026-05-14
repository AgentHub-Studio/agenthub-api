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

const codingCatMigration = "000080_seed_coding_agent_category_default_templates.up.sql"
const codingCatMigrationDown = "000080_seed_coding_agent_category_default_templates.down.sql"

func TestIntegration_CoreCodingAgentCategory_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, codingCatMigration)

	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCodingAgentCategoryRowCount, len(got))
}

func TestIntegration_CoreCodingAgentCategory_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCodingAgentCategory_FindBySlug_ChatIntegratedIsAgentHubTarget(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, codingCatMigration)

	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	cat, found, err := loader.FindBySlug(context.Background(), "chat_integrated")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, cat.IsAgentHubTarget)
	assert.Equal(t, 1, cat.GradientIndex)
	assert.Equal(t, "ide_coupled_product", cat.ExecutionPattern)
}

func TestIntegration_CoreCodingAgentCategory_FindBySlug_InlineCompletionNotTarget(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, codingCatMigration)

	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	cat, found, err := loader.FindBySlug(context.Background(), "inline_completion")
	require.NoError(t, err)
	require.True(t, found)
	assert.False(t, cat.IsAgentHubTarget, "inline_completion is not an AgentHub target")
	assert.Equal(t, 0, cat.GradientIndex, "inline_completion is the least autonomous")
	assert.Equal(t, "editor_plugin", cat.ExecutionPattern)
}

func TestIntegration_CoreCodingAgentCategory_FindBySlug_FullyAutonomousHasSandbox(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, codingCatMigration)

	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	cat, found, err := loader.FindBySlug(context.Background(), "fully_autonomous")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 3, cat.GradientIndex, "fully_autonomous is the most autonomous")
	assert.Equal(t, "container_sandbox", cat.IsolationModel)
	assert.False(t, cat.IsAgentHubTarget)
	assert.NotEmpty(t, cat.ExampleSystems)
}

func TestIntegration_CoreCodingAgentCategory_FindBySlug_AgenticCLIUsesPermissionGates(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, codingCatMigration)

	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	cat, found, err := loader.FindBySlug(context.Background(), "agentic_cli")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "permission_gates", cat.IsolationModel,
		"agentic_cli uses the same permission gates as AgentHub's capability tier system")
	assert.True(t, cat.IsAgentHubTarget)
}

func TestIntegration_CoreCodingAgentCategory_FindBySlug_UnknownNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, codingCatMigration)

	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreCodingAgentCategory_LoadAgentHubTargets(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, codingCatMigration)

	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	got, err := loader.LoadAgentHubTargets(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, len(got), "AgentHub targets chat_integrated and agentic_cli")
	for _, cat := range got {
		assert.True(t, cat.IsAgentHubTarget)
	}
}

func TestIntegration_CoreCodingAgentCategory_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, codingCatMigration)
	applyMigration(t, pool, migDir, codingCatMigration)

	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedCodingAgentCategoryRowCount, len(got))
}

func TestIntegration_CoreCodingAgentCategory_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, codingCatMigration)
	applyMigration(t, pool, migDir, codingCatMigrationDown)

	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreCodingAgentCategory_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, codingCatMigration)

	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, c := range all {
		dbSlugs = append(dbSlugs, c.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedCodingAgentCategorySlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}

func TestIntegration_CoreCodingAgentCategory_GradientOrderAscending(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, codingCatMigration)

	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.Equal(t, 4, len(all))

	for i := 1; i < len(all); i++ {
		assert.LessOrEqual(t, all[i-1].GradientIndex, all[i].GradientIndex,
			"gradient_index must be non-decreasing: %q vs %q", all[i-1].Slug, all[i].Slug)
	}
	assert.Equal(t, "inline_completion", all[0].Slug, "least autonomous category must be first")
}

func TestIntegration_CoreCodingAgentCategory_ExampleSystemsNonEmpty(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, codingCatMigration)

	loader := core.NewCoreCodingAgentCategoryDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	for _, cat := range all {
		assert.NotEmpty(t, cat.ExampleSystems,
			"category %q must list at least one example system", cat.Slug)
	}
}
