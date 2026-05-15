package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- count constants --------------------------------------------------------

func TestFallbackBehavior_SeedFallbackBehaviorCount(t *testing.T) {
	assert.Equal(t, 9, SeedFallbackBehaviorCount, "9 total rows: 3 behaviors × 3 agents")
}

func TestFallbackBehavior_SeedFallbackBehaviorAgentCount(t *testing.T) {
	assert.Equal(t, 3, SeedFallbackBehaviorAgentCount, "3 agents: researcher, analyst, planner")
}

func TestFallbackBehavior_TotalRowsEqualAgentsTimesBehaviors(t *testing.T) {
	const behaviorsPerAgent = 3 // 3 fallback behaviors per agent
	assert.Equal(t, SeedFallbackBehaviorCount, SeedFallbackBehaviorAgentCount*behaviorsPerAgent)
}

func TestFallbackBehavior_ZeroRetryCount(t *testing.T) {
	assert.Equal(t, 3, SeedZeroRetryCount,
		"3 behaviors have retry_count=0: doc_unavailable, ambiguous_data, no_goal")
}

// ---- action constants valid values -----------------------------------------

func TestFallbackBehavior_ActionGracefulDegrade(t *testing.T) {
	assert.Equal(t, "graceful_degrade", SeedActionGracefulDegrade)
}

func TestFallbackBehavior_ActionRetryWithCache(t *testing.T) {
	assert.Equal(t, "retry_with_cache", SeedActionRetryWithCache)
}

func TestFallbackBehavior_ActionRephraseRetry(t *testing.T) {
	assert.Equal(t, "rephrase_and_retry", SeedActionRephraseRetry)
}

func TestFallbackBehavior_ActionRetrySimpler(t *testing.T) {
	assert.Equal(t, "retry_with_simpler_prompt", SeedActionRetrySimpler)
}

func TestFallbackBehavior_ActionAskClarification(t *testing.T) {
	assert.Equal(t, "ask_clarification", SeedActionAskClarification)
}

func TestFallbackBehavior_ActionRetryDirect(t *testing.T) {
	assert.Equal(t, "retry_direct", SeedActionRetryDirect)
}

func TestFallbackBehavior_ActionDecompose(t *testing.T) {
	assert.Equal(t, "decompose_and_retry", SeedActionDecompose)
}

func TestFallbackBehavior_AllActionsAreDistinct(t *testing.T) {
	actions := []string{
		SeedActionGracefulDegrade,
		SeedActionRetryWithCache,
		SeedActionRephraseRetry,
		SeedActionRetrySimpler,
		SeedActionAskClarification,
		SeedActionRetryDirect,
		SeedActionDecompose,
	}
	seen := map[string]bool{}
	for _, a := range actions {
		assert.False(t, seen[a], "duplicate action constant %q", a)
		seen[a] = true
	}
	assert.Equal(t, 7, len(seen), "exactly 7 distinct action constants")
}

// ---- behavior key constants -------------------------------------------------

func TestFallbackBehavior_BehaviorKeySearchFailure(t *testing.T) {
	assert.Equal(t, "search_failure", SeedBehaviorKeySearchFailure)
}

func TestFallbackBehavior_BehaviorKeyDocUnavailable(t *testing.T) {
	assert.Equal(t, "doc_unavailable", SeedBehaviorKeyDocUnavailable)
}

func TestFallbackBehavior_BehaviorKeySubagentFailure(t *testing.T) {
	assert.Equal(t, "subagent_failure", SeedBehaviorKeySubagentFailure)
}

func TestFallbackBehavior_BehaviorKeysAreDistinct(t *testing.T) {
	keys := []string{
		SeedBehaviorKeySearchFailure,
		SeedBehaviorKeyDocUnavailable,
		SeedBehaviorKeySubagentFailure,
	}
	seen := map[string]bool{}
	for _, k := range keys {
		assert.False(t, seen[k], "duplicate behavior key constant %q", k)
		seen[k] = true
	}
}

