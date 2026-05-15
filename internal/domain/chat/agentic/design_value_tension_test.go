package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDesignValueTensionRegistry_FiveTensionsFromTable4(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	assert.Equal(t, 5, len(r.AllTensions()))
}

func TestDesignValueTensionRegistry_ProfileAuthoritySafety(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	p, ok := r.Profile(TensionAuthoritySafety)
	assert.True(t, ok)
	assert.Equal(t, DesignValueHumanAuthority, p.Value1)
	assert.Equal(t, DesignValueSafety, p.Value2)
	assert.Contains(t, p.TensionLabel, "fatigue")
}

func TestDesignValueTensionRegistry_ProfileSafetyCapability(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	p, ok := r.Profile(TensionSafetyCapability)
	assert.True(t, ok)
	assert.Equal(t, DesignValueSafety, p.Value1)
	assert.Equal(t, DesignValueCapability, p.Value2)
	assert.Contains(t, p.TensionLabel, "defense")
}

func TestDesignValueTensionRegistry_ProfileCapabilityAdaptability(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	p, ok := r.Profile(TensionCapabilityAdaptability)
	assert.True(t, ok)
	assert.Equal(t, DesignValueCapability, p.Value1)
	assert.Equal(t, DesignValueAdaptability, p.Value2)
	assert.Contains(t, p.TensionLabel, "disruption")
}

func TestDesignValueTensionRegistry_ProfileCapabilityReliability(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	p, ok := r.Profile(TensionCapabilityReliability)
	assert.True(t, ok)
	assert.Equal(t, DesignValueCapability, p.Value1)
	assert.Equal(t, DesignValueReliability, p.Value2)
	assert.Contains(t, p.TensionLabel, "coherence")
}

func TestDesignValueTensionRegistry_ProfileAdaptabilitySafety(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	p, ok := r.Profile(TensionAdaptabilitySafety)
	assert.True(t, ok)
	assert.Equal(t, DesignValueAdaptability, p.Value1)
	assert.Equal(t, DesignValueSafety, p.Value2)
	assert.Contains(t, p.TensionLabel, "attack")
}

func TestDesignValueTensionRegistry_UnknownIDReturnsFalse(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	_, ok := r.Profile("does_not_exist")
	assert.False(t, ok)
}

func TestDesignValueTensionRegistry_IsValidTension_KnownReturnsTrue(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	assert.True(t, r.IsValidTension(TensionCapabilityReliability))
}

func TestDesignValueTensionRegistry_IsValidTension_UnknownReturnsFalse(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	assert.False(t, r.IsValidTension("not_a_real_tension"))
}

func TestDesignValueTensionRegistry_TensionsInvolvingCapability_ThreeTensions(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	tensions := r.TensionsInvolvingValue(DesignValueCapability)
	assert.Equal(t, 3, len(tensions))
}

func TestDesignValueTensionRegistry_TensionsInvolvingSafety_ThreeTensions(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	tensions := r.TensionsInvolvingValue(DesignValueSafety)
	assert.Equal(t, 3, len(tensions))
}

func TestDesignValueTensionRegistry_TensionsInvolvingReliability_OneTension(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	tensions := r.TensionsInvolvingValue(DesignValueReliability)
	assert.Equal(t, 1, len(tensions))
	assert.Equal(t, TensionCapabilityReliability, tensions[0].ID)
}

func TestDesignValueTensionRegistry_AllTensionsHaveNonEmptyLabels(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	for _, p := range r.AllTensions() {
		assert.NotEmpty(t, p.TensionLabel, "tension %s has empty label", p.ID)
		assert.NotEmpty(t, p.EvidenceSummary, "tension %s has empty evidence", p.ID)
	}
}

func TestDesignValueTensionRegistry_AllTensionValuesAreKnownDesignValues(t *testing.T) {
	r := NewDesignValueTensionRegistry()
	knownValues := map[DesignValue]bool{}
	for _, v := range AllDesignValues {
		knownValues[v] = true
	}
	for _, p := range r.AllTensions() {
		assert.True(t, knownValues[p.Value1], "tension %s Value1 %q not in AllDesignValues", p.ID, p.Value1)
		assert.True(t, knownValues[p.Value2], "tension %s Value2 %q not in AllDesignValues", p.ID, p.Value2)
	}
}
