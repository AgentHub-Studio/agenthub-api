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

const lidMigration = "000038_seed_lazy_instruction_default_templates.up.sql"
const lidMigrationDown = "000038_seed_lazy_instruction_default_templates.down.sql"

func TestIntegration_CoreLID_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedLIDTemplateRowCount, len(got))
}

func TestIntegration_CoreLID_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreLID_FindBySlug_StableRuleCatalogHasZeroTTL(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "stable-rule-catalog")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 0, tmpl.TTLSeconds, "stable-rule-catalog must use TTL=0")
	assert.Equal(t, "ah_core_seed", tmpl.SourceKind)
}

func TestIntegration_CoreLID_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreLID_LoadBySourceKind_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	tenant, err := loader.LoadBySourceKind(context.Background(), "tenant_db")
	require.NoError(t, err)
	// tenant-config-medium-ttl + dev-debug-no-cache = 2.
	assert.Equal(t, 2, len(tenant))

	for _, kind := range []string{"ah_core_seed", "external_http", "compliance_store", "memory_hierarchy"} {
		got, err := loader.LoadBySourceKind(context.Background(), kind)
		require.NoError(t, err)
		assert.Equal(t, 1, len(got), "source %q must have 1 template", kind)
	}
}

func TestIntegration_CoreLID_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedLIDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreLID_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewLIDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreLID_AllSourceKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedLIDTemplateSourceKinds {
		allowed[k] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.SourceKind])
	}
}

func TestIntegration_CoreLID_AllUseCasesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedLIDTemplateUseCases {
		allowed[u] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.TargetUseCase])
	}
}

func TestIntegration_CoreLID_AllTenantKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedLIDTemplateTenantKinds {
		allowed[k] = true
	}
	for _, p := range all {
		assert.True(t, allowed[p.RecommendedForTenantKind])
	}
}

func TestIntegration_CoreLID_AllTTLsAreNonNegative(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, p.TTLSeconds, 0,
			"%s TTL must be ≥ 0 (0 = never expire)", p.Slug)
	}
}

func TestIntegration_CoreLID_StableRuleCatalogIsTheOnlyZeroTTLTemplate(t *testing.T) {
	// Cross-row invariant: only the platform-stable catalog has TTL=0.
	// Everything else picks up updates eventually.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)

	zeroTTL := []string{}
	for _, p := range all {
		if p.TTLSeconds == 0 {
			zeroTTL = append(zeroTTL, p.Slug)
		}
	}
	assert.Equal(t, []string{"stable-rule-catalog"}, zeroTTL)
}

func TestIntegration_CoreLID_RegulatedTemplateRecommendedForRegulatedKind(t *testing.T) {
	// Cross-row invariant: regulated-policy template targets regulated tenants.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "regulated-policy-strict-ttl")
	require.NoError(t, err)
	assert.Equal(t, "regulated", tmpl.RecommendedForTenantKind)
	assert.True(t, tmpl.RequiresAdminReview)
}

func TestIntegration_CoreLID_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreLID_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CoreLID_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
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
	assert.Equal(t, "stable-rule-catalog", all[0].Slug)
}

func TestIntegration_CoreLID_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedLIDTemplateRowCount, len(got))
}

func TestIntegration_CoreLID_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)
	applyMigration(t, pool, migDir, lidMigrationDown)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreLID_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, lidMigration)

	loader := core.NewCoreLazyInstructionDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedLIDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
