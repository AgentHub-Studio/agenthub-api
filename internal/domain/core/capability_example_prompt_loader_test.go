package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability example prompt seed constants (migration 000108).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityExamplePromptCount_IsTwelve(t *testing.T) {
	assert.Equal(t, 12, SeedCapabilityExamplePromptCount,
		"migration 000108 seeds exactly 12 capability example prompt rows (four per capability agent)")
}

func TestSeedCapabilityExamplePromptAgentSlugs_HasThreeEntries(t *testing.T) {
	assert.Len(t, SeedCapabilityExamplePromptAgentSlugs, 3,
		"SeedCapabilityExamplePromptAgentSlugs must have exactly 3 entries (researcher, analyst, planner)")
}

func TestSeedCapabilityExamplePromptAgentSlugs_AllStartWithCore(t *testing.T) {
	for _, slug := range SeedCapabilityExamplePromptAgentSlugs {
		assert.True(t, strings.HasPrefix(slug, "core-"),
			"agent slug %q must start with 'core-' (capability agent namespace contract)", slug)
	}
}

func TestSeedResearcherExamplePromptCount_IsFour(t *testing.T) {
	assert.Equal(t, 4, SeedResearcherExamplePromptCount,
		"SeedResearcherExamplePromptCount must be 4 — web/competitive/technical/market research")
}

func TestSeedAnalystExamplePromptCount_IsFour(t *testing.T) {
	assert.Equal(t, 4, SeedAnalystExamplePromptCount,
		"SeedAnalystExamplePromptCount must be 4 — doc summary/compare/extract/pattern analysis")
}

func TestSeedPlannerExamplePromptCount_IsFour(t *testing.T) {
	assert.Equal(t, 4, SeedPlannerExamplePromptCount,
		"SeedPlannerExamplePromptCount must be 4 — project/sprint/release/debug planning")
}

func TestSeedPerAgentExamplePromptCounts_SumToTotal(t *testing.T) {
	sum := SeedResearcherExamplePromptCount + SeedAnalystExamplePromptCount + SeedPlannerExamplePromptCount
	assert.Equal(t, SeedCapabilityExamplePromptCount, sum,
		"researcher (%d) + analyst (%d) + planner (%d) must equal total count (%d)",
		SeedResearcherExamplePromptCount, SeedAnalystExamplePromptCount, SeedPlannerExamplePromptCount,
		SeedCapabilityExamplePromptCount)
}

func TestSeedResearcherExamplePromptSlugs_HasFourEntries(t *testing.T) {
	assert.Len(t, SeedResearcherExamplePromptSlugs, 4,
		"SeedResearcherExamplePromptSlugs must have exactly 4 entries")
}

func TestSeedAnalystExamplePromptSlugs_HasFourEntries(t *testing.T) {
	assert.Len(t, SeedAnalystExamplePromptSlugs, 4,
		"SeedAnalystExamplePromptSlugs must have exactly 4 entries")
}

func TestSeedPlannerExamplePromptSlugs_HasFourEntries(t *testing.T) {
	assert.Len(t, SeedPlannerExamplePromptSlugs, 4,
		"SeedPlannerExamplePromptSlugs must have exactly 4 entries")
}

func TestSeedCapabilityExamplePromptSlugs_HasTwelveEntries(t *testing.T) {
	assert.Len(t, SeedCapabilityExamplePromptSlugs, 12,
		"SeedCapabilityExamplePromptSlugs must have exactly 12 entries (all agents combined)")
}

func TestSeedCapabilityExamplePromptSlugs_AllStartWithExample(t *testing.T) {
	for _, slug := range SeedCapabilityExamplePromptSlugs {
		assert.True(t, strings.HasPrefix(slug, "example-"),
			"example prompt slug %q must start with 'example-' (namespace contract)", slug)
	}
}

func TestSeedCapabilityExamplePromptSlugs_AllAreDistinct(t *testing.T) {
	unique := map[string]struct{}{}
	for _, slug := range SeedCapabilityExamplePromptSlugs {
		unique[slug] = struct{}{}
	}
	assert.Len(t, unique, 12,
		"all entries in SeedCapabilityExamplePromptSlugs must be distinct (no duplicates)")
}

func TestSeedCapabilityExamplePromptCount_EqualsSlugsLength(t *testing.T) {
	assert.Equal(t, SeedCapabilityExamplePromptCount, len(SeedCapabilityExamplePromptSlugs),
		"SeedCapabilityExamplePromptCount must equal len(SeedCapabilityExamplePromptSlugs)")
}

func TestSeedCapabilityExamplePromptSlugs_ContainsAllPerAgentSlugs(t *testing.T) {
	allSet := map[string]struct{}{}
	for _, s := range SeedCapabilityExamplePromptSlugs {
		allSet[s] = struct{}{}
	}
	for _, s := range SeedResearcherExamplePromptSlugs {
		_, ok := allSet[s]
		assert.True(t, ok, "researcher slug %q must appear in SeedCapabilityExamplePromptSlugs", s)
	}
	for _, s := range SeedAnalystExamplePromptSlugs {
		_, ok := allSet[s]
		assert.True(t, ok, "analyst slug %q must appear in SeedCapabilityExamplePromptSlugs", s)
	}
	for _, s := range SeedPlannerExamplePromptSlugs {
		_, ok := allSet[s]
		assert.True(t, ok, "planner slug %q must appear in SeedCapabilityExamplePromptSlugs", s)
	}
}
