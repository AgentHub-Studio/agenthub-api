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

const mcpPresetMigration = "000072_seed_mcp_server_preset_default_templates.up.sql"
const mcpPresetMigrationDown = "000072_seed_mcp_server_preset_default_templates.down.sql"

func TestIntegration_CoreMCPPreset_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mcpPresetMigration)

	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedMCPPresetRowCount, len(got))
}

func TestIntegration_CoreMCPPreset_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreMCPPreset_FindBySlug_FilesystemShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mcpPresetMigration)

	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "filesystem")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "Filesystem", tmpl.Label)
	assert.Equal(t, "stdio", tmpl.TransportType)
	assert.True(t, tmpl.AutoStart)
	require.NotNil(t, tmpl.Command)
	assert.Equal(t, "npx", *tmpl.Command)
	assert.Nil(t, tmpl.BaseURL)
}

func TestIntegration_CoreMCPPreset_FindBySlug_RemoteSSEShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mcpPresetMigration)

	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "remote-sse")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "streamable_http", tmpl.TransportType)
	assert.False(t, tmpl.AutoStart)
	assert.Nil(t, tmpl.Command)
	require.NotNil(t, tmpl.BaseURL)
	assert.Contains(t, *tmpl.BaseURL, "https://")
}

func TestIntegration_CoreMCPPreset_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mcpPresetMigration)

	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreMCPPreset_LoadByTransportType_Stdio(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mcpPresetMigration)

	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
	got, err := loader.LoadByTransportType(context.Background(), "stdio")
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedMCPStdioPresetSlugs), len(got))
	for _, tmpl := range got {
		assert.Equal(t, "stdio", tmpl.TransportType)
	}
}

func TestIntegration_CoreMCPPreset_LoadByTransportType_HTTP(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mcpPresetMigration)

	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
	got, err := loader.LoadByTransportType(context.Background(), "streamable_http")
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedMCPHTTPPresetSlugs), len(got))
	for _, tmpl := range got {
		assert.Equal(t, "streamable_http", tmpl.TransportType)
	}
}

func TestIntegration_CoreMCPPreset_LoadAutoStart(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mcpPresetMigration)

	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAutoStart(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(core.SeedMCPAutoStartSlugs), len(got))
	for _, tmpl := range got {
		assert.True(t, tmpl.AutoStart)
	}
}

func TestIntegration_CoreMCPPreset_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mcpPresetMigration)
	applyMigration(t, pool, migDir, mcpPresetMigration)

	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedMCPPresetRowCount, len(got))
}

func TestIntegration_CoreMCPPreset_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mcpPresetMigration)
	applyMigration(t, pool, migDir, mcpPresetMigrationDown)

	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreMCPPreset_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mcpPresetMigration)

	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedMCPPresetSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreMCPPreset_AllSlugsMatchKebabRegex(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mcpPresetMigration)

	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.True(t, core.SeedMCPPresetSlugRE.MatchString(t2.Slug),
			"slug %q must match kebab regex", t2.Slug)
	}
}

func TestIntegration_CoreMCPPreset_SortOrderAscending(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mcpPresetMigration)

	loader := core.NewCoreMCPServerPresetDefaultTemplateLoader(pool)
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
	assert.Equal(t, "filesystem", all[0].Slug)
}
