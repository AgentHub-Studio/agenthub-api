package agentic_test

// context_bottleneck_bdd_test.go — BDD scenarios for §3.6 ContextBottleneckRegistry
//
// Each scenario follows the Given / When / Then pattern from the project test conventions.

import (
	"testing"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Scenario 1 — Registry is initialised with all §3.6 strategies
func TestBDD_ContextBottleneck_AllFiveStrategiesRegistered(t *testing.T) {
	t.Run("Scenario_AllFive_§3.6_StrategiesPresent", func(t *testing.T) {
		// Given a freshly constructed ContextBottleneckRegistry
		reg := agentic.NewContextBottleneckRegistry()

		// When all strategies are retrieved
		all := reg.AllStrategies()

		// Then exactly five strategies are returned (lazy_frontmatter, deferred_tool_schema,
		// summary_only_subagent, per_result_token_budget, progressive_summarization)
		assert.Len(t, all, 5,
			"§3.6 lists four named strategies plus progressive_summarization from §4.3")

		names := make([]agentic.ContextOptimizationStrategy, len(all))
		for i, p := range all {
			names[i] = p.Strategy
		}
		assert.Contains(t, names, agentic.StrategyLazyFrontmatter)
		assert.Contains(t, names, agentic.StrategyDeferredToolSchema)
		assert.Contains(t, names, agentic.StrategySummaryOnlySubagent)
		assert.Contains(t, names, agentic.StrategyPerResultTokenBudget)
		assert.Contains(t, names, agentic.StrategyProgressiveSummarization)
	})
}

// Scenario 2 — Deferred tool schema strategy targets "tool_schemas"
func TestBDD_ContextBottleneck_DeferredToolSchema_AppliesToToolSchemas(t *testing.T) {
	t.Run("Scenario_DeferredToolSchema_Targets_ToolSchemas", func(t *testing.T) {
		// Given the registry
		reg := agentic.NewContextBottleneckRegistry()

		// When querying strategies that apply to the "tool_schemas" context segment
		results := reg.StrategiesForAppliesTo("tool_schemas")

		// Then deferred_tool_schema is returned and is not CLI-specific
		require.Len(t, results, 1)
		p := results[0]
		assert.Equal(t, agentic.StrategyDeferredToolSchema, p.Strategy)
		assert.False(t, p.CLISpecific,
			"deferred_tool_schema applies to web/API deployments too (ToolSearch MCP)")
		assert.Equal(t, agentic.TokenSavingsLarge, p.TokenSavingsCategory)
	})
}

// Scenario 3 — Summary-only subagent prevents parent context inflation
func TestBDD_ContextBottleneck_SummaryOnlySubagent_PreventsHistoryInflation(t *testing.T) {
	t.Run("Scenario_SummaryOnlySubagent_SubagentResultsSegment", func(t *testing.T) {
		// Given the registry
		reg := agentic.NewContextBottleneckRegistry()

		// When filtering strategies that target "subagent_results"
		results := reg.StrategiesForAppliesTo("subagent_results")

		// Then exactly summary_only_subagent is returned with large savings category
		require.Len(t, results, 1)
		p := results[0]
		assert.Equal(t, agentic.StrategySummaryOnlySubagent, p.Strategy)
		assert.Equal(t, agentic.TokenSavingsLarge, p.TokenSavingsCategory)
		assert.False(t, p.CLISpecific, "summary-only return applies to AgentHub subagents")
	})
}

// Scenario 4 — lazy_frontmatter is CLI-specific but adapted for AgentHub web
func TestBDD_ContextBottleneck_LazyFrontmatter_WebAdaptationExists(t *testing.T) {
	t.Run("Scenario_LazyFrontmatter_HasWebAdaptation_ForAgentHub", func(t *testing.T) {
		// Given the registry
		reg := agentic.NewContextBottleneckRegistry()

		// When the lazy_frontmatter profile is inspected
		p, ok := reg.Profile(agentic.StrategyLazyFrontmatter)
		require.True(t, ok)

		// Then it is flagged as CLI-specific but has a non-empty WebAdaptation
		// so that AgentHubAdaptedStrategies() still includes it
		assert.True(t, p.CLISpecific,
			"CLAUDE.md directory nesting is a CLI concept")
		assert.NotEmpty(t, p.WebAdaptation,
			"AgentHub maps lazy loading to per-skill/per-KB instruction deferred injection")

		// And it is included in the adapted set
		adapted := reg.AgentHubAdaptedStrategies()
		found := false
		for _, ap := range adapted {
			if ap.Strategy == agentic.StrategyLazyFrontmatter {
				found = true
				break
			}
		}
		assert.True(t, found, "lazy_frontmatter with WebAdaptation must appear in AgentHubAdaptedStrategies")
	})
}

// Scenario 5 — progressive_summarization is the only extreme-savings strategy
func TestBDD_ContextBottleneck_ProgressiveSummarization_ExtremeCategory(t *testing.T) {
	t.Run("Scenario_ProgressiveSummarization_OnlyExtremeSavings", func(t *testing.T) {
		// Given the registry
		reg := agentic.NewContextBottleneckRegistry()

		// When all extreme-savings strategies are retrieved
		extreme := reg.StrategiesBySavingsCategory(agentic.TokenSavingsExtreme)

		// Then only progressive_summarization qualifies
		require.Len(t, extreme, 1,
			"auto-compact (compactConversation) is the only full-history collapse strategy")
		assert.Equal(t, agentic.StrategyProgressiveSummarization, extreme[0].Strategy)
		assert.Contains(t, extreme[0].AppliesTo, "conversation_history")
	})
}

// Scenario 6 — IsValidStrategy rejects unknown identifiers
func TestBDD_ContextBottleneck_IsValidStrategy_RejectsUnknown(t *testing.T) {
	t.Run("Scenario_IsValidStrategy_KnownAndUnknown", func(t *testing.T) {
		// Given the registry
		reg := agentic.NewContextBottleneckRegistry()

		// When a known strategy is validated
		valid := reg.IsValidStrategy(agentic.StrategyPerResultTokenBudget)

		// Then it is accepted
		assert.True(t, valid)

		// And when an arbitrary string is validated
		invalid := reg.IsValidStrategy("make_it_faster")

		// Then it is rejected
		assert.False(t, invalid,
			"only the five §3.6 strategies are registered; arbitrary strings are invalid")
	})
}
