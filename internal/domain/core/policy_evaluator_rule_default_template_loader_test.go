package core

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePERRTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedPERRTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedPERRTemplateRowCount)
}

func TestCorePERRTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPERRTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCorePERRTemplate_RuleIDsMatchPERM007Regex(t *testing.T) {
	// Cross-feature invariant: every rule_id must satisfy PERM-007 kebab regex.
	for _, s := range SeedExpectedPERRTemplateSlugs {
		assert.True(t, PolicyEvaluatorRuleIDRE.MatchString(s),
			"rule_id %q must match PERM-007 kebab regex", s)
	}
}

func TestCorePERRTemplate_DecisionsMatchPERM007EnumByteForByte(t *testing.T) {
	expected := map[string]bool{
		"deny": true, "escalate": true,
		// Note: allow and abstain not used in starter set; classifier
		// fallback errs on the side of caution.
	}
	for _, d := range SeedExpectedPERRTemplateDecisions {
		assert.True(t, expected[d], "decision %q outside used subset", d)
	}
}

func TestCorePERRTemplate_ConfidencesMatchPERM007EnumByteForByte(t *testing.T) {
	expected := map[string]bool{"high": true, "medium": true}
	for _, c := range SeedExpectedPERRTemplateConfidences {
		assert.True(t, expected[c], "confidence %q outside used subset", c)
	}
}

func TestCorePERRTemplate_RiskKindsClosedSet(t *testing.T) {
	expected := map[string]bool{
		"destructive_io": true, "prod_data_mutation": true,
		"secret_exposure": true, "network_egress": true,
		"pii_compliance": true, "privilege_escalation": true,
	}
	for _, r := range SeedExpectedPERRTemplateRiskKinds {
		assert.True(t, expected[r], "risk_kind %q outside closed set", r)
	}
	assert.Equal(t, 6, len(SeedExpectedPERRTemplateRiskKinds))
}

func TestCorePERRTemplate_RiskKindsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range SeedExpectedPERRTemplateRiskKinds {
		assert.False(t, seen[r])
		seen[r] = true
	}
}

func TestCorePERRTemplate_DenyAndEscalatePartitionExactlyCoverAll(t *testing.T) {
	combined := append([]string{}, SeedDenyDecisionPERRTemplateSlugs...)
	combined = append(combined, SeedEscalateDecisionPERRTemplateSlugs...)
	assert.ElementsMatch(t, SeedExpectedPERRTemplateSlugs, combined,
		"deny ∪ escalate must equal all rules — no overlap allowed")
}

func TestCorePERRTemplate_DenySubsetIsTwoHardDenies(t *testing.T) {
	assert.ElementsMatch(t,
		[]string{"deny-destructive-shell", "deny-secret-path-access"},
		SeedDenyDecisionPERRTemplateSlugs)
}

func TestCorePERRTemplate_AllRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedPERRTemplateSlugs,
		SeedRecommendedPERRTemplateSlugs)
}

func TestCorePERRTemplate_OneToOneRuleToRiskKind(t *testing.T) {
	assert.Equal(t,
		len(SeedExpectedPERRTemplateRiskKinds),
		SeedExpectedPERRTemplateRowCount)
}

func TestCorePERRTemplate_PolicyEvaluatorRuleIDREMatchesPERM007Pattern(t *testing.T) {
	// Sanity: the regex constant is the same shape PERM-007 expects.
	sample := regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
	assert.Equal(t, sample.String(), PolicyEvaluatorRuleIDRE.String())
}
