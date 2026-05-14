package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability agent memory config seed constants (migration 000102).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityAgentMemoryConfigCount_IsNine(t *testing.T) {
	assert.Equal(t, 9, SeedCapabilityAgentMemoryConfigCount,
		"migration 000102 seeds exactly 9 capability agent memory config rows across three capability agents")
}

func TestSeedCapabilityAgentMemoryConfigAgentSlugs_HasLengthThree(t *testing.T) {
	assert.Len(t, SeedCapabilityAgentMemoryConfigAgentSlugs, 3,
		"SeedCapabilityAgentMemoryConfigAgentSlugs must have exactly 3 entries — one per capability agent")
}

func TestSeedCapabilityAgentMemoryConfigAgentSlugs_ContainsResearcher(t *testing.T) {
	assert.Contains(t, SeedCapabilityAgentMemoryConfigAgentSlugs, "core-researcher",
		"SeedCapabilityAgentMemoryConfigAgentSlugs must contain core-researcher (migration 000102)")
}

func TestSeedCapabilityAgentMemoryConfigAgentSlugs_ContainsAnalyst(t *testing.T) {
	assert.Contains(t, SeedCapabilityAgentMemoryConfigAgentSlugs, "core-analyst",
		"SeedCapabilityAgentMemoryConfigAgentSlugs must contain core-analyst (migration 000102)")
}

func TestSeedCapabilityAgentMemoryConfigAgentSlugs_ContainsPlanner(t *testing.T) {
	assert.Contains(t, SeedCapabilityAgentMemoryConfigAgentSlugs, "core-planner",
		"SeedCapabilityAgentMemoryConfigAgentSlugs must contain core-planner (migration 000102)")
}

func TestSeedCapabilityAgentMemoryConfigAgentSlugs_AllStartWithCorePrefix(t *testing.T) {
	for _, slug := range SeedCapabilityAgentMemoryConfigAgentSlugs {
		assert.True(t, strings.HasPrefix(slug, "core-"),
			"capability agent slug %q must start with 'core-' (capability-layer namespace contract)",
			slug)
	}
}

func TestSeedResearcherMemoryConfigCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedResearcherMemoryConfigCount,
		"core-researcher must have exactly 3 memory config rows in migration 000102")
}

func TestSeedAnalystMemoryConfigCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedAnalystMemoryConfigCount,
		"core-analyst must have exactly 3 memory config rows in migration 000102")
}

func TestSeedPlannerMemoryConfigCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedPlannerMemoryConfigCount,
		"core-planner must have exactly 3 memory config rows in migration 000102")
}

func TestSeedCapabilityAgentMemoryConfigCount_EqualsPerAgentSum(t *testing.T) {
	sum := SeedResearcherMemoryConfigCount + SeedAnalystMemoryConfigCount + SeedPlannerMemoryConfigCount
	assert.Equal(t, SeedCapabilityAgentMemoryConfigCount, sum,
		"SeedCapabilityAgentMemoryConfigCount must equal the sum of per-agent counts (3+3+3=9)")
}

func TestSeedMemoryConfigMaxContextTokens_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedMemoryConfigMaxContextTokens,
		"SeedMemoryConfigMaxContextTokens must be a non-empty string")
}

func TestSeedMemoryConfigMaxContextTokens_IsCorrectString(t *testing.T) {
	assert.Equal(t, "max_context_tokens", SeedMemoryConfigMaxContextTokens,
		"SeedMemoryConfigMaxContextTokens must equal \"max_context_tokens\"")
}

func TestSeedMemoryConfigSummaryStrategy_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedMemoryConfigSummaryStrategy,
		"SeedMemoryConfigSummaryStrategy must be a non-empty string")
}

func TestSeedMemoryConfigSummaryStrategy_IsCorrectString(t *testing.T) {
	assert.Equal(t, "summary_strategy", SeedMemoryConfigSummaryStrategy,
		"SeedMemoryConfigSummaryStrategy must equal \"summary_strategy\"")
}

func TestSeedMemoryConfigPersistenceScope_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedMemoryConfigPersistenceScope,
		"SeedMemoryConfigPersistenceScope must be a non-empty string")
}

func TestSeedMemoryConfigPersistenceScope_IsCorrectString(t *testing.T) {
	assert.Equal(t, "persistence_scope", SeedMemoryConfigPersistenceScope,
		"SeedMemoryConfigPersistenceScope must equal \"persistence_scope\"")
}

func TestSeedMemoryConfigConfigKeys_AreDistinct(t *testing.T) {
	keys := []string{
		SeedMemoryConfigMaxContextTokens,
		SeedMemoryConfigSummaryStrategy,
		SeedMemoryConfigPersistenceScope,
	}
	unique := map[string]struct{}{}
	for _, k := range keys {
		unique[k] = struct{}{}
	}
	assert.Len(t, unique, 3,
		"migration 000102 must define exactly 3 distinct memory config key constants")
}

func TestSeedSummaryStrategyProgressive_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedSummaryStrategyProgressive,
		"SeedSummaryStrategyProgressive must be a non-empty string")
}

func TestSeedSummaryStrategySnapshot_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedSummaryStrategySnapshot,
		"SeedSummaryStrategySnapshot must be a non-empty string")
}

func TestSeedSummaryStrategyAppendOnly_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedSummaryStrategyAppendOnly,
		"SeedSummaryStrategyAppendOnly must be a non-empty string")
}

func TestSeedSummaryStrategies_AreDistinct(t *testing.T) {
	strategies := []string{
		SeedSummaryStrategyProgressive,
		SeedSummaryStrategySnapshot,
		SeedSummaryStrategyAppendOnly,
	}
	unique := map[string]struct{}{}
	for _, s := range strategies {
		unique[s] = struct{}{}
	}
	assert.Len(t, unique, 3,
		"three summary strategy constants must all be distinct values")
}

func TestSeedPersistenceScopeSession_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedPersistenceScopeSession,
		"SeedPersistenceScopeSession must be a non-empty string")
}

func TestSeedPersistenceScopeGlobal_IsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedPersistenceScopeGlobal,
		"SeedPersistenceScopeGlobal must be a non-empty string")
}

func TestSeedPersistenceScopes_AreDistinct(t *testing.T) {
	assert.NotEqual(t, SeedPersistenceScopeSession, SeedPersistenceScopeGlobal,
		"session and global persistence scope constants must be distinct values")
}

func TestSeedAnalystUsesGlobalPersistenceScope(t *testing.T) {
	// core-analyst persists globally to support cross-session analysis.
	// This is encoded in the migration; the constant allows the runtime to
	// detect this without hard-coding the string "global".
	assert.Equal(t, "global", SeedPersistenceScopeGlobal,
		"SeedPersistenceScopeGlobal must be \"global\" — analyst cross-session persistence requires this value")
}

func TestSeedResearcherAndPlannerUseSessionScope(t *testing.T) {
	// Both core-researcher and core-planner use session persistence scope.
	assert.Equal(t, "session", SeedPersistenceScopeSession,
		"SeedPersistenceScopeSession must be \"session\" — researcher and planner are scoped to the active session")
}
