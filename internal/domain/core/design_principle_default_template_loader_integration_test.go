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

const designPrincipleMigration = "000089_seed_design_principle_default_templates.up.sql"
const designPrincipleMigrationDown = "000089_seed_design_principle_default_templates.down.sql"

func TestIntegration_CoreDesignPrinciple_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designPrincipleMigration)

	loader := core.NewCoreDesignPrincipleDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedDesignPrincipleRowCount, len(got))
}

func TestIntegration_CoreDesignPrinciple_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreDesignPrincipleDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreDesignPrinciple_FindBySlug_DenyFirst(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designPrincipleMigration)

	loader := core.NewCoreDesignPrincipleDefaultTemplateLoader(pool)
	m, found, err := loader.FindBySlug(context.Background(), "deny_first_human_escalation")
	require.NoError(t, err)
	require.True(t, found)
	assert.Contains(t, m.ValuesServed, "human_authority")
	assert.Contains(t, m.ValuesServed, "safety")
	assert.Contains(t, m.ReferencedSections, "5")
}

func TestIntegration_CoreDesignPrinciple_FindBySlug_UnknownNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designPrincipleMigration)

	loader := core.NewCoreDesignPrincipleDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreDesignPrinciple_LoadServingCapability_SevenPrinciples(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designPrincipleMigration)

	loader := core.NewCoreDesignPrincipleDefaultTemplateLoader(pool)
	got, err := loader.LoadServingValue(context.Background(), "capability")
	require.NoError(t, err)
	assert.Equal(t, 7, len(got))
}

func TestIntegration_CoreDesignPrinciple_IsolatedSubagentHasThreeValues(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designPrincipleMigration)

	loader := core.NewCoreDesignPrincipleDefaultTemplateLoader(pool)
	m, found, err := loader.FindBySlug(context.Background(), "isolated_subagent_boundaries")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 3, len(m.ValuesServed))
	assert.Contains(t, m.ValuesServed, "reliability")
	assert.Contains(t, m.ValuesServed, "safety")
	assert.Contains(t, m.ValuesServed, "capability")
}

func TestIntegration_CoreDesignPrinciple_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designPrincipleMigration)
	applyMigration(t, pool, migDir, designPrincipleMigration)

	loader := core.NewCoreDesignPrincipleDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedDesignPrincipleRowCount, len(got))
}

func TestIntegration_CoreDesignPrinciple_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designPrincipleMigration)
	applyMigration(t, pool, migDir, designPrincipleMigrationDown)

	loader := core.NewCoreDesignPrincipleDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreDesignPrinciple_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designPrincipleMigration)

	loader := core.NewCoreDesignPrincipleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, m := range all {
		dbSlugs = append(dbSlugs, m.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedDesignPrincipleSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}
