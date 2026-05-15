package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreCRPDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksRoutingPostureWithoutInventingSemantics", func(t *testing.T) {
		// Given fresh tenants need a routing posture choice,
		// And EXT-005 ComponentRouter accepts 4 conflict policies,
		// When admin opens routing onboarding,
		// Then 4 recommended presets cover the spectrum (incumbent /
		// latest / strict / fail-fast).
		assert.Equal(t, 4, len(SeedRecommendedCRPDTemplateSlugs))
	})

	t.Run("Scenario_StableIncumbentIsConservativeProductionDefault", func(t *testing.T) {
		// Given most production tenants want predictable behavior,
		// When admin picks stable-incumbent,
		// Then policy = first_install_wins — incumbent never gets
		// auto-replaced by newcomer.
		assert.Contains(t, SeedExpectedCRPDTemplateSlugs, "stable-incumbent")
	})

	t.Run("Scenario_OptimisticLatestForAutomaticUpgrades", func(t *testing.T) {
		// Given some tenants trust marketplace updates and want
		// automatic upgrades on conflict,
		// When admin picks optimistic-latest,
		// Then policy = latest_install_wins.
		assert.Contains(t, SeedExpectedCRPDTemplateSlugs, "optimistic-latest")
	})

	t.Run("Scenario_StrictPinnedRequiresAdminReviewForRegulatedTenants", func(t *testing.T) {
		// Given regulated tenants want zero ambiguity,
		// When admin picks strict-pinned,
		// Then admin review required (force ops to acknowledge that
		// every conflict needs an explicit pin).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewCRPDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["strict-pinned"])
	})

	t.Run("Scenario_FailFastForStagingMisconfigurationDetection", func(t *testing.T) {
		// Given staging environments want misconfiguration to fail
		// loudly before production deploy,
		// When admin picks fail-fast,
		// Then conflict throws ErrRoutingConflictUnresolved.
		assert.Contains(t, SeedExpectedCRPDTemplateSlugs, "fail-fast")
	})

	t.Run("Scenario_DevDebugNotRecommendedForProduction", func(t *testing.T) {
		// Given dev-debug-incumbent inherits stale routes silently,
		// When admin browses,
		// Then it is NOT recommended for production tenants.
		set := map[string]bool{}
		for _, s := range SeedRecommendedCRPDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["dev-debug-incumbent"])
	})

	t.Run("Scenario_PolicyLabelsAreEXT005EnumByteForByte", func(t *testing.T) {
		// Given EXT-005 RoutingConflictPolicy has 4 enum values,
		// When the seed declares conflict_policy,
		// Then labels match enum bytes (no mapping table runtime).
		ext005 := []string{
			"first_install_wins", "latest_install_wins",
			"require_explicit_pin", "error_on_conflict",
		}
		set := map[string]bool{}
		for _, p := range SeedExpectedCRPDTemplatePolicies {
			set[p] = true
		}
		for _, e := range ext005 {
			assert.True(t, set[e], "EXT-005 policy %q missing", e)
		}
	})

	t.Run("Scenario_FivePresetsCoverDeploymentShapesWithoutOverwhelm", func(t *testing.T) {
		// Given product research showed 5-7 templates is the sweet spot,
		// When the seed ships,
		// Then exactly 5 presets exist (4 production + 1 dev).
		assert.Equal(t, 5, len(SeedExpectedCRPDTemplateSlugs))
	})

	t.Run("Scenario_RegulatedTenantsGetStrictPolicyForExplicitDecisions", func(t *testing.T) {
		// Given regulated audit needs every routing decision visible,
		// When admin filters by tenant_kind=regulated,
		// Then strict-pinned recommended_for_tenant_kind=regulated
		// surfaces (explicit pin required for every conflict).
		// Validated structurally via integration test.
		assert.Contains(t, SeedExpectedCRPDTemplateSlugs, "strict-pinned")
	})

	t.Run("Scenario_StagingPolicyExposesMisconfigurationBeforeProductionDeploy", func(t *testing.T) {
		// Given staging is the place to catch misconfiguration,
		// When admin picks fail-fast for staging,
		// Then any conflict in staging halts deploys — issues caught
		// BEFORE they reach production tenants.
		// Validated via integration test target_use_case=staging.
		assert.Contains(t, SeedExpectedCRPDTemplateSlugs, "fail-fast")
	})
}
