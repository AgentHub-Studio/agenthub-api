package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedDesignValueTension_ExpectedRowCount(t *testing.T) {
	assert.Equal(t, 5, SeedExpectedDesignValueTensionRowCount)
}

func TestSeedDesignValueTension_SlugCountMatchesRowCount(t *testing.T) {
	assert.Equal(t, SeedExpectedDesignValueTensionRowCount, len(SeedExpectedDesignValueTensionSlugs))
}

func TestSeedDesignValueTension_SlugsContainAuthoritySafety(t *testing.T) {
	assert.Contains(t, SeedExpectedDesignValueTensionSlugs, "authority_safety")
}

func TestSeedDesignValueTension_SlugsContainSafetyCapability(t *testing.T) {
	assert.Contains(t, SeedExpectedDesignValueTensionSlugs, "safety_capability")
}

func TestSeedDesignValueTension_SlugsContainAdaptabilitySafety(t *testing.T) {
	assert.Contains(t, SeedExpectedDesignValueTensionSlugs, "adaptability_safety")
}

func TestSeedDesignValueTension_SlugsContainCapabilityAdaptability(t *testing.T) {
	assert.Contains(t, SeedExpectedDesignValueTensionSlugs, "capability_adaptability")
}

func TestSeedDesignValueTension_SlugsContainCapabilityReliability(t *testing.T) {
	assert.Contains(t, SeedExpectedDesignValueTensionSlugs, "capability_reliability")
}

func TestSeedDesignValueTension_ThreeCapabilityTensions(t *testing.T) {
	assert.Equal(t, 3, len(SeedDesignValueTensionCapabilitySlugs))
	assert.Contains(t, SeedDesignValueTensionCapabilitySlugs, "safety_capability")
	assert.Contains(t, SeedDesignValueTensionCapabilitySlugs, "capability_adaptability")
	assert.Contains(t, SeedDesignValueTensionCapabilitySlugs, "capability_reliability")
}

func TestSeedDesignValueTension_ThreeSafetyTensions(t *testing.T) {
	assert.Equal(t, 3, len(SeedDesignValueTensionSafetySlugs))
	assert.Contains(t, SeedDesignValueTensionSafetySlugs, "authority_safety")
	assert.Contains(t, SeedDesignValueTensionSafetySlugs, "safety_capability")
	assert.Contains(t, SeedDesignValueTensionSafetySlugs, "adaptability_safety")
}

func TestSeedDesignValueTension_SubsetSlugsAreInCanonicalList(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedDesignValueTensionSlugs {
		all[s] = true
	}
	for _, s := range SeedDesignValueTensionCapabilitySlugs {
		assert.True(t, all[s], "capability slug %q not in canonical list", s)
	}
	for _, s := range SeedDesignValueTensionSafetySlugs {
		assert.True(t, all[s], "safety slug %q not in canonical list", s)
	}
}
