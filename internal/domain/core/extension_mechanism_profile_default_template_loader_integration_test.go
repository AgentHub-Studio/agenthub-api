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

const extMechanismMigration = "000086_seed_extension_mechanism_profile_default_templates.up.sql"
const extMechanismMigrationDown = "000086_seed_extension_mechanism_profile_default_templates.down.sql"

func TestIntegration_CoreExtensionMechanism_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, extMechanismMigration)

	loader := core.NewCoreExtensionMechanismProfileDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedExtensionMechanismRowCount, len(got))
}

func TestIntegration_CoreExtensionMechanism_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreExtensionMechanismProfileDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreExtensionMechanism_FindBySlug_HooksIsZeroCost(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, extMechanismMigration)

	loader := core.NewCoreExtensionMechanismProfileDefaultTemplateLoader(pool)
	m, found, err := loader.FindBySlug(context.Background(), "hooks")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, m.IsZeroCostByDefault, "hooks zero context cost by default (Table 2)")
	assert.Equal(t, "micro", m.ContextCostCategory)
	assert.Equal(t, "execute", m.InsertionPoint)
	assert.False(t, m.CoversAllInsertPoints)
}

func TestIntegration_CoreExtensionMechanism_FindBySlug_PluginsCoversAll(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, extMechanismMigration)

	loader := core.NewCoreExtensionMechanismProfileDefaultTemplateLoader(pool)
	m, found, err := loader.FindBySlug(context.Background(), "plugins")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, m.CoversAllInsertPoints)
	assert.Equal(t, "all", m.InsertionPoint)
	assert.Equal(t, "medium", m.ContextCostCategory)
}

func TestIntegration_CoreExtensionMechanism_FindBySlug_MCPHighCost(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, extMechanismMigration)

	loader := core.NewCoreExtensionMechanismProfileDefaultTemplateLoader(pool)
	m, found, err := loader.FindBySlug(context.Background(), "mcp_servers")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "large", m.ContextCostCategory)
	assert.Equal(t, "model", m.InsertionPoint)
}

func TestIntegration_CoreExtensionMechanism_FindBySlug_UnknownNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, extMechanismMigration)

	loader := core.NewCoreExtensionMechanismProfileDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreExtensionMechanism_LoadZeroCostByDefault_OnlyHooks(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, extMechanismMigration)

	loader := core.NewCoreExtensionMechanismProfileDefaultTemplateLoader(pool)
	got, err := loader.LoadZeroCostByDefault(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, "hooks", got[0].Slug)
}

func TestIntegration_CoreExtensionMechanism_LoadAtInsertionPoint_Assemble(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, extMechanismMigration)

	loader := core.NewCoreExtensionMechanismProfileDefaultTemplateLoader(pool)
	got, err := loader.LoadAtInsertionPoint(context.Background(), "assemble")
	require.NoError(t, err)
	// skills (assemble) + plugins (all)
	assert.Equal(t, 2, len(got))
	slugs := []string{got[0].Slug, got[1].Slug}
	assert.Contains(t, slugs, "skills")
	assert.Contains(t, slugs, "plugins")
}

func TestIntegration_CoreExtensionMechanism_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, extMechanismMigration)
	applyMigration(t, pool, migDir, extMechanismMigration)

	loader := core.NewCoreExtensionMechanismProfileDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedExtensionMechanismRowCount, len(got))
}

func TestIntegration_CoreExtensionMechanism_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, extMechanismMigration)
	applyMigration(t, pool, migDir, extMechanismMigrationDown)

	loader := core.NewCoreExtensionMechanismProfileDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreExtensionMechanism_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, extMechanismMigration)

	loader := core.NewCoreExtensionMechanismProfileDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, m := range all {
		dbSlugs = append(dbSlugs, m.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedExtensionMechanismSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}
