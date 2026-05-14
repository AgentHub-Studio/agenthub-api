package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCorePERRTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsSensibleFallbackRulesAtBoot", func(t *testing.T) {
		// Given fresh tenants must have classifier fallback behavior
		// without authoring rules,
		// When admin queries ah_core for starter classifier rules,
		// Then 6 recommended rules surface (deny + escalate, no allow).
		assert.Equal(t, 6, len(SeedRecommendedPERRTemplateSlugs))
	})

	t.Run("Scenario_StarterRulesErrOnSideOfCaution", func(t *testing.T) {
		// Given classifier fallback must err on the side of safety,
		// When admin inspects starter decisions,
		// Then only deny + escalate are used (no allow rules in defaults).
		set := map[string]bool{}
		for _, d := range SeedExpectedPERRTemplateDecisions {
			set[d] = true
		}
		assert.True(t, set["deny"])
		assert.True(t, set["escalate"])
		assert.False(t, set["allow"])
	})

	t.Run("Scenario_DestructiveShellHardDeniedByDefault", func(t *testing.T) {
		// Given rm/drop/truncate/force-push are universally destructive,
		// When the deny-destructive-shell rule fires,
		// Then decision=deny with high confidence.
		set := map[string]bool{}
		for _, s := range SeedDenyDecisionPERRTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["deny-destructive-shell"])
	})

	t.Run("Scenario_SecretPathsHardDeniedByDefault", func(t *testing.T) {
		set := map[string]bool{}
		for _, s := range SeedDenyDecisionPERRTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["deny-secret-path-access"])
	})

	t.Run("Scenario_ProdDBWritesEscalateToHuman", func(t *testing.T) {
		set := map[string]bool{}
		for _, s := range SeedEscalateDecisionPERRTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["escalate-prod-db-writes"])
	})

	t.Run("Scenario_NetworkEgressMutationsEscalate", func(t *testing.T) {
		// Given outbound POST/PUT/DELETE may exfiltrate or mutate state,
		// When admin inspects starter rules,
		// Then escalate-network-egress is registered.
		set := map[string]bool{}
		for _, s := range SeedEscalateDecisionPERRTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["escalate-network-egress"])
	})

	t.Run("Scenario_PIIReadsEscalateForCompliance", func(t *testing.T) {
		// Given GDPR/LGPD treat PII reads as auditable events,
		// When admin inspects starter rules,
		// Then escalate-pii-read is registered.
		set := map[string]bool{}
		for _, s := range SeedEscalateDecisionPERRTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["escalate-pii-read"])
	})

	t.Run("Scenario_PrivilegeEscalationsEscalate", func(t *testing.T) {
		// Given installing extensions / granting privileges shouldn't
		// happen without admin approval,
		// When admin inspects starter rules,
		// Then escalate-installation-privileges is registered.
		set := map[string]bool{}
		for _, s := range SeedEscalateDecisionPERRTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["escalate-installation-privileges"])
	})

	t.Run("Scenario_RiskKindsCoverSixDistinctCategories", func(t *testing.T) {
		// Given the starter set covers 6 risk taxonomies,
		// When admin compares risk_kind,
		// Then 6 distinct kinds appear.
		assert.Equal(t, 6, len(SeedExpectedPERRTemplateRiskKinds))
	})

	t.Run("Scenario_DenyEscalatePartitionIsExhaustive", func(t *testing.T) {
		// Given every rule is either deny or escalate (no allow/abstain
		// in defaults),
		// When admin unions both subsets,
		// Then the result equals the full canonical list.
		combined := append([]string{}, SeedDenyDecisionPERRTemplateSlugs...)
		combined = append(combined, SeedEscalateDecisionPERRTemplateSlugs...)
		assert.ElementsMatch(t, SeedExpectedPERRTemplateSlugs, combined)
	})
}
