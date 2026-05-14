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

const ctxMgmtApproachMigration = "000088_seed_context_management_approach_default_templates.up.sql"
const ctxMgmtApproachMigrationDown = "000088_seed_context_management_approach_default_templates.down.sql"

func TestIntegration_CoreContextManagementApproach_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxMgmtApproachMigration)

	loader := core.NewCoreContextManagementApproachDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedContextManagementApproachRowCount, len(got))
}

func TestIntegration_CoreContextManagementApproach_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreContextManagementApproachDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreContextManagementApproach_FindBySlug_GraduatedCompaction(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxMgmtApproachMigration)

	loader := core.NewCoreContextManagementApproachDefaultTemplateLoader(pool)
	m, found, err := loader.FindBySlug(context.Background(), "graduated_compaction")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "very_fine", m.Granularity)
	assert.True(t, m.IsAgentHubApproach)
	assert.Contains(t, m.Mechanism, "pipeline")
}

func TestIntegration_CoreContextManagementApproach_FindBySlug_UnknownNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxMgmtApproachMigration)

	loader := core.NewCoreContextManagementApproachDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreContextManagementApproach_LoadAgentHubApproach(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxMgmtApproachMigration)

	loader := core.NewCoreContextManagementApproachDefaultTemplateLoader(pool)
	m, found, err := loader.LoadAgentHubApproach(context.Background())
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "graduated_compaction", m.Slug)
}

func TestIntegration_CoreContextManagementApproach_LoadByGranularity_CoarseHasTwo(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxMgmtApproachMigration)

	loader := core.NewCoreContextManagementApproachDefaultTemplateLoader(pool)
	got, err := loader.LoadByGranularity(context.Background(), "coarse")
	require.NoError(t, err)
	assert.Equal(t, 2, len(got))
}

func TestIntegration_CoreContextManagementApproach_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxMgmtApproachMigration)
	applyMigration(t, pool, migDir, ctxMgmtApproachMigration)

	loader := core.NewCoreContextManagementApproachDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedContextManagementApproachRowCount, len(got))
}

func TestIntegration_CoreContextManagementApproach_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxMgmtApproachMigration)
	applyMigration(t, pool, migDir, ctxMgmtApproachMigrationDown)

	loader := core.NewCoreContextManagementApproachDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreContextManagementApproach_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, ctxMgmtApproachMigration)

	loader := core.NewCoreContextManagementApproachDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, m := range all {
		dbSlugs = append(dbSlugs, m.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedContextManagementApproachSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}
