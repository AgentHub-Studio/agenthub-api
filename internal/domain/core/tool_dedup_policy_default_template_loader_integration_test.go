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

const tdpdMigration = "000045_seed_tool_dedup_policy_default_templates.up.sql"
const tdpdMigrationDown = "000045_seed_tool_dedup_policy_default_templates.down.sql"

func TestIntegration_CoreTDPD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedTDPDTemplateRowCount, len(got))
}

func TestIntegration_CoreTDPD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreTDPD_FindBySlug_DefaultShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "default-source-rank")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "prefer_source_rank", tmpl.TargetPolicy)
	assert.Equal(t, "balanced", tmpl.SafetyPosture)
	assert.False(t, tmpl.RequiresAdminReview)
}

func TestIntegration_CoreTDPD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CoreTDPD_LoadByPolicy_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	for _, policy := range core.SeedExpectedTDPDTemplatePolicies {
		matched, err := loader.LoadByPolicy(context.Background(), policy)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "policy %q must have exactly 1 template (1:1 mapping)", policy)
	}
}

func TestIntegration_CoreTDPD_LoadBySafetyPosture_FiltersStrictly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	for _, posture := range core.SeedExpectedTDPDTemplateSafetyPostures {
		matched, err := loader.LoadBySafetyPosture(context.Background(), posture)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(matched), 1, "posture %q must have ≥1 template", posture)
	}
}

func TestIntegration_CoreTDPD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedTDPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreTDPD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewTDPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CoreTDPD_AllPoliciesAreInTOOL004Enum(t *testing.T) {
	// Cross-feature invariant: every policy MUST match TOOL-004 enum.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedTDPDTemplatePolicies {
		allowed[p] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetPolicy],
			"%s policy %q outside TOOL-004 enum", t2.Slug, t2.TargetPolicy)
	}
}

func TestIntegration_CoreTDPD_AllUseCasesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, u := range core.SeedExpectedTDPDTemplateUseCases {
		allowed[u] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetUseCase])
	}
}

func TestIntegration_CoreTDPD_AllSafetyPosturesAreInExpectedSet(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedTDPDTemplateSafetyPostures {
		allowed[p] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.SafetyPosture])
	}
}

func TestIntegration_CoreTDPD_AllPoliciesHaveExactlyOneTemplate(t *testing.T) {
	// Cross-row invariant: 1:1 mapping between policy and template.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	policyCount := map[string]int{}
	for _, t2 := range all {
		policyCount[t2.TargetPolicy]++
	}
	for _, policy := range core.SeedExpectedTDPDTemplatePolicies {
		assert.Equal(t, 1, policyCount[policy],
			"policy %q must have exactly 1 template", policy)
	}
}

func TestIntegration_CoreTDPD_ConservativeAndStrictGateAdminReview(t *testing.T) {
	// Cross-row invariant: only conservative + strict postures require
	// admin review (balanced/progressive/permissive are routine).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		switch t2.SafetyPosture {
		case "conservative", "strict":
			assert.True(t, t2.RequiresAdminReview, "%s should require review", t2.Slug)
		default:
			assert.False(t, t2.RequiresAdminReview, "%s should NOT require review", t2.Slug)
		}
	}
}

func TestIntegration_CoreTDPD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 30, "%s description", t2.Slug)
	}
}

func TestIntegration_CoreTDPD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CoreTDPD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
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
	assert.Equal(t, "default-source-rank", all[0].Slug)
}

func TestIntegration_CoreTDPD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedTDPDTemplateRowCount, len(got))
}

func TestIntegration_CoreTDPD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)
	applyMigration(t, pool, migDir, tdpdMigrationDown)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CoreTDPD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, tdpdMigration)

	loader := core.NewCoreToolDedupPolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedTDPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
