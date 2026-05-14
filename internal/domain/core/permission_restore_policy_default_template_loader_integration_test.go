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

const prpdMigration = "000052_seed_permission_restore_policy_default_templates.up.sql"
const prpdMigrationDown = "000052_seed_permission_restore_policy_default_templates.down.sql"

func TestIntegration_CorePRPD_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPRPDTemplateRowCount, len(got))
}

func TestIntegration_CorePRPD_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePRPD_FindBySlug_DiscardShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "discard-all-fresh-context")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "discard_all", tmpl.TargetRestorePolicy)
	assert.False(t, tmpl.FlagsFirstUseConfirmation)
	survivors, err := tmpl.SurvivingDurabilities()
	require.NoError(t, err)
	assert.Empty(t, survivors)
}

func TestIntegration_CorePRPD_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePRPD_LoadByPolicy_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	for _, policy := range core.SeedExpectedPRPDTemplatePolicies {
		matched, err := loader.LoadByPolicy(context.Background(), policy)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched), "policy %q must have exactly 1 template", policy)
	}
}

func TestIntegration_CorePRPD_LoadBySafetyPosture(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	for _, posture := range core.SeedExpectedPRPDTemplateSafetyPostures {
		matched, err := loader.LoadBySafetyPosture(context.Background(), posture)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(matched), 1, "posture %q must have ≥1 template", posture)
	}
}

func TestIntegration_CorePRPD_LoadRecommended_MatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	rec, err := loader.LoadRecommended(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range rec {
		got = append(got, p.Slug)
	}
	expected := append([]string{}, core.SeedRecommendedPRPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePRPD_AdminReviewMatchesGoConstant(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all {
		if p.RequiresAdminReview {
			got = append(got, p.Slug)
		}
	}
	expected := append([]string{}, core.SeedAdminReviewPRPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePRPD_AllPoliciesInPERM009Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, p := range core.SeedExpectedPRPDTemplatePolicies {
		allowed[p] = true
	}
	for _, t2 := range all {
		assert.True(t, allowed[t2.TargetRestorePolicy],
			"%s policy %q outside PERM-009 enum", t2.Slug, t2.TargetRestorePolicy)
	}
}

func TestIntegration_CorePRPD_AllSurvivingDurabilitiesInPERM009Enum(t *testing.T) {
	// Cross-feature invariant: every label in surviving_durabilities
	// MUST match PERM-009 GrantDurability enum byte-for-byte.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{}
	for _, d := range core.SeedExpectedPRPDTemplateDurabilities {
		allowed[d] = true
	}
	for _, t2 := range all {
		survivors, err := t2.SurvivingDurabilities()
		require.NoError(t, err)
		for _, s := range survivors {
			assert.True(t, allowed[s],
				"%s durability %q outside PERM-009 GrantDurability enum", t2.Slug, s)
		}
	}
}

func TestIntegration_CorePRPD_DiscardAllHasEmptySurvivors(t *testing.T) {
	// Cross-row invariant: discard_all policy → empty survivors (matches
	// PERM-009 FilterRestorableGrants behaviour exactly).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "discard-all-fresh-context")
	require.NoError(t, err)
	survivors, err := tmpl.SurvivingDurabilities()
	require.NoError(t, err)
	assert.Empty(t, survivors)
}

func TestIntegration_CorePRPD_StrictReRequestHasFirstUseFlag(t *testing.T) {
	// Cross-row invariant: strict_re_request must flag first-use
	// confirmation (matches PERM-009 RequireFirstUseConfirmation
	// behaviour).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "strict-re-request-audit")
	require.NoError(t, err)
	assert.True(t, tmpl.FlagsFirstUseConfirmation)
}

func TestIntegration_CorePRPD_OnlyStrictReRequestFlagsFirstUse(t *testing.T) {
	// Cross-row invariant: ONLY strict_re_request has
	// flags_first_use_confirmation=true. Other policies must have it
	// false to not contradict PERM-009 semantics.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		if t2.TargetRestorePolicy == "strict_re_request" {
			assert.True(t, t2.FlagsFirstUseConfirmation,
				"%s strict_re_request must flag first use", t2.Slug)
		} else {
			assert.False(t, t2.FlagsFirstUseConfirmation,
				"%s (policy %s) must NOT flag first use", t2.Slug, t2.TargetRestorePolicy)
		}
	}
}

func TestIntegration_CorePRPD_PreserveDurableSurvivesPersistedAndAdmin(t *testing.T) {
	// Cross-row invariant: preserve_durable_only → survivors contain
	// "persisted" AND "explicit_admin" (matches PERM-009 decideRestore).
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "preserve-durable-routine")
	require.NoError(t, err)
	survivors, err := tmpl.SurvivingDurabilities()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"persisted", "explicit_admin"}, survivors)
}

func TestIntegration_CorePRPD_PreserveExplicitSurvivesAdminOnly(t *testing.T) {
	// Cross-row invariant: preserve_explicit_grants → survivors is
	// exactly ["explicit_admin"].
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	tmpl, _, err := loader.FindBySlug(context.Background(), "preserve-explicit-compliance")
	require.NoError(t, err)
	survivors, err := tmpl.SurvivingDurabilities()
	require.NoError(t, err)
	assert.Equal(t, []string{"explicit_admin"}, survivors)
}

func TestIntegration_CorePRPD_DescriptionsArePresent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.GreaterOrEqual(t, len(t2.Description), 40, "%s description", t2.Slug)
	}
}

func TestIntegration_CorePRPD_AllSlugsAreUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seen[t2.Slug])
		seen[t2.Slug] = true
	}
}

func TestIntegration_CorePRPD_OrderingIsBySortOrderThenSlug(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
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
	assert.Equal(t, "discard-all-fresh-context", all[0].Slug)
}

func TestIntegration_CorePRPD_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPRPDTemplateRowCount, len(got))
}

func TestIntegration_CorePRPD_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)
	applyMigration(t, pool, migDir, prpdMigrationDown)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePRPD_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, prpdMigration)

	loader := core.NewCorePermissionRestorePolicyDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedPRPDTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
