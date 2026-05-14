package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaxonomyGradient_FourCategories(t *testing.T) {
	assert.Equal(t, 4, len(TaxonomyGradient))
}

func TestTaxonomyGradient_OrderPassiveFirst(t *testing.T) {
	assert.Equal(t, AgentCategoryInlineCompletion, TaxonomyGradient[0])
	assert.Equal(t, AgentCategoryFullyAutonomous, TaxonomyGradient[3])
}

func TestCodingAgentTaxonomyRegistry_ProfileInlineCompletion(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()
	p, ok := r.Profile(AgentCategoryInlineCompletion)
	require.True(t, ok)
	assert.Equal(t, TaxonomyGradientPassive, p.GradientIndex)
	assert.Equal(t, AgentExecPatternEditorPlugin, p.ExecutionPattern)
	assert.Equal(t, AgentIsolationNone, p.IsolationModel)
	assert.False(t, p.IsAgentHubTarget, "inline completion is not an AgentHub target")
}

func TestCodingAgentTaxonomyRegistry_ProfileChatIntegrated(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()
	p, ok := r.Profile(AgentCategoryChatIntegrated)
	require.True(t, ok)
	assert.Equal(t, TaxonomyGradientInteractive, p.GradientIndex)
	assert.Equal(t, AgentExecPatternIDECoupledProduct, p.ExecutionPattern)
	assert.True(t, p.IsAgentHubTarget, "AgentHub is chat_integrated by primary deployment")
}

func TestCodingAgentTaxonomyRegistry_ProfileAgenticCLI(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()
	p, ok := r.Profile(AgentCategoryAgenticCLI)
	require.True(t, ok)
	assert.Equal(t, TaxonomyGradientAgentic, p.GradientIndex)
	assert.Equal(t, AgentExecPatternToolUseLoop, p.ExecutionPattern)
	assert.Equal(t, AgentIsolationPermissionGates, p.IsolationModel)
	assert.True(t, p.IsAgentHubTarget, "AgentHub background agents use tool-use loops")
}

func TestCodingAgentTaxonomyRegistry_ProfileFullyAutonomous(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()
	p, ok := r.Profile(AgentCategoryFullyAutonomous)
	require.True(t, ok)
	assert.Equal(t, TaxonomyGradientAutonomous, p.GradientIndex)
	assert.Equal(t, AgentExecPatternSandboxPlanning, p.ExecutionPattern)
	assert.Equal(t, AgentIsolationSandbox, p.IsolationModel)
	assert.False(t, p.IsAgentHubTarget, "fully autonomous (sandbox) is not the current AgentHub model")
}

func TestCodingAgentTaxonomyRegistry_ProfileUnknown(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()
	_, ok := r.Profile("unknown-category")
	assert.False(t, ok)
}

func TestCodingAgentTaxonomyRegistry_AllCategoriesLength(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()
	assert.Equal(t, 4, len(r.AllCategories()))
}

func TestCodingAgentTaxonomyRegistry_AllCategoriesIsCopy(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()
	cats := r.AllCategories()
	cats[0] = "mutated"
	assert.Equal(t, AgentCategoryInlineCompletion, TaxonomyGradient[0], "original gradient must not be mutated")
}

func TestCodingAgentTaxonomyRegistry_AgentHubTargetCategories(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()
	targets := r.AgentHubTargetCategories()
	assert.Equal(t, 2, len(targets), "AgentHub targets chat_integrated and agentic_cli")
	assert.Equal(t, AgentCategoryChatIntegrated, targets[0])
	assert.Equal(t, AgentCategoryAgenticCLI, targets[1])
}

func TestCodingAgentTaxonomyRegistry_FindByExecutionPattern(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()

	cat, ok := r.FindByExecutionPattern(AgentExecPatternToolUseLoop)
	require.True(t, ok)
	assert.Equal(t, AgentCategoryAgenticCLI, cat)

	cat2, ok2 := r.FindByExecutionPattern(AgentExecPatternSandboxPlanning)
	require.True(t, ok2)
	assert.Equal(t, AgentCategoryFullyAutonomous, cat2)
}

func TestCodingAgentTaxonomyRegistry_FindByExecutionPatternUnknown(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()
	_, ok := r.FindByExecutionPattern("unknown-pattern")
	assert.False(t, ok)
}

func TestCodingAgentTaxonomyRegistry_FindByIsolationModel(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()

	cats := r.FindByIsolationModel(AgentIsolationPermissionGates)
	assert.Equal(t, 1, len(cats))
	assert.Equal(t, AgentCategoryAgenticCLI, cats[0])

	cats2 := r.FindByIsolationModel(AgentIsolationNone)
	assert.Equal(t, 1, len(cats2))
	assert.Equal(t, AgentCategoryInlineCompletion, cats2[0])
}

func TestCodingAgentTaxonomyRegistry_FindByIsolationModelNotFound(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()
	cats := r.FindByIsolationModel("unknown-isolation")
	assert.Empty(t, cats)
}

func TestCodingAgentTaxonomyRegistry_IsMoreAutonomousThan(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()
	assert.True(t, r.IsMoreAutonomousThan(AgentCategoryFullyAutonomous, AgentCategoryInlineCompletion))
	assert.True(t, r.IsMoreAutonomousThan(AgentCategoryAgenticCLI, AgentCategoryChatIntegrated))
	assert.False(t, r.IsMoreAutonomousThan(AgentCategoryInlineCompletion, AgentCategoryAgenticCLI))
	assert.False(t, r.IsMoreAutonomousThan(AgentCategoryAgenticCLI, AgentCategoryAgenticCLI))
}

func TestCodingAgentTaxonomyRegistry_IsMoreAutonomousThanUnknown(t *testing.T) {
	r := NewCodingAgentTaxonomyRegistry()
	assert.False(t, r.IsMoreAutonomousThan("unknown", AgentCategoryAgenticCLI))
	assert.False(t, r.IsMoreAutonomousThan(AgentCategoryAgenticCLI, "unknown"))
}

func TestCodingAgentTaxonomy_AllProfilesHaveDistinctGradientIndices(t *testing.T) {
	seen := map[TaxonomyGradientIndex]bool{}
	for _, cat := range TaxonomyGradient {
		p := codingAgentProfiles[cat]
		assert.False(t, seen[p.GradientIndex], "duplicate gradient index %d for %q", p.GradientIndex, cat)
		seen[p.GradientIndex] = true
	}
}

func TestCodingAgentTaxonomy_AllProfilesHaveNonEmptyExamples(t *testing.T) {
	for _, cat := range TaxonomyGradient {
		p := codingAgentProfiles[cat]
		assert.NotEmpty(t, p.ExampleSystems, "category %q must have example systems", cat)
	}
}

func TestCodingAgentTaxonomy_ChatIntegratedContainsAgentHub(t *testing.T) {
	p := codingAgentProfiles[AgentCategoryChatIntegrated]
	found := false
	for _, ex := range p.ExampleSystems {
		if ex == "AgentHub" {
			found = true
		}
	}
	assert.True(t, found, "AgentHub must appear in chat_integrated example systems")
}

func TestCodingAgentTaxonomy_ClaudeCodeInAgenticCLI(t *testing.T) {
	p := codingAgentProfiles[AgentCategoryAgenticCLI]
	found := false
	for _, ex := range p.ExampleSystems {
		if ex == "Claude Code" {
			found = true
		}
	}
	assert.True(t, found, "Claude Code must appear in agentic_cli example systems")
}
