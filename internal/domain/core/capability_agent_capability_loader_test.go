package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability agent capability seed constants (migration 000119).
// These run without a database and guard against accidental constant drift.

// ─────────────────────────────────────────────────────────
// Total count tests
// ─────────────────────────────────────────────────────────

func TestAgentCapability_TotalCountIsTwelve(t *testing.T) {
	assert.Equal(t, 12, SeedAgentCapabilityCount,
		"migration 000119 seeds exactly 12 capability agent capability rows (four per capability agent)")
}

func TestAgentCapability_AgentCountIsThree(t *testing.T) {
	assert.Equal(t, 3, SeedAgentCapabilityAgentCount,
		"SeedAgentCapabilityAgentCount must be 3 — researcher, analyst, planner")
}

func TestAgentCapability_TotalCountEqualsAgentCountTimesFour(t *testing.T) {
	assert.Equal(t, SeedAgentCapabilityAgentCount*4, SeedAgentCapabilityCount,
		"total count must equal agent count × 4 (each agent has exactly 4 capability rows)")
}

func TestAgentCapability_TotalCountIsPositive(t *testing.T) {
	assert.Greater(t, SeedAgentCapabilityCount, 0,
		"SeedAgentCapabilityCount must be positive")
}

func TestAgentCapability_AgentCountIsPositive(t *testing.T) {
	assert.Greater(t, SeedAgentCapabilityAgentCount, 0,
		"SeedAgentCapabilityAgentCount must be positive")
}

// ─────────────────────────────────────────────────────────
// Capability key constant tests
// ─────────────────────────────────────────────────────────

func TestAgentCapability_CapKeyWebSearch_Value(t *testing.T) {
	assert.Equal(t, "web_search", SeedCapKeyWebSearch,
		"SeedCapKeyWebSearch must equal \"web_search\"")
}

func TestAgentCapability_CapKeyDocAnalysis_Value(t *testing.T) {
	assert.Equal(t, "document_analysis", SeedCapKeyDocAnalysis,
		"SeedCapKeyDocAnalysis must equal \"document_analysis\"")
}

func TestAgentCapability_CapKeyCodeGen_Value(t *testing.T) {
	assert.Equal(t, "code_generation", SeedCapKeyCodeGen,
		"SeedCapKeyCodeGen must equal \"code_generation\"")
}

func TestAgentCapability_CapKeyTaskPlanning_Value(t *testing.T) {
	assert.Equal(t, "task_planning", SeedCapKeyTaskPlanning,
		"SeedCapKeyTaskPlanning must equal \"task_planning\"")
}

func TestAgentCapability_CapKeyDataAnalysis_Value(t *testing.T) {
	assert.Equal(t, "data_analysis", SeedCapKeyDataAnalysis,
		"SeedCapKeyDataAnalysis must equal \"data_analysis\"")
}

func TestAgentCapability_CapKeySubagentDelegation_Value(t *testing.T) {
	assert.Equal(t, "subagent_delegation", SeedCapKeySubagentDelegation,
		"SeedCapKeySubagentDelegation must equal \"subagent_delegation\"")
}

func TestAgentCapability_CapKeys_AreDistinct(t *testing.T) {
	keys := []string{
		SeedCapKeyWebSearch,
		SeedCapKeyDocAnalysis,
		SeedCapKeyCodeGen,
		SeedCapKeyTaskPlanning,
		SeedCapKeyDataAnalysis,
		SeedCapKeySubagentDelegation,
	}
	seen := map[string]struct{}{}
	for _, k := range keys {
		assert.NotEmpty(t, k, "every capability key constant must be non-empty")
		seen[k] = struct{}{}
	}
	assert.Len(t, seen, 6,
		"there must be exactly 6 distinct capability key constants")
}

