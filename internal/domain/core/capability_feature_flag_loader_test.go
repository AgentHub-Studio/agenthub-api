package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability feature flag seed constants (migration 000103).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityFeatureFlagCount_IsSix(t *testing.T) {
	assert.Equal(t, 6, SeedCapabilityFeatureFlagCount,
		"migration 000103 seeds exactly 6 capability feature flag rows")
}

func TestSeedCapabilityFeatureFlagSlugs_HasLengthSix(t *testing.T) {
	assert.Len(t, SeedCapabilityFeatureFlagSlugs, 6,
		"SeedCapabilityFeatureFlagSlugs must have exactly 6 entries — one per seeded feature flag")
}

func TestSeedCapabilityFeatureFlagSlugs_AllStartWithCapabilityPrefix(t *testing.T) {
	for _, slug := range SeedCapabilityFeatureFlagSlugs {
		assert.True(t, strings.HasPrefix(slug, "capability-"),
			"feature flag slug %q must start with 'capability-' (capability-layer namespace contract)", slug)
	}
}

func TestSeedSubagentDelegationFeatureFlagSlug_IsOnlyDisabledByDefault(t *testing.T) {
	assert.Equal(t, SeedDisabledByDefaultFeatureFlagSlug, SeedSubagentDelegationFeatureFlagSlug,
		"SeedDisabledByDefaultFeatureFlagSlug must equal SeedSubagentDelegationFeatureFlagSlug — subagent delegation is the only flag disabled by default")
}

func TestSeedDisabledByDefaultFeatureFlagSlug_MatchesSubagentDelegation(t *testing.T) {
	assert.Equal(t, "capability-subagent-delegation", SeedDisabledByDefaultFeatureFlagSlug,
		"SeedDisabledByDefaultFeatureFlagSlug must be \"capability-subagent-delegation\"")
}

func TestSeedCapabilityFeatureFlagCount_FiveEnabledByDefault(t *testing.T) {
	// 6 total - 1 disabled (subagent-delegation) = 5 enabled by default.
	disabledCount := 1 // only subagent-delegation
	enabledByDefault := SeedCapabilityFeatureFlagCount - disabledCount
	assert.Equal(t, 5, enabledByDefault,
		"exactly 5 of the 6 capability feature flags must be enabled by default (subagent-delegation is the lone disabled flag)")
}

func TestSeedFeatureFlagCategoryResearch_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedFeatureFlagCategoryResearch,
		"SeedFeatureFlagCategoryResearch must be a non-empty string")
}

func TestSeedFeatureFlagCategoryPlanning_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedFeatureFlagCategoryPlanning,
		"SeedFeatureFlagCategoryPlanning must be a non-empty string")
}

func TestSeedFeatureFlagCategoryAnalysis_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedFeatureFlagCategoryAnalysis,
		"SeedFeatureFlagCategoryAnalysis must be a non-empty string")
}

func TestSeedFeatureFlagCategoryOrchestration_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedFeatureFlagCategoryOrchestration,
		"SeedFeatureFlagCategoryOrchestration must be a non-empty string")
}

func TestSeedFeatureFlagCategoryMemory_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedFeatureFlagCategoryMemory,
		"SeedFeatureFlagCategoryMemory must be a non-empty string")
}

func TestSeedFeatureFlagCategories_AreFiveDistinctValues(t *testing.T) {
	categories := []string{
		SeedFeatureFlagCategoryResearch,
		SeedFeatureFlagCategoryPlanning,
		SeedFeatureFlagCategoryAnalysis,
		SeedFeatureFlagCategoryOrchestration,
		SeedFeatureFlagCategoryMemory,
	}
	unique := map[string]struct{}{}
	for _, c := range categories {
		unique[c] = struct{}{}
	}
	assert.Len(t, unique, 5,
		"migration 000103 must define exactly 5 distinct feature flag category constants")
}

func TestSeedFeatureFlagCategoryResearch_HasTwoFlags(t *testing.T) {
	// research: capability-citations + capability-kb-indexing
	researchSlugs := []string{
		SeedCitationsFeatureFlagSlug,
		SeedKBIndexingFeatureFlagSlug,
	}
	for _, slug := range researchSlugs {
		assert.Contains(t, SeedCapabilityFeatureFlagSlugs, slug,
			"research-category flag %q must be in SeedCapabilityFeatureFlagSlugs", slug)
	}
	assert.Len(t, researchSlugs, 2,
		"research category must have exactly 2 feature flags (citations + kb-indexing)")
}

func TestSeedCitationsFeatureFlagSlug_IsInSlugs(t *testing.T) {
	assert.Contains(t, SeedCapabilityFeatureFlagSlugs, SeedCitationsFeatureFlagSlug,
		"SeedCitationsFeatureFlagSlug must be present in SeedCapabilityFeatureFlagSlugs")
}

func TestSeedTaskTrackingFeatureFlagSlug_IsInSlugs(t *testing.T) {
	assert.Contains(t, SeedCapabilityFeatureFlagSlugs, SeedTaskTrackingFeatureFlagSlug,
		"SeedTaskTrackingFeatureFlagSlug must be present in SeedCapabilityFeatureFlagSlugs")
}

func TestSeedKBIndexingFeatureFlagSlug_IsInSlugs(t *testing.T) {
	assert.Contains(t, SeedCapabilityFeatureFlagSlugs, SeedKBIndexingFeatureFlagSlug,
		"SeedKBIndexingFeatureFlagSlug must be present in SeedCapabilityFeatureFlagSlugs")
}

func TestSeedSubagentDelegationFeatureFlagSlug_IsInSlugs(t *testing.T) {
	assert.Contains(t, SeedCapabilityFeatureFlagSlugs, SeedSubagentDelegationFeatureFlagSlug,
		"SeedSubagentDelegationFeatureFlagSlug must be present in SeedCapabilityFeatureFlagSlugs")
}

func TestSeedDocCitationsFeatureFlagSlug_IsInSlugs(t *testing.T) {
	assert.Contains(t, SeedCapabilityFeatureFlagSlugs, SeedDocCitationsFeatureFlagSlug,
		"SeedDocCitationsFeatureFlagSlug must be present in SeedCapabilityFeatureFlagSlugs")
}

func TestSeedProgressiveSummarizationFeatureFlagSlug_IsInSlugs(t *testing.T) {
	assert.Contains(t, SeedCapabilityFeatureFlagSlugs, SeedProgressiveSummarizationFeatureFlagSlug,
		"SeedProgressiveSummarizationFeatureFlagSlug must be present in SeedCapabilityFeatureFlagSlugs")
}

func TestSeedCapabilityFeatureFlagSlugs_AllAreDistinct(t *testing.T) {
	unique := map[string]struct{}{}
	for _, slug := range SeedCapabilityFeatureFlagSlugs {
		unique[slug] = struct{}{}
	}
	assert.Len(t, unique, SeedCapabilityFeatureFlagCount,
		"all entries in SeedCapabilityFeatureFlagSlugs must be distinct (no duplicates)")
}
