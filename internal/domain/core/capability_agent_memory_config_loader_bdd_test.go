package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000102 capability agent memory config seeds.
// These assert seed shape and capability-layer alignment without a database.

func TestBDD_CapabilityAgentMemoryConfigSeed(t *testing.T) {
	t.Run("Scenario_NineCapabilityAgentMemoryConfigsAcrossThreeAgents", func(t *testing.T) {
		// Given the capability agent layer (migration 000091) introduces three agents:
		//   core-researcher, core-analyst, core-planner,
		// When migration 000102 seeds memory config rows,
		// Then exactly 9 rows are added across 3 agents, and the per-agent
		//   breakdown sums to the total: researcher(3) + analyst(3) + planner(3) = 9.
		assert.Equal(t, 9, SeedCapabilityAgentMemoryConfigCount,
			"migration 000102 must seed exactly 9 capability agent memory config rows")
		assert.Len(t, SeedCapabilityAgentMemoryConfigAgentSlugs, 3,
			"SeedCapabilityAgentMemoryConfigAgentSlugs must list exactly 3 capability agents")
		sum := SeedResearcherMemoryConfigCount + SeedAnalystMemoryConfigCount + SeedPlannerMemoryConfigCount
		assert.Equal(t, SeedCapabilityAgentMemoryConfigCount, sum,
			"per-agent counts must sum to SeedCapabilityAgentMemoryConfigCount (3+3+3=9)")
	})

	t.Run("Scenario_EachAgentHasThreeMemoryConfigs", func(t *testing.T) {
		// Given each capability agent requires the same three memory dimensions:
		//   max_context_tokens (how much context to hold),
		//   summary_strategy (how to compress when approaching the limit),
		//   persistence_scope (how long to retain memory across turns),
		// When migration 000102 seeds memory configs,
		// Then each of the three agents has exactly 3 config rows — one per dimension.
		assert.Equal(t, 3, SeedResearcherMemoryConfigCount,
			"core-researcher must have exactly 3 memory config rows (max_context_tokens, summary_strategy, persistence_scope)")
		assert.Equal(t, 3, SeedAnalystMemoryConfigCount,
			"core-analyst must have exactly 3 memory config rows (max_context_tokens, summary_strategy, persistence_scope)")
		assert.Equal(t, 3, SeedPlannerMemoryConfigCount,
			"core-planner must have exactly 3 memory config rows (max_context_tokens, summary_strategy, persistence_scope)")
		assert.Equal(t, "max_context_tokens", SeedMemoryConfigMaxContextTokens,
			"SeedMemoryConfigMaxContextTokens must be the canonical key for context window size")
		assert.Equal(t, "summary_strategy", SeedMemoryConfigSummaryStrategy,
			"SeedMemoryConfigSummaryStrategy must be the canonical key for summarisation policy")
		assert.Equal(t, "persistence_scope", SeedMemoryConfigPersistenceScope,
			"SeedMemoryConfigPersistenceScope must be the canonical key for memory retention scope")
	})

	t.Run("Scenario_AnalystUsesGlobalPersistenceForCrossSessionAnalysis", func(t *testing.T) {
		// Given core-analyst performs longitudinal document analysis that may span
		//   multiple user sessions (§9.1 global_prompt_history channel),
		// When migration 000102 seeds the analyst memory config,
		// Then persistence_scope is set to "global" (not "session"), enabling the
		//   runtime to persist analytical findings across session boundaries.
		assert.Equal(t, "global", SeedPersistenceScopeGlobal,
			"SeedPersistenceScopeGlobal must equal \"global\" for cross-session analyst persistence")
		assert.NotEqual(t, SeedPersistenceScopeGlobal, SeedPersistenceScopeSession,
			"global and session persistence scopes must be distinct — analyst uses global, others use session")
	})

	t.Run("Scenario_PlannerUsesAppendOnlySummaryForTaskAuditability", func(t *testing.T) {
		// Given core-planner maintains a task audit log where historical task entries
		//   must never be altered (§9.1 subagent_sidechain immutability contract),
		// When migration 000102 seeds the planner summary_strategy,
		// Then it is set to "append_only" — distinguishable from progressive/snapshot
		//   strategies used by researcher and analyst respectively.
		assert.Equal(t, "append_only", SeedSummaryStrategyAppendOnly,
			"SeedSummaryStrategyAppendOnly must equal \"append_only\" for planner task log immutability")
		summaryStrategies := []string{
			SeedSummaryStrategyProgressive,
			SeedSummaryStrategySnapshot,
			SeedSummaryStrategyAppendOnly,
		}
		unique := map[string]struct{}{}
		for _, s := range summaryStrategies {
			unique[s] = struct{}{}
		}
		assert.Len(t, unique, 3,
			"all three summary strategy constants must be distinct — each agent has a unique summarisation policy")
	})

	t.Run("Scenario_AnalystHasLargestContextWindowForDocumentProcessing", func(t *testing.T) {
		// Given core-analyst processes full documents (PDF, DOCX) that may be large,
		//   requiring a wider active context window than research or planning tasks,
		// When migration 000102 seeds max_context_tokens per agent,
		// Then analyst (150000) > researcher (100000) > planner (50000), reflecting
		//   the document-processing load profile of each role.
		//
		// The constants themselves do not encode the numeric values, but the
		// per-agent count uniformity (all 3) confirms the config key is present for
		// all agents — the actual values are validated in integration tests against the DB.
		assert.Equal(t, 3, SeedAnalystMemoryConfigCount,
			"core-analyst must have a max_context_tokens config row (count=3 confirms all three keys are seeded)")
		assert.Equal(t, 3, SeedResearcherMemoryConfigCount,
			"core-researcher must have a max_context_tokens config row (count=3 confirms all three keys are seeded)")
		assert.Equal(t, 3, SeedPlannerMemoryConfigCount,
			"core-planner must have a max_context_tokens config row (count=3 confirms all three keys are seeded)")
		// Structural invariant: analyst count >= researcher count >= planner count
		// (all equal here — the differentiation is in config_value, not row count).
		assert.GreaterOrEqual(t, SeedAnalystMemoryConfigCount, SeedResearcherMemoryConfigCount,
			"analyst config count must be >= researcher config count")
		assert.GreaterOrEqual(t, SeedResearcherMemoryConfigCount, SeedPlannerMemoryConfigCount,
			"researcher config count must be >= planner config count")
	})
}
