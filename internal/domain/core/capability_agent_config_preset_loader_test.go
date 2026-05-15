package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability agent config preset seed constants (migration 000098).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityAgentConfigPresetCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedCapabilityAgentConfigPresetCount,
		"migration 000098 seeds exactly 3 capability agent config presets (one per capability agent)")
}

func TestSeedCapabilityAgentConfigPresetCount_MatchesSlugList(t *testing.T) {
	assert.Equal(t, SeedCapabilityAgentConfigPresetCount, len(SeedCapabilityAgentConfigPresetSlugs),
		"SeedCapabilityAgentConfigPresetCount must match len(SeedCapabilityAgentConfigPresetSlugs)")
}

func TestSeedCapabilityAgentConfigPresetSlugs_HasLengthThree(t *testing.T) {
	assert.Len(t, SeedCapabilityAgentConfigPresetSlugs, 3,
		"slug list must have exactly 3 entries — one per capability agent")
}

func TestSeedCapabilityAgentConfigPresetSlugs_ContainsResearchSlug(t *testing.T) {
	slugSet := toStringSet(SeedCapabilityAgentConfigPresetSlugs)
	assert.True(t, slugSet[SeedResearchAgentConfigSlug],
		"SeedResearchAgentConfigSlug %q must be in SeedCapabilityAgentConfigPresetSlugs",
		SeedResearchAgentConfigSlug)
}

func TestSeedCapabilityAgentConfigPresetSlugs_ContainsAnalysisSlug(t *testing.T) {
	slugSet := toStringSet(SeedCapabilityAgentConfigPresetSlugs)
	assert.True(t, slugSet[SeedAnalysisAgentConfigSlug],
		"SeedAnalysisAgentConfigSlug %q must be in SeedCapabilityAgentConfigPresetSlugs",
		SeedAnalysisAgentConfigSlug)
}

func TestSeedCapabilityAgentConfigPresetSlugs_ContainsPlanningSlug(t *testing.T) {
	slugSet := toStringSet(SeedCapabilityAgentConfigPresetSlugs)
	assert.True(t, slugSet[SeedPlanningAgentConfigSlug],
		"SeedPlanningAgentConfigSlug %q must be in SeedCapabilityAgentConfigPresetSlugs",
		SeedPlanningAgentConfigSlug)
}

func TestSeedCapabilityAgentConfigPresetSlugs_AllStartWithCapabilityPrefix(t *testing.T) {
	for _, slug := range SeedCapabilityAgentConfigPresetSlugs {
		assert.True(t, strings.HasPrefix(slug, "capability-"),
			"capability agent config preset slug %q must start with 'capability-' (namespace contract)", slug)
	}
}

func TestSeedCapabilityAgentConfigPresetSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedCapabilityAgentConfigPresetSlugs {
		assert.False(t, seen[slug], "duplicate capability agent config preset slug %q", slug)
		seen[slug] = true
	}
}

func TestSeedCapabilityAgentConfigTemperatureMap_HasAllThreeEntries(t *testing.T) {
	assert.Len(t, SeedCapabilityAgentConfigTemperatures, 3,
		"temperature map must have exactly 3 entries — one per capability agent config preset")
}

func TestSeedCapabilityAgentConfigTemperatureMap_AnalysisHasLowestTemperature(t *testing.T) {
	research := SeedCapabilityAgentConfigTemperatures[SeedResearchAgentConfigSlug]
	analysis := SeedCapabilityAgentConfigTemperatures[SeedAnalysisAgentConfigSlug]
	planning := SeedCapabilityAgentConfigTemperatures[SeedPlanningAgentConfigSlug]
	assert.Less(t, analysis, research,
		"analysis temperature (%.1f) must be lower than research temperature (%.1f) for consistent, reproducible outputs",
		analysis, research)
	assert.Less(t, analysis, planning,
		"analysis temperature (%.1f) must be lower than planning temperature (%.1f) for structured findings",
		analysis, planning)
}

func TestSeedCapabilityAgentConfigMaxTokens_AnalysisHasHighestMaxTokens(t *testing.T) {
	research := SeedCapabilityAgentConfigMaxTokens[SeedResearchAgentConfigSlug]
	analysis := SeedCapabilityAgentConfigMaxTokens[SeedAnalysisAgentConfigSlug]
	planning := SeedCapabilityAgentConfigMaxTokens[SeedPlanningAgentConfigSlug]
	assert.Greater(t, analysis, research,
		"analysis max_tokens (%d) must be greater than research max_tokens (%d) for deep document corpora",
		analysis, research)
	assert.Greater(t, analysis, planning,
		"analysis max_tokens (%d) must be greater than planning max_tokens (%d)",
		analysis, planning)
}

func TestSeedCapabilityAgentConfigModelProvider_IsAnthropic(t *testing.T) {
	assert.Equal(t, "anthropic", SeedCapabilityAgentConfigModelProvider,
		"all capability agent config presets must use the 'anthropic' provider")
}

func TestSeedCapabilityAgentConfigModelID_IsClaudeSonnet46(t *testing.T) {
	assert.Equal(t, "claude-sonnet-4-6", SeedCapabilityAgentConfigModelID,
		"all capability agent config presets must reference claude-sonnet-4-6")
}

// toStringSet converts a slice of strings into a membership set for fast lookup.
// Scoped to this file — same helper used only within this test file.
func toStringSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
