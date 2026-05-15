package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000101 capability agent tool config seeds.
// These assert seed shape and capability-layer alignment without a database.

func TestBDD_CapabilityAgentToolConfigSeed(t *testing.T) {
	t.Run("Scenario_SevenCapabilityAgentToolConfigsAcrossThreeAgents", func(t *testing.T) {
		// Given the capability agent layer (migration 000091) introduces three agents:
		//   core-researcher, core-analyst, core-planner,
		// When migration 000101 seeds tool config overrides,
		// Then exactly 7 rows are added across 3 agents, and the per-agent
		//   breakdown sums to the total: researcher(3) + analyst(2) + planner(2) = 7.
		assert.Equal(t, 7, SeedCapabilityAgentToolConfigCount,
			"migration 000101 must seed exactly 7 capability agent tool config overrides")
		assert.Len(t, SeedCapabilityAgentToolConfigAgentSlugs, 3,
			"SeedCapabilityAgentToolConfigAgentSlugs must list exactly 3 capability agents")
		sum := SeedResearcherToolConfigCount + SeedAnalystToolConfigCount + SeedPlannerToolConfigCount
		assert.Equal(t, SeedCapabilityAgentToolConfigCount, sum,
			"per-agent counts must sum to SeedCapabilityAgentToolConfigCount")
	})

	t.Run("Scenario_ResearcherHasThreeToolConfigs", func(t *testing.T) {
		// Given core-researcher is a web research agent requiring broad retrieval,
		// When migration 000101 seeds its tool config overrides,
		// Then exactly 3 parameter overrides are set:
		//   max_results for core-web-search,
		//   timeout_seconds for core-web-fetch,
		//   similarity_threshold for core-doc-search.
		assert.Equal(t, 3, SeedResearcherToolConfigCount,
			"core-researcher must have exactly 3 tool config overrides (max_results, timeout_seconds, similarity_threshold)")
		assert.Equal(t, "max_results", SeedToolConfigMaxResults,
			"SeedToolConfigMaxResults must be the canonical key for result count overrides")
		assert.Equal(t, "timeout_seconds", SeedToolConfigTimeoutSeconds,
			"SeedToolConfigTimeoutSeconds must be the canonical key for timeout overrides")
		assert.Equal(t, "similarity_threshold", SeedToolConfigSimilarityThreshold,
			"SeedToolConfigSimilarityThreshold must be the canonical key for vector similarity overrides")
	})

	t.Run("Scenario_AnalystHasTwoToolConfigs", func(t *testing.T) {
		// Given core-analyst is a document analysis agent requiring deep reads,
		// When migration 000101 seeds its tool config overrides,
		// Then exactly 2 parameter overrides are set:
		//   max_results for core-doc-search,
		//   max_tokens for core-doc-read.
		assert.Equal(t, 2, SeedAnalystToolConfigCount,
			"core-analyst must have exactly 2 tool config overrides (max_results, max_tokens)")
		assert.Equal(t, "max_results", SeedToolConfigMaxResults,
			"SeedToolConfigMaxResults must be the canonical key for result count overrides")
		assert.Equal(t, "max_tokens", SeedToolConfigMaxTokens,
			"SeedToolConfigMaxTokens must be the canonical key for token budget overrides")
	})

	t.Run("Scenario_PlannerHasTwoToolConfigs", func(t *testing.T) {
		// Given core-planner is a task management agent requiring structured item handling,
		// When migration 000101 seeds its tool config overrides,
		// Then exactly 2 parameter overrides are set:
		//   max_items for core-todo-create,
		//   include_completed for core-todo-list.
		assert.Equal(t, 2, SeedPlannerToolConfigCount,
			"core-planner must have exactly 2 tool config overrides (max_items, include_completed)")
		assert.Equal(t, "max_items", SeedToolConfigMaxItems,
			"SeedToolConfigMaxItems must be the canonical key for item limit overrides")
		assert.Equal(t, "include_completed", SeedToolConfigIncludeCompleted,
			"SeedToolConfigIncludeCompleted must be the canonical key for completed-items flag overrides")
	})

	t.Run("Scenario_ToolConfigParamKeysAreWellDefined", func(t *testing.T) {
		// Given tool config param keys must be machine-readable identifiers for the
		//   runtime layer to recognize without ambiguity,
		// When migration 000101 seeds the 6 distinct parameter keys,
		// Then all constants are non-empty strings and the set of unique keys
		//   across all seeded agents totals 6 (max_results, timeout_seconds,
		//   similarity_threshold, max_tokens, max_items, include_completed).
		paramKeys := []string{
			SeedToolConfigMaxResults,
			SeedToolConfigTimeoutSeconds,
			SeedToolConfigSimilarityThreshold,
			SeedToolConfigMaxTokens,
			SeedToolConfigMaxItems,
			SeedToolConfigIncludeCompleted,
		}
		for _, key := range paramKeys {
			assert.NotEmpty(t, key,
				"tool config param key constant must be a non-empty string")
		}
		uniqueKeys := map[string]struct{}{}
		for _, key := range paramKeys {
			uniqueKeys[key] = struct{}{}
		}
		assert.Len(t, uniqueKeys, 6,
			"migration 000101 must define exactly 6 distinct tool config param key constants")
	})
}
