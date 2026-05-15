package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability agent tool config seed constants (migration 000101).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityAgentToolConfigCount_IsSeven(t *testing.T) {
	assert.Equal(t, 7, SeedCapabilityAgentToolConfigCount,
		"migration 000101 seeds exactly 7 capability agent tool config overrides across three capability agents")
}

func TestSeedCapabilityAgentToolConfigAgentSlugs_HasLengthThree(t *testing.T) {
	assert.Len(t, SeedCapabilityAgentToolConfigAgentSlugs, 3,
		"SeedCapabilityAgentToolConfigAgentSlugs must have exactly 3 entries — one per capability agent")
}

func TestSeedCapabilityAgentToolConfigAgentSlugs_ContainsResearcher(t *testing.T) {
	assert.Contains(t, SeedCapabilityAgentToolConfigAgentSlugs, "core-researcher",
		"SeedCapabilityAgentToolConfigAgentSlugs must contain core-researcher (migration 000101)")
}

func TestSeedCapabilityAgentToolConfigAgentSlugs_ContainsAnalyst(t *testing.T) {
	assert.Contains(t, SeedCapabilityAgentToolConfigAgentSlugs, "core-analyst",
		"SeedCapabilityAgentToolConfigAgentSlugs must contain core-analyst (migration 000101)")
}

func TestSeedCapabilityAgentToolConfigAgentSlugs_ContainsPlanner(t *testing.T) {
	assert.Contains(t, SeedCapabilityAgentToolConfigAgentSlugs, "core-planner",
		"SeedCapabilityAgentToolConfigAgentSlugs must contain core-planner (migration 000101)")
}

func TestSeedCapabilityAgentToolConfigAgentSlugs_AllStartWithCorePrefix(t *testing.T) {
	for _, slug := range SeedCapabilityAgentToolConfigAgentSlugs {
		assert.True(t, strings.HasPrefix(slug, "core-"),
			"capability agent slug %q must start with 'core-' (capability-layer namespace contract)",
			slug)
	}
}

func TestSeedResearcherToolConfigCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedResearcherToolConfigCount,
		"core-researcher must have exactly 3 tool config overrides in migration 000101")
}

func TestSeedAnalystToolConfigCount_IsTwo(t *testing.T) {
	assert.Equal(t, 2, SeedAnalystToolConfigCount,
		"core-analyst must have exactly 2 tool config overrides in migration 000101")
}

func TestSeedPlannerToolConfigCount_IsTwo(t *testing.T) {
	assert.Equal(t, 2, SeedPlannerToolConfigCount,
		"core-planner must have exactly 2 tool config overrides in migration 000101")
}

func TestSeedCapabilityAgentToolConfigCount_EqualsPerAgentSum(t *testing.T) {
	sum := SeedResearcherToolConfigCount + SeedAnalystToolConfigCount + SeedPlannerToolConfigCount
	assert.Equal(t, SeedCapabilityAgentToolConfigCount, sum,
		"SeedCapabilityAgentToolConfigCount must equal the sum of per-agent counts (3+2+2=7)")
}

func TestSeedToolConfigMaxResults_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedToolConfigMaxResults,
		"SeedToolConfigMaxResults must be a non-empty string")
}

func TestSeedToolConfigMaxResults_IsCorrectString(t *testing.T) {
	assert.Equal(t, "max_results", SeedToolConfigMaxResults,
		"SeedToolConfigMaxResults must equal \"max_results\"")
}

func TestSeedToolConfigTimeoutSeconds_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedToolConfigTimeoutSeconds,
		"SeedToolConfigTimeoutSeconds must be a non-empty string")
}

func TestSeedToolConfigSimilarityThreshold_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedToolConfigSimilarityThreshold,
		"SeedToolConfigSimilarityThreshold must be a non-empty string")
}

func TestSeedToolConfigMaxTokens_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedToolConfigMaxTokens,
		"SeedToolConfigMaxTokens must be a non-empty string")
}

func TestSeedToolConfigMaxItems_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedToolConfigMaxItems,
		"SeedToolConfigMaxItems must be a non-empty string")
}

func TestSeedToolConfigIncludeCompleted_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedToolConfigIncludeCompleted,
		"SeedToolConfigIncludeCompleted must be a non-empty string")
}
