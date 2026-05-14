package agentic_test

import (
	"testing"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ContextOptimizationStrategy constants
// ---------------------------------------------------------------------------

func TestContextBottleneck_StrategyConstants(t *testing.T) {
	assert.Equal(t, agentic.ContextOptimizationStrategy("lazy_frontmatter"), agentic.StrategyLazyFrontmatter)
	assert.Equal(t, agentic.ContextOptimizationStrategy("deferred_tool_schema"), agentic.StrategyDeferredToolSchema)
	assert.Equal(t, agentic.ContextOptimizationStrategy("summary_only_subagent"), agentic.StrategySummaryOnlySubagent)
	assert.Equal(t, agentic.ContextOptimizationStrategy("per_result_token_budget"), agentic.StrategyPerResultTokenBudget)
	assert.Equal(t, agentic.ContextOptimizationStrategy("progressive_summarization"), agentic.StrategyProgressiveSummarization)
}

// ---------------------------------------------------------------------------
// TokenSavingsCategory constants
// ---------------------------------------------------------------------------

func TestContextBottleneck_TokenSavingsCategoryConstants(t *testing.T) {
	assert.Equal(t, agentic.TokenSavingsCategory("small"), agentic.TokenSavingsSmall)
	assert.Equal(t, agentic.TokenSavingsCategory("medium"), agentic.TokenSavingsMedium)
	assert.Equal(t, agentic.TokenSavingsCategory("large"), agentic.TokenSavingsLarge)
	assert.Equal(t, agentic.TokenSavingsCategory("extreme"), agentic.TokenSavingsExtreme)
}

// ---------------------------------------------------------------------------
// AllStrategies
// ---------------------------------------------------------------------------

func TestContextBottleneck_AllStrategies_Count(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	all := reg.AllStrategies()
	assert.Len(t, all, 5, "expected exactly 5 §3.6 optimization strategies")
}

func TestContextBottleneck_AllStrategies_Order(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	all := reg.AllStrategies()
	require.Len(t, all, 5)
	assert.Equal(t, agentic.StrategyLazyFrontmatter, all[0].Strategy)
	assert.Equal(t, agentic.StrategyDeferredToolSchema, all[1].Strategy)
	assert.Equal(t, agentic.StrategySummaryOnlySubagent, all[2].Strategy)
	assert.Equal(t, agentic.StrategyPerResultTokenBudget, all[3].Strategy)
	assert.Equal(t, agentic.StrategyProgressiveSummarization, all[4].Strategy)
}

func TestContextBottleneck_AllStrategies_NoEmptyDescription(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	for _, p := range reg.AllStrategies() {
		assert.NotEmpty(t, p.Description, "strategy %s must have a description", p.Strategy)
	}
}

func TestContextBottleneck_AllStrategies_AppliesToNonEmpty(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	for _, p := range reg.AllStrategies() {
		assert.NotEmpty(t, p.AppliesTo, "strategy %s must specify at least one AppliesTo target", p.Strategy)
	}
}

// ---------------------------------------------------------------------------
// StrategiesForAppliesTo
// ---------------------------------------------------------------------------

func TestContextBottleneck_StrategiesForAppliesTo_ToolSchemas(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	results := reg.StrategiesForAppliesTo("tool_schemas")
	require.Len(t, results, 1)
	assert.Equal(t, agentic.StrategyDeferredToolSchema, results[0].Strategy)
}

func TestContextBottleneck_StrategiesForAppliesTo_SubagentResults(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	results := reg.StrategiesForAppliesTo("subagent_results")
	require.Len(t, results, 1)
	assert.Equal(t, agentic.StrategySummaryOnlySubagent, results[0].Strategy)
}

func TestContextBottleneck_StrategiesForAppliesTo_ToolResults(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	results := reg.StrategiesForAppliesTo("tool_results")
	require.Len(t, results, 1)
	assert.Equal(t, agentic.StrategyPerResultTokenBudget, results[0].Strategy)
}

func TestContextBottleneck_StrategiesForAppliesTo_CaseInsensitive(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	lower := reg.StrategiesForAppliesTo("tool_schemas")
	upper := reg.StrategiesForAppliesTo("TOOL_SCHEMAS")
	assert.Equal(t, lower, upper)
}

func TestContextBottleneck_StrategiesForAppliesTo_Unknown(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	results := reg.StrategiesForAppliesTo("nonexistent_target")
	assert.Empty(t, results)
}

// ---------------------------------------------------------------------------
// StrategiesBySavingsCategory
// ---------------------------------------------------------------------------

func TestContextBottleneck_BySavingsCategory_Large(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	large := reg.StrategiesBySavingsCategory(agentic.TokenSavingsLarge)
	require.Len(t, large, 2, "deferred_tool_schema and summary_only_subagent are both large")
	strategies := make([]agentic.ContextOptimizationStrategy, len(large))
	for i, p := range large {
		strategies[i] = p.Strategy
	}
	assert.Contains(t, strategies, agentic.StrategyDeferredToolSchema)
	assert.Contains(t, strategies, agentic.StrategySummaryOnlySubagent)
}

func TestContextBottleneck_BySavingsCategory_Extreme(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	extreme := reg.StrategiesBySavingsCategory(agentic.TokenSavingsExtreme)
	require.Len(t, extreme, 1)
	assert.Equal(t, agentic.StrategyProgressiveSummarization, extreme[0].Strategy)
}

func TestContextBottleneck_BySavingsCategory_Medium(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	medium := reg.StrategiesBySavingsCategory(agentic.TokenSavingsMedium)
	require.Len(t, medium, 2, "lazy_frontmatter and per_result_token_budget are medium")
}

func TestContextBottleneck_BySavingsCategory_Small(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	small := reg.StrategiesBySavingsCategory(agentic.TokenSavingsSmall)
	assert.Empty(t, small, "no §3.6 strategy is classified as small savings")
}

// ---------------------------------------------------------------------------
// IsValidStrategy
// ---------------------------------------------------------------------------

func TestContextBottleneck_IsValidStrategy_AllValid(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	valid := []agentic.ContextOptimizationStrategy{
		agentic.StrategyLazyFrontmatter,
		agentic.StrategyDeferredToolSchema,
		agentic.StrategySummaryOnlySubagent,
		agentic.StrategyPerResultTokenBudget,
		agentic.StrategyProgressiveSummarization,
	}
	for _, s := range valid {
		assert.True(t, reg.IsValidStrategy(s), "expected %s to be valid", s)
	}
}

func TestContextBottleneck_IsValidStrategy_Invalid(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	assert.False(t, reg.IsValidStrategy("not_a_real_strategy"))
	assert.False(t, reg.IsValidStrategy(""))
}

// ---------------------------------------------------------------------------
// AgentHubAdaptedStrategies
// ---------------------------------------------------------------------------

func TestContextBottleneck_AgentHubAdaptedStrategies_AllFive(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	adapted := reg.AgentHubAdaptedStrategies()
	// lazy_frontmatter is CLI-specific but has a WebAdaptation, so it is included.
	assert.Len(t, adapted, 5, "all 5 strategies should be adapted for AgentHub web surface")
}

func TestContextBottleneck_AgentHubAdaptedStrategies_LazyFrontmatterHasWebAdaptation(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	adapted := reg.AgentHubAdaptedStrategies()
	var found bool
	for _, p := range adapted {
		if p.Strategy == agentic.StrategyLazyFrontmatter {
			found = true
			assert.NotEmpty(t, p.WebAdaptation, "lazy_frontmatter must have a WebAdaptation for AgentHub")
		}
	}
	assert.True(t, found, "lazy_frontmatter must appear in AgentHubAdaptedStrategies")
}

// ---------------------------------------------------------------------------
// Profile
// ---------------------------------------------------------------------------

func TestContextBottleneck_Profile_KnownStrategy(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	p, ok := reg.Profile(agentic.StrategyDeferredToolSchema)
	require.True(t, ok)
	assert.Equal(t, agentic.TokenSavingsLarge, p.TokenSavingsCategory)
	assert.Contains(t, p.AppliesTo, "tool_schemas")
	assert.False(t, p.CLISpecific)
}

func TestContextBottleneck_Profile_UnknownStrategy(t *testing.T) {
	reg := agentic.NewContextBottleneckRegistry()
	_, ok := reg.Profile("unknown")
	assert.False(t, ok)
}
