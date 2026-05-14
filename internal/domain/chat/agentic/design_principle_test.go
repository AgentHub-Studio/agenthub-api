package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDesignPrincipleRegistry_ThirteenPrinciplesFromTable1(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	assert.Equal(t, 13, len(r.AllPrinciples()))
}

func TestDesignPrincipleRegistry_ProfileDenyFirstHumanEscalation(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	p, ok := r.Profile(PrincipleDenyFirstHumanEscalation)
	assert.True(t, ok)
	assert.Equal(t, 2, len(p.ValuesServed))
	assert.Contains(t, p.ValuesServed, DesignValueHumanAuthority)
	assert.Contains(t, p.ValuesServed, DesignValueSafety)
	assert.Contains(t, p.ReferencedSections, "5")
}

func TestDesignPrincipleRegistry_ProfileGraduatedTrustSpectrum(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	p, ok := r.Profile(PrincipleGraduatedTrustSpectrum)
	assert.True(t, ok)
	assert.Contains(t, p.ValuesServed, DesignValueAdaptability)
	assert.Contains(t, p.ReferencedSections, "5")
}

func TestDesignPrincipleRegistry_ProfileDefenseInDepthHasThreeValues(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	p, ok := r.Profile(PrincipleDefenseInDepthLayered)
	assert.True(t, ok)
	assert.Equal(t, 3, len(p.ValuesServed))
	assert.Contains(t, p.ValuesServed, DesignValueSafety)
	assert.Contains(t, p.ValuesServed, DesignValueReliability)
}

func TestDesignPrincipleRegistry_ProfileIsolatedSubagentBoundariesHasThreeValues(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	p, ok := r.Profile(PrincipleIsolatedSubagentBoundaries)
	assert.True(t, ok)
	assert.Equal(t, 3, len(p.ValuesServed))
	assert.Contains(t, p.ValuesServed, DesignValueReliability)
	assert.Contains(t, p.ValuesServed, DesignValueSafety)
	assert.Contains(t, p.ValuesServed, DesignValueCapability)
}

func TestDesignPrincipleRegistry_UnknownIDReturnsFalse(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	_, ok := r.Profile("does_not_exist")
	assert.False(t, ok)
}

func TestDesignPrincipleRegistry_IsValidPrinciple_KnownTrue(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	assert.True(t, r.IsValidPrinciple(PrincipleValuesOverRules))
}

func TestDesignPrincipleRegistry_IsValidPrinciple_UnknownFalse(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	assert.False(t, r.IsValidPrinciple("invented"))
}

func TestDesignPrincipleRegistry_PrinciplesServingAuthority_SixPrinciples(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	// Table 1: authority appears in deny_first, graduated_trust, defense_in_depth,
	// externalized_policy, append_only, values_over_rules, transparent_file_based
	principles := r.PrinciplesServingValue(DesignValueHumanAuthority)
	assert.GreaterOrEqual(t, len(principles), 5)
}

func TestDesignPrincipleRegistry_PrinciplesServingCapability_MultiplePrinciples(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	principles := r.PrinciplesServingValue(DesignValueCapability)
	assert.GreaterOrEqual(t, len(principles), 5)
}

func TestDesignPrincipleRegistry_AllPrinciplesHaveNonEmptyFields(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	for _, p := range r.AllPrinciples() {
		assert.NotEmpty(t, p.Label, "principle %s has empty label", p.ID)
		assert.NotEmpty(t, p.DesignQuestion, "principle %s has empty design question", p.ID)
		assert.NotEmpty(t, p.ValuesServed, "principle %s has no values served", p.ID)
		assert.NotEmpty(t, p.ReferencedSections, "principle %s has no sections", p.ID)
	}
}

func TestDesignPrincipleRegistry_AllPrincipleValuesAreKnownDesignValues(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	known := map[DesignValue]bool{}
	for _, v := range AllDesignValues {
		known[v] = true
	}
	for _, p := range r.AllPrinciples() {
		for _, v := range p.ValuesServed {
			assert.True(t, known[v], "principle %s references unknown value %q", p.ID, v)
		}
	}
}

func TestDesignPrincipleRegistry_Table1RowOrderIsPreserved(t *testing.T) {
	r := NewDesignPrincipleRegistry()
	all := r.AllPrinciples()
	assert.Equal(t, PrincipleDenyFirstHumanEscalation, all[0].ID)
	assert.Equal(t, PrincipleGracefulRecoveryResilience, all[12].ID)
}
