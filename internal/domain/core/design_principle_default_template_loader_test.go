package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedDesignPrinciple_ExpectedRowCount(t *testing.T) {
	assert.Equal(t, 13, SeedExpectedDesignPrincipleRowCount)
}

func TestSeedDesignPrinciple_SlugCountMatchesRowCount(t *testing.T) {
	assert.Equal(t, SeedExpectedDesignPrincipleRowCount, len(SeedExpectedDesignPrincipleSlugs))
}

func TestSeedDesignPrinciple_ContainsDenyFirst(t *testing.T) {
	assert.Contains(t, SeedExpectedDesignPrincipleSlugs, "deny_first_human_escalation")
}

func TestSeedDesignPrinciple_ContainsValuesOverRules(t *testing.T) {
	assert.Contains(t, SeedExpectedDesignPrincipleSlugs, "values_over_rules")
}

func TestSeedDesignPrinciple_ContainsGracefulRecovery(t *testing.T) {
	assert.Contains(t, SeedExpectedDesignPrincipleSlugs, "graceful_recovery_resilience")
}

func TestSeedDesignPrinciple_ContainsIsolatedSubagentBoundaries(t *testing.T) {
	assert.Contains(t, SeedExpectedDesignPrincipleSlugs, "isolated_subagent_boundaries")
}

func TestSeedDesignPrinciple_ContainsContextAsScarceResource(t *testing.T) {
	assert.Contains(t, SeedExpectedDesignPrincipleSlugs, "context_as_scarce_resource")
}

func TestSeedDesignPrinciple_ThreeSafetyAuthorityPrinciples(t *testing.T) {
	assert.Equal(t, 3, len(SeedDesignPrincipleSafetyAuthorityServingSlugs))
	assert.Contains(t, SeedDesignPrincipleSafetyAuthorityServingSlugs, "deny_first_human_escalation")
	assert.Contains(t, SeedDesignPrincipleSafetyAuthorityServingSlugs, "defense_in_depth_layered")
}

func TestSeedDesignPrinciple_SevenCapabilityPrinciples(t *testing.T) {
	assert.Equal(t, 7, len(SeedDesignPrincipleCapabilityServingSlugs))
	assert.Contains(t, SeedDesignPrincipleCapabilityServingSlugs, "values_over_rules")
	assert.Contains(t, SeedDesignPrincipleCapabilityServingSlugs, "reversibility_weighted_risk")
}

func TestSeedDesignPrinciple_SubsetsAreInCanonicalList(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedDesignPrincipleSlugs {
		all[s] = true
	}
	for _, s := range SeedDesignPrincipleSafetyAuthorityServingSlugs {
		assert.True(t, all[s], "safety+authority slug %q not in canonical list", s)
	}
	for _, s := range SeedDesignPrincipleCapabilityServingSlugs {
		assert.True(t, all[s], "capability slug %q not in canonical list", s)
	}
}