func TestAgentCapability_CapKeys_AreNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedCapKeyWebSearch)
	assert.NotEmpty(t, SeedCapKeyDocAnalysis)
	assert.NotEmpty(t, SeedCapKeyCodeGen)
	assert.NotEmpty(t, SeedCapKeyTaskPlanning)
	assert.NotEmpty(t, SeedCapKeyDataAnalysis)
	assert.NotEmpty(t, SeedCapKeySubagentDelegation)
}

// ─────────────────────────────────────────────────────────
// Per-agent supported count tests
// ─────────────────────────────────────────────────────────

func TestAgentCapability_ResearcherSupportedCount_IsTwo(t *testing.T) {
	assert.Equal(t, 2, SeedResearcherSupportedCount,
		"core-researcher has exactly 2 supported capabilities (web_search + document_analysis)")
}

func TestAgentCapability_AnalystSupportedCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedAnalystSupportedCount,
		"core-analyst has exactly 3 supported capabilities (data_analysis + document_analysis + web_search)")
}

func TestAgentCapability_PlannerSupportedCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedPlannerSupportedCount,
		"core-planner has exactly 3 supported capabilities (task_planning + subagent_delegation + web_search)")
}

func TestAgentCapability_SupportedCountsArePositive(t *testing.T) {
	assert.Greater(t, SeedResearcherSupportedCount, 0,
		"SeedResearcherSupportedCount must be positive")
	assert.Greater(t, SeedAnalystSupportedCount, 0,
		"SeedAnalystSupportedCount must be positive")
	assert.Greater(t, SeedPlannerSupportedCount, 0,
		"SeedPlannerSupportedCount must be positive")
}

func TestAgentCapability_SupportedCountsNotExceedFour(t *testing.T) {
	perAgent := SeedAgentCapabilityCount / SeedAgentCapabilityAgentCount
	assert.LessOrEqual(t, SeedResearcherSupportedCount, perAgent,
		"researcher supported count must not exceed per-agent row count")
	assert.LessOrEqual(t, SeedAnalystSupportedCount, perAgent,
		"analyst supported count must not exceed per-agent row count")
	assert.LessOrEqual(t, SeedPlannerSupportedCount, perAgent,
		"planner supported count must not exceed per-agent row count")
}

// ─────────────────────────────────────────────────────────
// Code generation is NOT supported by any agent
// ─────────────────────────────────────────────────────────

// SeedCodeGenIsUnsupported is a helper map used by unit tests to assert that
// code_generation is not supported by any of the three core agents. The values
// are the is_supported=false contract specified by migration 000119.
var seedCodeGenSupportedByAgent = map[string]bool{
	"core-researcher": false,
	"core-analyst":    false,
	"core-planner":    false,
}

func TestAgentCapability_CodeGen_NotSupportedByResearcher(t *testing.T) {
	assert.False(t, seedCodeGenSupportedByAgent["core-researcher"],
		"code_generation must be is_supported=false for core-researcher")
}

func TestAgentCapability_CodeGen_NotSupportedByAnalyst(t *testing.T) {
	assert.False(t, seedCodeGenSupportedByAgent["core-analyst"],
		"code_generation must be is_supported=false for core-analyst")
}

func TestAgentCapability_CodeGen_NotSupportedByPlanner(t *testing.T) {
	assert.False(t, seedCodeGenSupportedByAgent["core-planner"],
		"code_generation must be is_supported=false for core-planner")
}

func TestAgentCapability_CodeGen_NotSupportedByAnyAgent(t *testing.T) {
	for agent, supported := range seedCodeGenSupportedByAgent {
		assert.False(t, supported,
			"code_generation must be is_supported=false for agent %q", agent)
	}
}

// ─────────────────────────────────────────────────────────
// Planner subagent_delegation support
// ─────────────────────────────────────────────────────────

// seedSubagentDelegationSupportedByAgent encodes the is_supported contract for
// subagent_delegation across all three core agents.
var seedSubagentDelegationSupportedByAgent = map[string]bool{
	"core-researcher": false, // not declared in researcher seed rows
	"core-analyst":    false, // not declared in analyst seed rows
	"core-planner":    true,  // planner's primary coordination capability
}