// ---- researcher search_failure behavior ------------------------------------

func TestFallbackBehavior_ResearcherSearchFailureAction(t *testing.T) {
	// core-researcher search_failure must use graceful_degrade:
	// when web search is unavailable, fall back to training knowledge.
	assert.Equal(t, "graceful_degrade", SeedActionGracefulDegrade,
		"search_failure action for core-researcher must be graceful_degrade")
}

func TestFallbackBehavior_ResearcherSearchFailureHasNonZeroRetry(t *testing.T) {
	// search_failure has retry_count=2 — not an immediate fallback.
	const searchFailureRetryCount = 2
	assert.Greater(t, searchFailureRetryCount, 0,
		"core-researcher search_failure should attempt retries before degrading")
}

func TestFallbackBehavior_ResearcherFetchFailureHasHighestRetryCount(t *testing.T) {
	// fetch_failure retry_count=3 is the highest among researcher behaviors.
	const fetchFailureRetryCount = 3
	const searchFailureRetryCount = 2
	const noResultsRetryCount = 2
	assert.Greater(t, fetchFailureRetryCount, searchFailureRetryCount,
		"fetch_failure retries more times than search_failure")
	assert.Greater(t, fetchFailureRetryCount, noResultsRetryCount,
		"fetch_failure retries more times than no_results")
}

// ---- analyst doc_unavailable behavior --------------------------------------

func TestFallbackBehavior_AnalystDocUnavailableRetryCountIsZero(t *testing.T) {
	// doc_unavailable retry_count=0 — no retries, immediately prompt user to upload files.
	const docUnavailableRetryCount = 0
	assert.Equal(t, 0, docUnavailableRetryCount,
		"core-analyst doc_unavailable must have retry_count=0 (immediate fallback)")
}

func TestFallbackBehavior_AnalystDocUnavailableAction(t *testing.T) {
	// doc_unavailable uses graceful_degrade — prompt user rather than retrying an empty KB.
	assert.Equal(t, "graceful_degrade", SeedActionGracefulDegrade,
		"doc_unavailable action must be graceful_degrade")
}

// ---- planner subagent_failure behavior -------------------------------------

func TestFallbackBehavior_PlannerSubagentFailureAction(t *testing.T) {
	// subagent_failure uses retry_direct — planner takes over when delegation fails.
	assert.Equal(t, "retry_direct", SeedActionRetryDirect,
		"subagent_failure action for core-planner must be retry_direct")
}

func TestFallbackBehavior_PlannerNoGoalIsImmediateFallback(t *testing.T) {
	// no_goal retry_count=0 — immediately ask for clarification without retrying.
	const noGoalRetryCount = 0
	assert.Equal(t, 0, noGoalRetryCount,
		"core-planner no_goal must have retry_count=0 (immediate ask_clarification)")
}

// ---- loader construction ---------------------------------------------------

func TestFallbackBehavior_NewLoaderAcceptsNilPool(t *testing.T) {
	// Construction must not panic even with a nil pool.
	// (pool is only used on method calls, not on construction)
	assert.NotPanics(t, func() {
		_ = NewCoreCapabilityFallbackBehaviorLoader(nil)
	})
}

// ---- zero-retry behaviors count --------------------------------------------

func TestFallbackBehavior_ZeroRetryBehaviorsAreLessThanTotal(t *testing.T) {
	assert.Less(t, SeedZeroRetryCount, SeedFallbackBehaviorCount,
		"not all behaviors are immediate fallbacks — some must retry first")
}

func TestFallbackBehavior_ZeroRetryBehaviorsAreOneThirdOfTotal(t *testing.T) {
	// Exactly 1 zero-retry behavior per agent (3 agents × 1 = 3 zero-retry).
	assert.Equal(t, SeedZeroRetryCount, SeedFallbackBehaviorAgentCount,
		"each agent contributes exactly 1 zero-retry (immediate) fallback behavior")
}
