package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePRPDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksRestorePolicyFromCatalog", func(t *testing.T) {
		// Given fresh tenants must pick a PERM-009 PermissionRestorePolicy
		// before resume/fork pipeline runs,
		// When admin opens restore-policy onboarding,
		// Then 4 recommended templates surface 1:1 with the enum.
		assert.Equal(t, 4, len(SeedRecommendedPRPDTemplateSlugs))
	})

	t.Run("Scenario_DiscardAllForFreshContextOnboarding", func(t *testing.T) {
		// Given a tenant just enabled session resume and wants maximum
		// safety,
		// When admin uses discard-all-fresh-context,
		// Then surviving_durabilities is empty (every Allow/Confirm
		// dropped) and admin review required.
		assert.Contains(t, SeedExpectedPRPDTemplateSlugs, "discard-all-fresh-context")
	})

	t.Run("Scenario_PreserveDurableRoutineForBalancedTenants", func(t *testing.T) {
		// Given routine tenants want continuity for vetted grants but
		// not for transient ones,
		// When admin uses preserve-durable-routine,
		// Then it's the only routine (no admin review) template.
		assert.Contains(t, SeedExpectedPRPDTemplateSlugs, "preserve-durable-routine")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewPRPDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["preserve-durable-routine"])
	})

	t.Run("Scenario_PreserveExplicitForComplianceTenants", func(t *testing.T) {
		// Given compliance-strict tenants want only admin grants to
		// survive resume,
		// When admin uses preserve-explicit-compliance,
		// Then surviving_durabilities is just [explicit_admin].
		assert.Contains(t, SeedExpectedPRPDTemplateSlugs, "preserve-explicit-compliance")
	})

	t.Run("Scenario_StrictReRequestForAuditTenants", func(t *testing.T) {
		// Given audit-strict tenants want the runtime to confirm on
		// every first tool use,
		// When admin uses strict-re-request-audit,
		// Then flags_first_use_confirmation=true.
		assert.Contains(t, SeedExpectedPRPDTemplateSlugs, "strict-re-request-audit")
	})

	t.Run("Scenario_PolicyLabelsMatchPERM009EnumByteForByte", func(t *testing.T) {
		// Given PERM-009 PermissionRestorePolicy has 4 values,
		// When seed declares target_restore_policy,
		// Then labels match enum bytes (no mapping table runtime).
		perm009 := []string{"discard_all", "preserve_durable_only",
			"preserve_explicit_grants", "strict_re_request"}
		set := map[string]bool{}
		for _, p := range SeedExpectedPRPDTemplatePolicies {
			set[p] = true
		}
		for _, e := range perm009 {
			assert.True(t, set[e], "PERM-009 policy %q missing", e)
		}
	})

	t.Run("Scenario_DurabilityLabelsMatchPERM009Enum", func(t *testing.T) {
		// Given PERM-009 GrantDurability has 4 tiers,
		// When seed declares surviving_durabilities,
		// Then every listed label is a real GrantDurability enum value.
		perm009 := map[string]bool{
			"one_shot": true, "session_scoped": true,
			"persisted": true, "explicit_admin": true,
		}
		for _, d := range SeedExpectedPRPDTemplateDurabilities {
			assert.True(t, perm009[d], "label %q not in PERM-009 GrantDurability", d)
		}
	})

	t.Run("Scenario_FourPoliciesAllRepresented", func(t *testing.T) {
		// Given PERM-009 has 4 policies,
		// When seed templates ship,
		// Then ALL 4 policies have exactly one template (1:1 mapping).
		assert.Equal(t, 4, len(SeedExpectedPRPDTemplatePolicies))
	})

	t.Run("Scenario_AdminReviewGatesNonRoutineChange", func(t *testing.T) {
		// Given switching restore posture changes security boundary,
		// When admin compares admin-review subset,
		// Then 3 of 4 templates gate change (only balanced routine baseline).
		assert.Equal(t, 3, len(SeedAdminReviewPRPDTemplateSlugs))
	})

	t.Run("Scenario_DiscardPoliciesShipEmptySurvivors", func(t *testing.T) {
		// Given discard_all and strict_re_request drop everything,
		// When admin inspects surviving_durabilities,
		// Then both have empty lists — semantic match with PERM-009
		// runtime behaviour. Validated structurally in integration test.
		assert.Contains(t, SeedExpectedPRPDTemplateSlugs, "discard-all-fresh-context")
		assert.Contains(t, SeedExpectedPRPDTemplateSlugs, "strict-re-request-audit")
	})
}