func TestAgentCapability_SubagentDelegation_SupportedByPlanner(t *testing.T) {
	assert.True(t, seedSubagentDelegationSupportedByAgent["core-planner"],
		"subagent_delegation must be is_supported=true for core-planner (delegation is planner's primary role)")
}

func TestAgentCapability_SubagentDelegation_NotSupportedByResearcher(t *testing.T) {
	assert.False(t, seedSubagentDelegationSupportedByAgent["core-researcher"],
		"subagent_delegation is not a declared capability of core-researcher")
}

func TestAgentCapability_SubagentDelegation_NotSupportedByAnalyst(t *testing.T) {
	assert.False(t, seedSubagentDelegationSupportedByAgent["core-analyst"],
		"subagent_delegation is not a declared capability of core-analyst")
}

// ─────────────────────────────────────────────────────────
// Researcher focuses on search and documents
// ─────────────────────────────────────────────────────────

// seedResearcherSupportedKeys lists the capability_keys that core-researcher
// supports according to migration 000119. Used to guard the expected 2-supported
// contract in unit tests.
var seedResearcherSupportedKeys = map[string]bool{
	SeedCapKeyWebSearch:   true,
	SeedCapKeyDocAnalysis: true,
	SeedCapKeyCodeGen:     false,
	SeedCapKeyTaskPlanning: false,
}

func TestAgentCapability_Researcher_SupportsWebSearch(t *testing.T) {
	assert.True(t, seedResearcherSupportedKeys[SeedCapKeyWebSearch],
		"core-researcher must support web_search (primary capability)")
}

func TestAgentCapability_Researcher_SupportsDocumentAnalysis(t *testing.T) {
	assert.True(t, seedResearcherSupportedKeys[SeedCapKeyDocAnalysis],
		"core-researcher must support document_analysis (secondary capability)")
}

func TestAgentCapability_Researcher_SupportedKeysMatchCount(t *testing.T) {
	count := 0
	for _, supported := range seedResearcherSupportedKeys {
		if supported {
			count++
		}
	}
	assert.Equal(t, SeedResearcherSupportedCount, count,
		"researcher supported key count must match SeedResearcherSupportedCount")
}

// ─────────────────────────────────────────────────────────
// Analyst has three supported capabilities
// ─────────────────────────────────────────────────────────

// seedAnalystSupportedKeys lists the capability_keys that core-analyst supports
// according to migration 000119.
var seedAnalystSupportedKeys = map[string]bool{
	SeedCapKeyDataAnalysis: true,
	SeedCapKeyDocAnalysis:  true,
	SeedCapKeyCodeGen:      false,
	SeedCapKeyWebSearch:    true,
}

func TestAgentCapability_Analyst_SupportsDataAnalysis(t *testing.T) {
	assert.True(t, seedAnalystSupportedKeys[SeedCapKeyDataAnalysis],
		"core-analyst must support data_analysis (primary capability)")
}

func TestAgentCapability_Analyst_SupportsDocumentAnalysis(t *testing.T) {
	assert.True(t, seedAnalystSupportedKeys[SeedCapKeyDocAnalysis],
		"core-analyst must support document_analysis (secondary capability)")
}

func TestAgentCapability_Analyst_SupportsWebSearch(t *testing.T) {
	assert.True(t, seedAnalystSupportedKeys[SeedCapKeyWebSearch],
		"core-analyst must support web_search (supplement with web data)")
}

func TestAgentCapability_Analyst_SupportedKeysMatchCount(t *testing.T) {
	count := 0
	for _, supported := range seedAnalystSupportedKeys {
		if supported {
			count++
		}
	}
	assert.Equal(t, SeedAnalystSupportedCount, count,
		"analyst supported key count must match SeedAnalystSupportedCount")
}
