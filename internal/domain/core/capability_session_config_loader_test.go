package core

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability session config seed constants (migration 000111).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilitySessionConfigCount_IsNine(t *testing.T) {
	assert.Equal(t, 9, SeedCapabilitySessionConfigCount,
		"migration 000111 seeds exactly 9 capability session config rows (three per capability agent)")
}

func TestSeedCapabilitySessionConfigAgentSlugs_HasThreeEntries(t *testing.T) {
	assert.Len(t, SeedCapabilitySessionConfigAgentSlugs, 3,
		"SeedCapabilitySessionConfigAgentSlugs must have exactly 3 entries (researcher, analyst, planner)")
}

func TestSeedResearcherSessionConfigCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedResearcherSessionConfigCount,
		"SeedResearcherSessionConfigCount must be 3 — max_turns + idle_timeout_seconds + welcome_message")
}

func TestSeedAnalystSessionConfigCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedAnalystSessionConfigCount,
		"SeedAnalystSessionConfigCount must be 3 — max_turns + idle_timeout_seconds + welcome_message")
}

func TestSeedPlannerSessionConfigCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedPlannerSessionConfigCount,
		"SeedPlannerSessionConfigCount must be 3 — max_turns + idle_timeout_seconds + welcome_message")
}

func TestSeedPerAgentSessionConfigCounts_SumToTotal(t *testing.T) {
	sum := SeedResearcherSessionConfigCount + SeedAnalystSessionConfigCount + SeedPlannerSessionConfigCount
	assert.Equal(t, SeedCapabilitySessionConfigCount, sum,
		"researcher (%d) + analyst (%d) + planner (%d) must equal total count (%d)",
		SeedResearcherSessionConfigCount, SeedAnalystSessionConfigCount, SeedPlannerSessionConfigCount,
		SeedCapabilitySessionConfigCount)
}

func TestSeedSessionConfigKeyConstants_AllNonEmpty(t *testing.T) {
	keys := []string{
		SeedSessionConfigMaxTurns,
		SeedSessionConfigIdleTimeout,
		SeedSessionConfigWelcomeMessage,
	}
	for _, k := range keys {
		assert.NotEmpty(t, k, "every session config key constant must be non-empty")
	}
}

func TestSeedSessionConfigMaxTurns_IsCorrectString(t *testing.T) {
	assert.Equal(t, "max_turns", SeedSessionConfigMaxTurns,
		"SeedSessionConfigMaxTurns must equal the string \"max_turns\"")
}

func TestSeedSessionConfigIdleTimeout_IsCorrectString(t *testing.T) {
	assert.Equal(t, "idle_timeout_seconds", SeedSessionConfigIdleTimeout,
		"SeedSessionConfigIdleTimeout must equal the string \"idle_timeout_seconds\"")
}

func TestSeedSessionConfigWelcomeMessage_IsCorrectString(t *testing.T) {
	assert.Equal(t, "welcome_message", SeedSessionConfigWelcomeMessage,
		"SeedSessionConfigWelcomeMessage must equal the string \"welcome_message\"")
}

func TestSeedPlannerMaxTurns_IsGreaterThanResearcherAndAnalyst(t *testing.T) {
	plannerTurns, err := strconv.Atoi(SeedPlannerMaxTurns)
	assert.NoError(t, err, "SeedPlannerMaxTurns must be a numeric string")

	researcherTurns, err := strconv.Atoi(SeedResearcherMaxTurns)
	assert.NoError(t, err, "SeedResearcherMaxTurns must be a numeric string")

	analystTurns, err := strconv.Atoi(SeedAnalystMaxTurns)
	assert.NoError(t, err, "SeedAnalystMaxTurns must be a numeric string")

	assert.Greater(t, plannerTurns, researcherTurns,
		"planner (%d) must have more max turns than researcher (%d)", plannerTurns, researcherTurns)
	assert.Greater(t, plannerTurns, analystTurns,
		"planner (%d) must have more max turns than analyst (%d)", plannerTurns, analystTurns)
}

func TestSeedPlannerIdleTimeout_IsLongestAmongThreeAgents(t *testing.T) {
	plannerTimeout, err := strconv.Atoi(SeedPlannerIdleTimeout)
	assert.NoError(t, err, "SeedPlannerIdleTimeout must be a numeric string")

	researcherTimeout, err := strconv.Atoi(SeedResearcherIdleTimeout)
	assert.NoError(t, err, "SeedResearcherIdleTimeout must be a numeric string")

	analystTimeout, err := strconv.Atoi(SeedAnalystIdleTimeout)
	assert.NoError(t, err, "SeedAnalystIdleTimeout must be a numeric string")

	assert.Greater(t, plannerTimeout, researcherTimeout,
		"planner (%d s) must have a longer idle timeout than researcher (%d s)", plannerTimeout, researcherTimeout)
	assert.Greater(t, plannerTimeout, analystTimeout,
		"planner (%d s) must have a longer idle timeout than analyst (%d s)", plannerTimeout, analystTimeout)
}

func TestSeedResearcherIdleTimeout_IsShortestAmongThreeAgents(t *testing.T) {
	researcherTimeout, err := strconv.Atoi(SeedResearcherIdleTimeout)
	assert.NoError(t, err, "SeedResearcherIdleTimeout must be a numeric string")

	analystTimeout, err := strconv.Atoi(SeedAnalystIdleTimeout)
	assert.NoError(t, err, "SeedAnalystIdleTimeout must be a numeric string")

	plannerTimeout, err := strconv.Atoi(SeedPlannerIdleTimeout)
	assert.NoError(t, err, "SeedPlannerIdleTimeout must be a numeric string")

	assert.Less(t, researcherTimeout, analystTimeout,
		"researcher (%d s) must have a shorter idle timeout than analyst (%d s)", researcherTimeout, analystTimeout)
	assert.Less(t, researcherTimeout, plannerTimeout,
		"researcher (%d s) must have a shorter idle timeout than planner (%d s)", researcherTimeout, plannerTimeout)
}

func TestSeedAllMaxTurnsValues_AreNumericStrings(t *testing.T) {
	values := []string{
		SeedResearcherMaxTurns,
		SeedAnalystMaxTurns,
		SeedPlannerMaxTurns,
	}
	for _, v := range values {
		n, err := strconv.Atoi(v)
		assert.NoError(t, err, "max_turns value %q must be parseable as int", v)
		assert.Greater(t, n, 0, "max_turns value %q must be a positive integer", v)
	}
}

func TestSeedCapabilitySessionConfigAgentSlugs_ContainsAllThreeAgents(t *testing.T) {
	slugSet := map[string]struct{}{}
	for _, s := range SeedCapabilitySessionConfigAgentSlugs {
		slugSet[s] = struct{}{}
	}
	_, hasResearcher := slugSet["core-researcher"]
	assert.True(t, hasResearcher, "SeedCapabilitySessionConfigAgentSlugs must contain 'core-researcher'")

	_, hasAnalyst := slugSet["core-analyst"]
	assert.True(t, hasAnalyst, "SeedCapabilitySessionConfigAgentSlugs must contain 'core-analyst'")

	_, hasPlanner := slugSet["core-planner"]
	assert.True(t, hasPlanner, "SeedCapabilitySessionConfigAgentSlugs must contain 'core-planner'")
}
