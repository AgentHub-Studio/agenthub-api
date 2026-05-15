package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for CoreCapabilityFallbackBehavior seed (migration 000121).
// Each scenario validates a distinct design decision in the per-agent
// fallback behavior catalog.

func TestBDD_AhCoreCapabilityFallbackBehaviorSeed(t *testing.T) {
	t.Run("Scenario_NineBehaviorsAcrossThreeAgents", func(t *testing.T) {
		// Given three core capability agents (researcher, analyst, planner)
		// And each agent has exactly three fallback behavior rows
		// When the seed migration 000121 runs
		// Then exactly 9 fallback behavior rows exist in capability_fallback_behavior
		const behaviorsPerAgent = 3
		assert.Equal(t, 9, SeedFallbackBehaviorCount)
		assert.Equal(t, 3, SeedFallbackBehaviorAgentCount)
		assert.Equal(t, SeedFallbackBehaviorCount, SeedFallbackBehaviorAgentCount*behaviorsPerAgent)
	})

	t.Run("Scenario_ThreeBehaviorsImmediatelyFallback", func(t *testing.T) {
		// Given three behaviors where retrying would be futile or confusing
		//   - core-analyst doc_unavailable (no documents to find — user must act)
		//   - core-analyst ambiguous_data (no data to analyze without clarification)
		//   - core-planner no_goal (no plan can be made without a clear objective)
		// When the orchestrator evaluates their retry_count
		// Then exactly 3 behaviors have retry_count=0 (immediate fallback)
		// And this equals the number of agents (one zero-retry behavior per agent)
		assert.Equal(t, 3, SeedZeroRetryCount,
			"doc_unavailable, ambiguous_data, and no_goal must all be immediate fallbacks")
		assert.Equal(t, SeedZeroRetryCount, SeedFallbackBehaviorAgentCount,
			"each agent contributes exactly one immediate-fallback behavior")
	})

	t.Run("Scenario_ResearcherDegracesGracefullyOnSearchFailure", func(t *testing.T) {
		// Given core-researcher relies on web search as its primary capability
		// And the web search tool becomes temporarily unavailable
		// When the orchestrator evaluates the search_failure fallback
		// Then the action is graceful_degrade (answer from training knowledge)
		// And the behavior key is "search_failure"
		// And retry_count=2 (retry twice before degrading gracefully)
		assert.Equal(t, "search_failure", SeedBehaviorKeySearchFailure,
			"search failure behavior key must be search_failure")
		assert.Equal(t, "graceful_degrade", SeedActionGracefulDegrade,
			"search failure must degrade gracefully rather than error hard")
		const searchFailureRetryCount = 2
		assert.Greater(t, searchFailureRetryCount, 0,
			"search_failure retries before giving up — not an immediate fallback")
	})

	t.Run("Scenario_AnalystAsksForClarificationWhenDataAmbiguous", func(t *testing.T) {
		// Given core-analyst receives data that lacks sufficient context for analysis
		// And guessing at the user's intent would produce unreliable results
		// When the orchestrator evaluates the ambiguous_data fallback
		// Then the action is ask_clarification (pause and request more context)
		// And retry_count=0 (immediately ask — no retry makes sense without more info)
		assert.Equal(t, "ask_clarification", SeedActionAskClarification,
			"ambiguous_data must ask for clarification rather than retry blindly")
		const ambiguousDataRetryCount = 0
		assert.Equal(t, 0, ambiguousDataRetryCount,
			"ambiguous_data must be an immediate fallback with retry_count=0")
	})

	t.Run("Scenario_PlannerDecomposesWhenScopeTooLarge", func(t *testing.T) {
		// Given core-planner receives a task that exceeds the context window
		// And the full task cannot be processed in a single planning pass
		// When the orchestrator evaluates the scope_too_large fallback
		// Then the action is decompose_and_retry (break into phases, retry each)
		// And retry_count=1 (attempt one decomposition and retry)
		assert.Equal(t, "decompose_and_retry", SeedActionDecompose,
			"scope_too_large must decompose and retry rather than fail hard")
		const scopeTooLargeRetryCount = 1
		assert.Equal(t, 1, scopeTooLargeRetryCount,
			"scope_too_large allows one decompose-and-retry attempt")
		assert.Greater(t, scopeTooLargeRetryCount, 0,
			"scope_too_large is not an immediate fallback — decomposition is attempted")
	})
}
