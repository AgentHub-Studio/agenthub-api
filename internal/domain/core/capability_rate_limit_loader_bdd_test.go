package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000105 capability rate limit seeds.
// These assert seed shape and cost-protection rationale without a database.

func TestBDD_CapabilityRateLimitSeed(t *testing.T) {
	t.Run("Scenario_NineCapabilityRateLimitsAcrossThreeAgents", func(t *testing.T) {
		// Given the capability layer ships with three specialist agents
		//   (researcher, analyst, planner) each requiring three guardrails to
		//   prevent runaway LLM costs for new tenants,
		// When migration 000105 seeds rate limit rows,
		// Then exactly 9 rows are added — three limit types per agent — and
		//   the per-agent counts sum to the total count constant.
		assert.Equal(t, 9, SeedCapabilityRateLimitCount,
			"migration 000105 must seed exactly 9 rate limit rows (3 agents × 3 limits)")
		assert.Len(t, SeedCapabilityRateLimitAgentSlugs, 3,
			"SeedCapabilityRateLimitAgentSlugs must list exactly 3 agent slugs")
		sum := SeedResearcherRateLimitCount + SeedAnalystRateLimitCount + SeedPlannerRateLimitCount
		assert.Equal(t, SeedCapabilityRateLimitCount, sum,
			"per-agent rate limit counts must sum to SeedCapabilityRateLimitCount")
	})

	t.Run("Scenario_EachAgentHasThreeDistinctRateLimits", func(t *testing.T) {
		// Given each capability agent must be independently governable via three
		//   orthogonal levers (call frequency, token volume, concurrency),
		// When migration 000105 seeds limit keys,
		// Then the three limit key constants are non-empty, mutually distinct,
		//   and each per-agent count is exactly 3.
		keys := []string{
			SeedRateLimitKeyRequestsPerMinute,
			SeedRateLimitKeyTokensPerMinute,
			SeedRateLimitKeyMaxConcurrentRuns,
		}
		unique := map[string]struct{}{}
		for _, k := range keys {
			assert.NotEmpty(t, k, "limit key must not be empty")
			unique[k] = struct{}{}
		}
		assert.Len(t, unique, 3,
			"the three limit key constants must be mutually distinct")
		assert.Equal(t, 3, SeedResearcherRateLimitCount,
			"researcher must have exactly 3 rate limit rows")
		assert.Equal(t, 3, SeedAnalystRateLimitCount,
			"analyst must have exactly 3 rate limit rows")
		assert.Equal(t, 3, SeedPlannerRateLimitCount,
			"planner must have exactly 3 rate limit rows")
	})

	t.Run("Scenario_AnalystHasHighestTokenBudgetForDocumentProcessing", func(t *testing.T) {
		// Given the analyst capability agent processes large document corpora
		//   (PDFs, DOCX, structured reports) requiring the most tokens per pass,
		// When migration 000105 seeds tokens_per_minute limits,
		// Then the analyst token limit (80 000) exceeds both the researcher
		//   (50 000) and the planner (20 000), confirming it holds the highest
		//   token budget among the three agents.
		assert.Equal(t, 80000, SeedAnalystTokensPerMinute,
			"analyst tokens_per_minute must be 80 000 — highest token budget for large document processing")
		assert.Greater(t, SeedAnalystTokensPerMinute, SeedResearcherTokensPerMinute,
			"analyst token budget must exceed researcher's — document corpora dwarf search snippets")
		assert.Greater(t, SeedAnalystTokensPerMinute, SeedPlannerTokensPerMinute,
			"analyst token budget must exceed planner's — planning uses compact context windows")
	})

	t.Run("Scenario_PlannerHasHighestConcurrencyForLightweightTasks", func(t *testing.T) {
		// Given the planner capability agent handles structured, stateless task
		//   decomposition that imposes minimal resource pressure per run,
		// When migration 000105 seeds max_concurrent_runs limits,
		// Then the planner concurrency (3) exceeds both the researcher (2)
		//   and the analyst (2), and the concurrency window is 0 because
		//   concurrent-run limits are not time-windowed.
		assert.Equal(t, 3, SeedPlannerMaxConcurrentRuns,
			"planner max_concurrent_runs must be 3 — lightweight tasks allow highest concurrency")
		assert.Greater(t, SeedPlannerMaxConcurrentRuns, SeedResearcherMaxConcurrentRuns,
			"planner concurrency must exceed researcher's — planning is cheaper than web research")
		assert.Greater(t, SeedPlannerMaxConcurrentRuns, SeedAnalystMaxConcurrentRuns,
			"planner concurrency must exceed analyst's — planning is cheaper than document analysis")
		assert.Equal(t, 0, SeedConcurrentRunsWindowSeconds,
			"max_concurrent_runs must use window_seconds=0 — concurrency is not time-windowed")
	})

	t.Run("Scenario_ResearcherHasHighestRequestRateForActiveWebResearch", func(t *testing.T) {
		// Given the researcher capability agent drives active web-research loops
		//   that chain multiple LLM calls to gather and synthesise information,
		// When migration 000105 seeds requests_per_minute limits,
		// Then the researcher request rate (10) exceeds the analyst (8) and the
		//   planner (5), and all timed limits use a 60 s window.
		assert.Equal(t, 10, SeedResearcherRequestsPerMinute,
			"researcher requests_per_minute must be 10 — highest call rate for active research loops")
		assert.Greater(t, SeedResearcherRequestsPerMinute, SeedAnalystRequestsPerMinute,
			"researcher call rate must exceed analyst's — research is iterative, analysis is deliberate")
		assert.Greater(t, SeedAnalystRequestsPerMinute, SeedPlannerRequestsPerMinute,
			"analyst call rate must exceed planner's — planning makes the fewest LLM calls")
		assert.Equal(t, 60, SeedTimedLimitWindowSeconds,
			"timed rate limits must use a 60 s window (requests_per_minute and tokens_per_minute)")
	})
}
