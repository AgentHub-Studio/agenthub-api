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

const agentRoleMigration = "000076_seed_agent_role_preset_default_templates.up.sql"
const agentRoleMigrationDown = "000076_seed_agent_role_preset_default_templates.down.sql"

func TestIntegration_CoreAgentRolePreset_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedAgentRolePresetRowCount, len(got))
}

func TestIntegration_CoreAgentRolePreset_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreAgentRolePreset_FindBySlug_GeneralAssistantIsDefault(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "general-assistant")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "General Assistant", tmpl.Label)
	assert.Equal(t, "general-purpose", tmpl.SourceSubagentType)
	assert.Equal(t, "default", tmpl.PermissionMode)
	assert.Equal(t, "inline", tmpl.DefaultContextMode)
}

func TestIntegration_CoreAgentRolePreset_FindBySlug_PlannerHasPlanMode(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "planner")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "plan", tmpl.PermissionMode,
		"planner must use plan permission_mode to stop before execution")
	assert.Equal(t, "plan", tmpl.SourceSubagentType)
}

func TestIntegration_CoreAgentRolePreset_FindBySlug_ReadResearcherHasDisallowedTools(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "read-researcher")
	require.NoError(t, err)
	require.True(t, found)
	assert.NotEmpty(t, tmpl.DisallowedTools, "read-researcher must have disallowed mutate tools")
	assert.Equal(t, "explore", tmpl.SourceSubagentType)
}

func TestIntegration_CoreAgentRolePreset_FindBySlug_ValidatorUsesForkContext(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "validator")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "fork", tmpl.DefaultContextMode,
		"validator runs in isolated fork context to avoid polluting main session")
}

func TestIntegration_CoreAgentRolePreset_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "statusline-setup")
	require.NoError(t, err)
	assert.False(t, found, "statusline-setup is NOT_APPLICABLE_WEB and must not be seeded")
}

func TestIntegration_CoreAgentRolePreset_LoadByPermissionMode_PlanMode(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	got, err := loader.LoadByPermissionMode(context.Background(), "plan")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "planner", got[0].Slug)
}

func TestIntegration_CoreAgentRolePreset_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)
	applyMigration(t, pool, migDir, agentRoleMigration)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedAgentRolePresetRowCount, len(got))
}

func TestIntegration_CoreAgentRolePreset_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)
	applyMigration(t, pool, migDir, agentRoleMigrationDown)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreAgentRolePreset_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedAgentRolePresetSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreAgentRolePreset_SortOrderAscending(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, all)
	for i := 1; i < len(all); i++ {
		assert.LessOrEqual(t, all[i-1].SortOrder, all[i].SortOrder)
	}
	assert.Equal(t, "general-assistant", all[0].Slug, "general-assistant sorts first")
}

func TestIntegration_CoreAgentRolePreset_JSONBFieldsDeserialiseCorrectly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "documentation-guide")
	require.NoError(t, err)
	require.True(t, found)
	assert.NotEmpty(t, tmpl.AllowedTools, "documentation-guide must have allowed_tools")
	assert.NotEmpty(t, tmpl.RecommendedFor, "documentation-guide must have recommended_for")
}

func TestIntegration_CoreAgentRolePreset_LoadByContextMode_Fork(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, agentRoleMigration)

	loader := core.NewCoreAgentRolePresetDefaultTemplateLoader(pool)
	got, err := loader.LoadByContextMode(context.Background(), "fork")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "validator", got[0].Slug)
}
