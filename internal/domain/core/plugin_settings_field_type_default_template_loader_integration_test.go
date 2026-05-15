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

const pluginSettingsMigration = "000085_seed_plugin_settings_field_type_default_templates.up.sql"
const pluginSettingsMigrationDown = "000085_seed_plugin_settings_field_type_default_templates.down.sql"

func TestIntegration_CorePluginSettingsFieldType_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pluginSettingsMigration)

	loader := core.NewCorePluginSettingsFieldTypeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPluginSettingsFieldTypeRowCount, len(got))
}

func TestIntegration_CorePluginSettingsFieldType_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePluginSettingsFieldTypeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePluginSettingsFieldType_FindBySlug_SecretIsMaskable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pluginSettingsMigration)

	loader := core.NewCorePluginSettingsFieldTypeDefaultTemplateLoader(pool)
	ft, found, err := loader.FindBySlug(context.Background(), "secret")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, ft.IsMaskable)
	assert.False(t, ft.RequiresOptions)
	assert.Equal(t, "password_input", ft.UIWidget)
}

func TestIntegration_CorePluginSettingsFieldType_FindBySlug_SelectRequiresOptions(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pluginSettingsMigration)

	loader := core.NewCorePluginSettingsFieldTypeDefaultTemplateLoader(pool)
	ft, found, err := loader.FindBySlug(context.Background(), "select")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, ft.RequiresOptions, "select must declare an options list")
	assert.False(t, ft.IsMaskable)
	assert.Equal(t, "dropdown", ft.UIWidget)
}

func TestIntegration_CorePluginSettingsFieldType_FindBySlug_NumberIsNumeric(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pluginSettingsMigration)

	loader := core.NewCorePluginSettingsFieldTypeDefaultTemplateLoader(pool)
	ft, found, err := loader.FindBySlug(context.Background(), "number")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, ft.IsNumeric)
	assert.Equal(t, "number_input", ft.UIWidget)
}

func TestIntegration_CorePluginSettingsFieldType_FindBySlug_UnknownNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pluginSettingsMigration)

	loader := core.NewCorePluginSettingsFieldTypeDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePluginSettingsFieldType_LoadMaskable_OnlySecret(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pluginSettingsMigration)

	loader := core.NewCorePluginSettingsFieldTypeDefaultTemplateLoader(pool)
	got, err := loader.LoadMaskable(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, "secret", got[0].Slug)
}

func TestIntegration_CorePluginSettingsFieldType_LoadOptionsRequired_OnlySelect(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pluginSettingsMigration)

	loader := core.NewCorePluginSettingsFieldTypeDefaultTemplateLoader(pool)
	got, err := loader.LoadOptionsRequired(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, "select", got[0].Slug)
}

func TestIntegration_CorePluginSettingsFieldType_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pluginSettingsMigration)
	applyMigration(t, pool, migDir, pluginSettingsMigration)

	loader := core.NewCorePluginSettingsFieldTypeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPluginSettingsFieldTypeRowCount, len(got))
}

func TestIntegration_CorePluginSettingsFieldType_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pluginSettingsMigration)
	applyMigration(t, pool, migDir, pluginSettingsMigrationDown)

	loader := core.NewCorePluginSettingsFieldTypeDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePluginSettingsFieldType_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pluginSettingsMigration)

	loader := core.NewCorePluginSettingsFieldTypeDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, ft := range all {
		dbSlugs = append(dbSlugs, ft.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedPluginSettingsFieldTypeSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}
