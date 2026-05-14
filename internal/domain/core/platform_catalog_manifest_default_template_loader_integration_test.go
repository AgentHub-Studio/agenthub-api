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

const pcmdMigration = "000064_seed_platform_catalog_manifest_default_templates.up.sql"
const pcmdMigrationDown = "000064_seed_platform_catalog_manifest_default_templates.down.sql"

func TestIntegration_CorePCMD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPCMDTemplateRowCount, len(got))
}

func TestIntegration_CorePCMD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePCMD_FindBySlug_BuiltinSubagentShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "builtin_subagent_default_template")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "subagent_roster", tmpl.Kind)
	assert.Equal(t, 61, tmpl.MigrationNumber)
	assert.Equal(t, "core", tmpl.LoaderPackage)
	assert.Equal(t, 7, tmpl.ExpectedRowCount)
	assert.True(t, tmpl.IsTenantShared)
	assert.True(t, tmpl.IsRecommended)
}

func TestIntegration_CorePCMD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePCMD_LoadByKind_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	for _, k := range core.SeedExpectedPCMDTemplateKinds {
		matched, err := loader.LoadByKind(context.Background(), k)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched),
			"kind %q must have exactly 1 manifest (1:1)", k)
	}
}

func TestIntegration_CorePCMD_AllKindsInCORE_SEED_001Enum(t *testing.T) {
	// Cross-feature invariant: every seeded kind must exist in the
	// CORE-SEED-001 PlatformCatalogKind enum.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedPCMDTemplateKinds {
		allowed[k] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.Kind],
			"%s kind %q outside CORE-SEED-001 enum", t2.Slug, t2.Kind)
	}
}

func TestIntegration_CorePCMD_AllManifestsRegisterInDomainRegistry(t *testing.T) {
	// Cross-feature invariant: every seeded manifest must validate against
	// the CORE-SEED-001 PlatformCatalogManifest contract and be acceptable
	// by PlatformCatalogRegistry.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	r := core.NewPlatformCatalogRegistry()
	for _, t2 := range all {
		m := core.PlatformCatalogManifest{
			Slug:                  t2.Slug,
			Kind:                  core.PlatformCatalogKind(t2.Kind),
			MigrationNumber:       t2.MigrationNumber,
			LoaderPackage:         t2.LoaderPackage,
			ExpectedRowCount:      t2.ExpectedRowCount,
			RequiresAdminApproval: t2.RequiresAdminApproval,
			IsTenantShared:        t2.IsTenantShared,
		}
		err := r.Register(m)
		assert.NoError(t, err, "%s must register cleanly in CORE-SEED-001 registry", t2.Slug)
	}
	assert.Equal(t, core.SeedExpectedPCMDTemplateRowCount, r.Size())
}

func TestIntegration_CorePCMD_UniqueMigrationNumbers(t *testing.T) {
	// DB-level invariant (UNIQUE constraint) AND cross-feature:
	// golang-migrate must not see two catalogs claiming the same number.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[int]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.MigrationNumber],
			"duplicate migration number %d on %s", t2.MigrationNumber, t2.Slug)
		seen[t2.MigrationNumber] = true
	}
}

func TestIntegration_CorePCMD_AllLoaderPackagesCore(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.Equal(t, "core", t2.LoaderPackage,
			"%s loader_package must be 'core'", t2.Slug)
	}
}

func TestIntegration_CorePCMD_AllRowCountsPositive(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.Greater(t, t2.ExpectedRowCount, 0,
			"%s expected_row_count must be > 0", t2.Slug)
	}
}

func TestIntegration_CorePCMD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 30,
			"%s description must be substantive", t2.Slug)
	}
}

func TestIntegration_CorePCMD_AllSlugsUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CorePCMD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPCMDTemplateRowCount, len(got))
}

func TestIntegration_CorePCMD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)
	applyMigration(t, pool, migDir, pcmdMigrationDown)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePCMD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, pcmdMigration)

	loader := core.NewCorePlatformCatalogManifestDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedPCMDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
