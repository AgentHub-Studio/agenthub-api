package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability onboarding checklist seed constants (migration 000106).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityOnboardingChecklistCount_IsFive(t *testing.T) {
	assert.Equal(t, 5, SeedCapabilityOnboardingChecklistCount,
		"migration 000106 seeds exactly 5 capability onboarding checklist rows (one per guided step)")
}

func TestSeedCapabilityOnboardingChecklistSlugs_HasLengthFive(t *testing.T) {
	assert.Len(t, SeedCapabilityOnboardingChecklistSlugs, 5,
		"SeedCapabilityOnboardingChecklistSlugs must have exactly 5 entries — one per onboarding step")
}

func TestSeedCapabilityOnboardingChecklistSlugs_AllStartWithOnboarding(t *testing.T) {
	for _, slug := range SeedCapabilityOnboardingChecklistSlugs {
		assert.True(t, strings.HasPrefix(slug, "onboarding-"),
			"onboarding slug %q must start with 'onboarding-' (namespace contract)", slug)
	}
}

func TestSeedCapabilityOnboardingChecklistSlugs_AllAreDistinct(t *testing.T) {
	unique := map[string]struct{}{}
	for _, slug := range SeedCapabilityOnboardingChecklistSlugs {
		unique[slug] = struct{}{}
	}
	assert.Len(t, unique, 5,
		"all entries in SeedCapabilityOnboardingChecklistSlugs must be distinct (no duplicates)")
}

func TestSeedOnboardingBlockingStepCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedOnboardingBlockingStepCount,
		"SeedOnboardingBlockingStepCount must be 3 — steps 1-3 are mandatory for a working agent")
}

func TestSeedOnboardingNonBlockingStepCount_IsTwo(t *testing.T) {
	assert.Equal(t, 2, SeedOnboardingNonBlockingStepCount,
		"SeedOnboardingNonBlockingStepCount must be 2 — steps 4-5 are optional extras")
}

func TestSeedOnboardingBlockingAndNonBlocking_SumToTotal(t *testing.T) {
	sum := SeedOnboardingBlockingStepCount + SeedOnboardingNonBlockingStepCount
	assert.Equal(t, SeedCapabilityOnboardingChecklistCount, sum,
		"blocking (%d) + non-blocking (%d) must equal total count (%d)",
		SeedOnboardingBlockingStepCount, SeedOnboardingNonBlockingStepCount,
		SeedCapabilityOnboardingChecklistCount)
}

func TestSeedOnboardingResourceTypes_HasFiveEntries(t *testing.T) {
	assert.Len(t, SeedOnboardingResourceTypes, 5,
		"SeedOnboardingResourceTypes must have exactly 5 entries — one per onboarding step")
}

func TestSeedOnboardingResourceTypes_AllNonEmpty(t *testing.T) {
	for _, rt := range SeedOnboardingResourceTypes {
		assert.NotEmpty(t, rt, "resource_type must not be empty")
	}
}

func TestSeedOnboardingResourceTypes_AllDistinct(t *testing.T) {
	unique := map[string]struct{}{}
	for _, rt := range SeedOnboardingResourceTypes {
		unique[rt] = struct{}{}
	}
	assert.Len(t, unique, 5,
		"all entries in SeedOnboardingResourceTypes must be distinct (each step targets a different entity)")
}

func TestSeedOnboardingConnectLLMSlug_Value(t *testing.T) {
	assert.Equal(t, "onboarding-connect-llm", SeedOnboardingConnectLLMSlug,
		"SeedOnboardingConnectLLMSlug must equal \"onboarding-connect-llm\"")
}

func TestSeedOnboardingTestRunSlug_Value(t *testing.T) {
	assert.Equal(t, "onboarding-test-run", SeedOnboardingTestRunSlug,
		"SeedOnboardingTestRunSlug must equal \"onboarding-test-run\"")
}

func TestSeedOnboardingConnectLLMSlug_IsFirstInSlugs(t *testing.T) {
	assert.Equal(t, SeedOnboardingConnectLLMSlug, SeedCapabilityOnboardingChecklistSlugs[0],
		"SeedOnboardingConnectLLMSlug must be the first entry in SeedCapabilityOnboardingChecklistSlugs (step 1)")
}

func TestSeedOnboardingTestRunSlug_IsLastInSlugs(t *testing.T) {
	last := SeedCapabilityOnboardingChecklistSlugs[len(SeedCapabilityOnboardingChecklistSlugs)-1]
	assert.Equal(t, SeedOnboardingTestRunSlug, last,
		"SeedOnboardingTestRunSlug must be the last entry in SeedCapabilityOnboardingChecklistSlugs (step 5)")
}

func TestSeedOnboardingConfigureKBSlug_IsNotInBlockingSet(t *testing.T) {
	// configure-kb (step 4) is optional — not blocking.
	// Verify the slug exists and is distinct from the first 3 (blocking) slugs.
	blockingSlugs := SeedCapabilityOnboardingChecklistSlugs[:SeedOnboardingBlockingStepCount]
	for _, s := range blockingSlugs {
		assert.NotEqual(t, SeedOnboardingConfigureKBSlug, s,
			"SeedOnboardingConfigureKBSlug must not appear among the %d blocking steps", SeedOnboardingBlockingStepCount)
	}
}

func TestSeedOnboardingTestRunSlug_IsNotInBlockingSet(t *testing.T) {
	// test-run (step 5) is optional — not blocking.
	blockingSlugs := SeedCapabilityOnboardingChecklistSlugs[:SeedOnboardingBlockingStepCount]
	for _, s := range blockingSlugs {
		assert.NotEqual(t, SeedOnboardingTestRunSlug, s,
			"SeedOnboardingTestRunSlug must not appear among the %d blocking steps", SeedOnboardingBlockingStepCount)
	}
}

func TestSeedOnboardingChecklistCount_EqualsSlugsLength(t *testing.T) {
	assert.Equal(t, SeedCapabilityOnboardingChecklistCount, len(SeedCapabilityOnboardingChecklistSlugs),
		"SeedCapabilityOnboardingChecklistCount must equal len(SeedCapabilityOnboardingChecklistSlugs)")
}
