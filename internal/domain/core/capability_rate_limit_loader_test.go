package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability rate limit seed constants (migration 000105).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityRateLimitCount_IsNine(t *testing.T) {
	assert.Equal(t, 9, SeedCapabilityRateLimitCount,
		"migration 000105 seeds exactly 9 capability rate limit rows (3 agents × 3 limits)")
}

func TestSeedCapabilityRateLimitAgentSlugs_HasLengthThree(t *testing.T) {
	assert.Len(t, SeedCapabilityRateLimitAgentSlugs, 3,
		"SeedCapabilityRateLimitAgentSlugs must have exactly 3 entries — one per capability agent")
}

func TestSeedCapabilityRateLimitAgentSlugs_AllStartWithCore(t *testing.T) {
	for _, slug := range SeedCapabilityRateLimitAgentSlugs {
		assert.True(t, strings.HasPrefix(slug, "core-"),
			"agent slug %q must start with 'core-' (capability agent namespace contract)", slug)
	}
}

func TestSeedCapabilityRateLimitAgentSlugs_AllAreDistinct(t *testing.T) {
	unique := map[string]struct{}{}
	for _, slug := range SeedCapabilityRateLimitAgentSlugs {
		unique[slug] = struct{}{}
	}
	assert.Len(t, unique, 3,
		"all entries in SeedCapabilityRateLimitAgentSlugs must be distinct (no duplicates)")
}

func TestSeedPerAgentRateLimitCounts_AreEachThree(t *testing.T) {
	assert.Equal(t, 3, SeedResearcherRateLimitCount,
		"SeedResearcherRateLimitCount must be 3 (req-per-min, tokens-per-min, max-concurrent)")
	assert.Equal(t, 3, SeedAnalystRateLimitCount,
		"SeedAnalystRateLimitCount must be 3 (req-per-min, tokens-per-min, max-concurrent)")
	assert.Equal(t, 3, SeedPlannerRateLimitCount,
		"SeedPlannerRateLimitCount must be 3 (req-per-min, tokens-per-min, max-concurrent)")
}

func TestSeedPerAgentRateLimitCounts_SumToTotal(t *testing.T) {
	sum := SeedResearcherRateLimitCount + SeedAnalystRateLimitCount + SeedPlannerRateLimitCount
	assert.Equal(t, SeedCapabilityRateLimitCount, sum,
		"SeedCapabilityRateLimitCount must equal the sum of per-agent counts (%d+%d+%d=%d)",
		SeedResearcherRateLimitCount, SeedAnalystRateLimitCount, SeedPlannerRateLimitCount, sum)
}

func TestSeedRateLimitKeys_AreAllDistinctNonEmpty(t *testing.T) {
	keys := []string{
		SeedRateLimitKeyRequestsPerMinute,
		SeedRateLimitKeyTokensPerMinute,
		SeedRateLimitKeyMaxConcurrentRuns,
	}
	unique := map[string]struct{}{}
	for _, k := range keys {
		assert.NotEmpty(t, k, "limit key constant must not be empty")
		unique[k] = struct{}{}
	}
	assert.Len(t, unique, 3,
		"all three limit key constants must be distinct non-empty strings")
}

func TestSeedRateLimitKeyRequestsPerMinute_Value(t *testing.T) {
	assert.Equal(t, "requests_per_minute", SeedRateLimitKeyRequestsPerMinute,
		"SeedRateLimitKeyRequestsPerMinute must equal \"requests_per_minute\"")
}

func TestSeedRateLimitKeyTokensPerMinute_Value(t *testing.T) {
	assert.Equal(t, "tokens_per_minute", SeedRateLimitKeyTokensPerMinute,
		"SeedRateLimitKeyTokensPerMinute must equal \"tokens_per_minute\"")
}

func TestSeedRateLimitKeyMaxConcurrentRuns_Value(t *testing.T) {
	assert.Equal(t, "max_concurrent_runs", SeedRateLimitKeyMaxConcurrentRuns,
		"SeedRateLimitKeyMaxConcurrentRuns must equal \"max_concurrent_runs\"")
}

func TestSeedAnalystTokensPerMinute_IsHighest(t *testing.T) {
	assert.Greater(t, SeedAnalystTokensPerMinute, SeedResearcherTokensPerMinute,
		"analyst tokens_per_minute (%d) must exceed researcher (%d) — processes large documents",
		SeedAnalystTokensPerMinute, SeedResearcherTokensPerMinute)
	assert.Greater(t, SeedAnalystTokensPerMinute, SeedPlannerTokensPerMinute,
		"analyst tokens_per_minute (%d) must exceed planner (%d) — processes large documents",
		SeedAnalystTokensPerMinute, SeedPlannerTokensPerMinute)
}

func TestSeedPlannerMaxConcurrentRuns_IsHighest(t *testing.T) {
	assert.Greater(t, SeedPlannerMaxConcurrentRuns, SeedResearcherMaxConcurrentRuns,
		"planner max_concurrent_runs (%d) must exceed researcher (%d) — lightweight tasks allow more concurrency",
		SeedPlannerMaxConcurrentRuns, SeedResearcherMaxConcurrentRuns)
	assert.Greater(t, SeedPlannerMaxConcurrentRuns, SeedAnalystMaxConcurrentRuns,
		"planner max_concurrent_runs (%d) must exceed analyst (%d) — lightweight tasks allow more concurrency",
		SeedPlannerMaxConcurrentRuns, SeedAnalystMaxConcurrentRuns)
}

func TestSeedResearcherRequestsPerMinute_IsHighest(t *testing.T) {
	assert.Greater(t, SeedResearcherRequestsPerMinute, SeedAnalystRequestsPerMinute,
		"researcher requests_per_minute (%d) must exceed analyst (%d) — web research loops need higher call rates",
		SeedResearcherRequestsPerMinute, SeedAnalystRequestsPerMinute)
	assert.Greater(t, SeedAnalystRequestsPerMinute, SeedPlannerRequestsPerMinute,
		"analyst requests_per_minute (%d) must exceed planner (%d) — analyst calls more than planner",
		SeedAnalystRequestsPerMinute, SeedPlannerRequestsPerMinute)
}

func TestSeedConcurrentRunsWindowSeconds_IsZero(t *testing.T) {
	assert.Equal(t, 0, SeedConcurrentRunsWindowSeconds,
		"SeedConcurrentRunsWindowSeconds must be 0 — max_concurrent_runs is not time-windowed")
}

func TestSeedTimedLimitWindowSeconds_IsSixty(t *testing.T) {
	assert.Equal(t, 60, SeedTimedLimitWindowSeconds,
		"SeedTimedLimitWindowSeconds must be 60 — requests and tokens are measured per 60 s window")
}

func TestSeedCapabilityRateLimitCount_EqualsThreeAgentsTimesThreeLimits(t *testing.T) {
	agentCount := len(SeedCapabilityRateLimitAgentSlugs)
	limitsPerAgent := SeedResearcherRateLimitCount // all agents have the same limit count
	assert.Equal(t, SeedCapabilityRateLimitCount, agentCount*limitsPerAgent,
		"SeedCapabilityRateLimitCount must equal %d agents × %d limits = %d",
		agentCount, limitsPerAgent, agentCount*limitsPerAgent)
}
