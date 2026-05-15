//go:build integration

package core_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const mhdtMigration = "000032_seed_memory_hierarchy_default_templates.up.sql"
const mhdtMigrationDown = "000032_seed_memory_hierarchy_default_templates.down.sql"

func TestIntegration_CoreMHDT_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedMemoryHierarchyDefaultTemplateRowCount, len(got))
}

func TestIntegration_CoreMHDT_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreMHDT_FindBySlug_ReturnsKnown(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "global-platform-name")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "global", tmpl.TargetScope)
	assert.Equal(t, "platform_name", tmpl.Key)
	assert.Equal(t, "AgentHub", tmpl.DefaultValue)
	assert.Equal(t, 0, tmpl.MaxAgeSeconds)
}

func TestIntegration_CoreMHDT_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreMHDT_LoadByScope_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	globals, err := loader.LoadByScope(context.Background(), "global")
	require.NoError(t, err)
	assert.Equal(t, 4, len(globals))

	tenants, err := loader.LoadByScope(context.Background(), "tenant")
	require.NoError(t, err)
	assert.Equal(t, 4, len(tenants))
}

func TestIntegration_CoreMHDT_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedMemoryHierarchyDefaultTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreMHDT_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewMemoryHierarchyDefaultTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreMHDT_AllScopesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, s := range core.SeedExpectedMemoryHierarchyDefaultTemplateScopes {
		allowed[s] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetScope],
			"template %q has scope %q outside expected set", p.Slug, p.TargetScope)
	}
}

func TestIntegration_CoreMHDT_AllTenantKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedMemoryHierarchyDefaultTemplateTenantKinds {
		allowed[k] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.RecommendedForTenantKind],
			"template %q has tenant_kind %q outside expected set",
			p.Slug, p.RecommendedForTenantKind)
	}
}

func TestIntegration_CoreMHDT_GlobalDefaultsHaveNoExpiry(t *testing.T) {
	// Cross-row invariant: every global template has max_age_seconds=0.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	globals, err := loader.LoadByScope(context.Background(), "global")
	require.NoError(t, err)
	for _, p := range globals {
		assert.Equal(t, 0, p.MaxAgeSeconds,
			"global template %q must have max_age_seconds=0 (never expire)", p.Slug)
	}
}

func TestIntegration_CoreMHDT_TenantDefaultsHavePositiveExpiry(t *testing.T) {
	// Cross-row invariant: tenant defaults must have positive max_age
	// so the hierarchy eventually re-evaluates them.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	tenants, err := loader.LoadByScope(context.Background(), "tenant")
	require.NoError(t, err)
	for _, p := range tenants {
		assert.Positive(t, p.MaxAgeSeconds,
			"tenant template %q must have positive max_age_seconds", p.Slug)
	}
}

func TestIntegration_CoreMHDT_DefaultLocaleAlignsWithLocaleTranslationsSeed(t *testing.T) {
	// Cross-feature invariant: this seed's default-locale value MUST
	// match locale_translations seed default ("en-US"). No drift.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "global-default-locale")
	require.NoError(t, err)
	assert.Equal(t, core.SeedDefaultLocale, tmpl.DefaultValue,
		"global-default-locale must match locale_translations SeedDefaultLocale")
}

func TestIntegration_CoreMHDT_KeyIsLowerSnakeCase(t *testing.T) {
	// Cross-row invariant: keys lookup-friendly (no spaces/uppercase).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.Equal(t, strings.ToLower(p.Key), p.Key,
			"template %q key %q must be lowercase", p.Slug, p.Key)
		assert.NotContains(t, p.Key, " ",
			"template %q key %q must have no spaces", p.Slug, p.Key)
		assert.NotContains(t, p.Key, "-",
			"template %q key %q must use snake_case (no hyphens)", p.Slug, p.Key)
	}
}

func TestIntegration_CoreMHDT_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreMHDT_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30,
			"template %q description must be ≥30 chars", p.Slug)
	}
}

func TestIntegration_CoreMHDT_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
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
	assert.Equal(t, "global-platform-name", all[0].Slug,
		"first by sort_order=10 must be global-platform-name")
}

func TestIntegration_CoreMHDT_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedMemoryHierarchyDefaultTemplateRowCount, len(got))
}

func TestIntegration_CoreMHDT_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)
	applyMigration(t, pool, migDir, mhdtMigrationDown)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreMHDT_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, mhdtMigration)

	loader := core.NewCoreMemoryHierarchyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedMemoryHierarchyDefaultTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
