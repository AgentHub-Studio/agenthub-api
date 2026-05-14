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

const designValueTensionMigration = "000087_seed_design_value_tension_default_templates.up.sql"
const designValueTensionMigrationDown = "000087_seed_design_value_tension_default_templates.down.sql"

func TestIntegration_CoreDesignValueTension_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designValueTensionMigration)

	loader := core.NewCoreDesignValueTensionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedDesignValueTensionRowCount, len(got))
}

func TestIntegration_CoreDesignValueTension_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreDesignValueTensionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreDesignValueTension_FindBySlug_AuthoritySafetyProfile(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designValueTensionMigration)

	loader := core.NewCoreDesignValueTensionDefaultTemplateLoader(pool)
	m, found, err := loader.FindBySlug(context.Background(), "authority_safety")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "human_authority", m.Value1)
	assert.Equal(t, "safety", m.Value2)
	assert.Contains(t, m.TensionLabel, "fatigue")
}

func TestIntegration_CoreDesignValueTension_FindBySlug_UnknownNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designValueTensionMigration)

	loader := core.NewCoreDesignValueTensionDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreDesignValueTension_LoadInvolvingCapability_ThreeTensions(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designValueTensionMigration)

	loader := core.NewCoreDesignValueTensionDefaultTemplateLoader(pool)
	got, err := loader.LoadInvolvingValue(context.Background(), "capability")
	require.NoError(t, err)
	assert.Equal(t, 3, len(got))
}

func TestIntegration_CoreDesignValueTension_LoadInvolvingSafety_ThreeTensions(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designValueTensionMigration)

	loader := core.NewCoreDesignValueTensionDefaultTemplateLoader(pool)
	got, err := loader.LoadInvolvingValue(context.Background(), "safety")
	require.NoError(t, err)
	assert.Equal(t, 3, len(got))
}

func TestIntegration_CoreDesignValueTension_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designValueTensionMigration)
	applyMigration(t, pool, migDir, designValueTensionMigration)

	loader := core.NewCoreDesignValueTensionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedDesignValueTensionRowCount, len(got))
}

func TestIntegration_CoreDesignValueTension_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designValueTensionMigration)
	applyMigration(t, pool, migDir, designValueTensionMigrationDown)

	loader := core.NewCoreDesignValueTensionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreDesignValueTension_SeedSlugsMatchCanonicalList(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, designValueTensionMigration)

	loader := core.NewCoreDesignValueTensionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	dbSlugs := []string{}
	for _, m := range all {
		dbSlugs = append(dbSlugs, m.Slug)
	}
	sort.Strings(dbSlugs)
	expected := append([]string{}, core.SeedExpectedDesignValueTensionSlugs...)
	sort.Strings(expected)
	assert.Equal(t, expected, dbSlugs)
}
