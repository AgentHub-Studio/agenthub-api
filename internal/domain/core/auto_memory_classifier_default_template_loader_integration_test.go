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

const amcdMigration = "000033_seed_auto_memory_classifier_default_templates.up.sql"
const amcdMigrationDown = "000033_seed_auto_memory_classifier_default_templates.down.sql"

func TestIntegration_CoreAMCD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedAMCDTemplateRowCount, len(got))
}

func TestIntegration_CoreAMCD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreAMCD_FindBySlug_BalancedDefaultMatchesCTX006(t *testing.T) {
	// Cross-feature invariant: balanced-default DB row MUST match
	// CTX-006 DefaultAutoMemoryConfig byte-for-byte.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "balanced-default")
	require.NoError(t, err)
	require.True(t, found)
	// CTX-006 DefaultAutoMemoryConfig: MinConfidence=0.5, MaxPerTurn=10.
	assert.Equal(t, 0.5, tmpl.MinConfidence)
	assert.Equal(t, 10, tmpl.MaxPerTurn)
	// Block list must include the 5 standard sensitive keys.
	keys := tmpl.AdminBlockedKeysList()
	for _, expected := range []string{"password", "credit_card", "ssn", "api_key", "secret"} {
		assert.Contains(t, keys, expected,
			"balanced-default block list missing %q (CTX-006 default)", expected)
	}
}

func TestIntegration_CoreAMCD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreAMCD_LoadByPosture_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	for _, p := range core.SeedExpectedAMCDTemplatePostures {
		got, err := loader.LoadByPosture(context.Background(), p)
		require.NoError(t, err)
		assert.Equal(t, 1, len(got), "posture %q must have exactly 1 template", p)
	}
}

func TestIntegration_CoreAMCD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedAMCDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreAMCD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewAMCDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreAMCD_AllPosturesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedAMCDTemplatePostures {
		allowed[p] = true
	}
	for _, tmpl := range all {
		assert.True(t, allowed[tmpl.TargetPosture],
			"template %q has posture %q outside expected set",
			tmpl.Slug, tmpl.TargetPosture)
	}
}

func TestIntegration_CoreAMCD_AllTenantKindsAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, k := range core.SeedExpectedAMCDTemplateTenantKinds {
		allowed[k] = true
	}
	for _, tmpl := range all {
		assert.True(t, allowed[tmpl.RecommendedForTenantKind])
	}
}

func TestIntegration_CoreAMCD_MinConfidenceInRange(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, p.MinConfidence, 0.0, "%s min_confidence ≥ 0", p.Slug)
		assert.LessOrEqual(t, p.MinConfidence, 1.0, "%s min_confidence ≤ 1", p.Slug)
	}
}

func TestIntegration_CoreAMCD_MaxPerTurnIsNonNegative(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, p.MaxPerTurn, 0,
			"%s max_per_turn must be ≥ 0 (0 = unlimited)", p.Slug)
	}
}

func TestIntegration_CoreAMCD_BlockListWidensAsPostureGetsStricter(t *testing.T) {
	// Cross-template ordering invariant:
	//   dev_debug ≤ lenient ≤ balanced ≤ strict ≤ privacy_first ≤ pii_strict
	// (block list size monotonically widens with posture strictness).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	sizeOf := func(slug string) int {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		return len(tmpl.AdminBlockedKeysList())
	}

	dev := sizeOf("dev-debug")
	lenient := sizeOf("lenient-exploration")
	balanced := sizeOf("balanced-default")
	strict := sizeOf("strict-conservative")
	privacy := sizeOf("privacy-first")
	pii := sizeOf("pii-strict")

	assert.LessOrEqual(t, dev, lenient, "dev-debug ≤ lenient")
	assert.LessOrEqual(t, lenient, balanced, "lenient ≤ balanced")
	assert.LessOrEqual(t, balanced, strict, "balanced ≤ strict")
	assert.LessOrEqual(t, strict, privacy, "strict ≤ privacy-first")
	assert.LessOrEqual(t, privacy, pii, "privacy-first ≤ pii-strict")
	assert.Greater(t, pii, dev, "pii-strict > dev-debug (extreme ladder ends differ)")
}

func TestIntegration_CoreAMCD_StrictHasHigherConfidenceThanLenient(t *testing.T) {
	// Cross-template ladder invariant: dev_debug<lenient<balanced<strict<pii_strict.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	conf := func(slug string) float64 {
		tmpl, _, err := loader.FindBySlug(context.Background(), slug)
		require.NoError(t, err)
		return tmpl.MinConfidence
	}

	assert.Less(t, conf("dev-debug"), conf("lenient-exploration"))
	assert.Less(t, conf("lenient-exploration"), conf("balanced-default"))
	assert.Less(t, conf("balanced-default"), conf("strict-conservative"))
	assert.Less(t, conf("strict-conservative"), conf("pii-strict"))
}

func TestIntegration_CoreAMCD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Slug])
		seen[p.Slug] = true
	}
}

func TestIntegration_CoreAMCD_AllDescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, p := range all {
		assert.GreaterOrEqual(t, len(p.Description), 30, "%s description", p.Slug)
	}
}

func TestIntegration_CoreAMCD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
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
	assert.Equal(t, "balanced-default", all[0].Slug)
}

func TestIntegration_CoreAMCD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedAMCDTemplateRowCount, len(got))
}

func TestIntegration_CoreAMCD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)
	applyMigration(t, pool, migDir, amcdMigrationDown)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreAMCD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, amcdMigration)

	loader := core.NewCoreAutoMemoryClassifierDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedExpectedAMCDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
