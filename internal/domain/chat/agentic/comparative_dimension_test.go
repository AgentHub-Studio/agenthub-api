package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFEAT022_ComparativeDimensionCount verifies the registry contains exactly
// the six Table 3 dimensions from arXiv:2604.14228v1 §10.
func TestFEAT022_ComparativeDimensionCount(t *testing.T) {
	dims := NewComparativeDimensionRegistry()
	assert.Len(t, dims, ComparativeDimensionCount)
	assert.Equal(t, 6, ComparativeDimensionCount)
}

// TestFEAT022_ComparativeDimensionOrder verifies the registry is returned in
// Table 3 row order (system_scope first, multi_agent_routing last).
func TestFEAT022_ComparativeDimensionOrder(t *testing.T) {
	dims := NewComparativeDimensionRegistry()
	require.Len(t, dims, 6)
	assert.Equal(t, DimSystemScope, dims[0].ID)
	assert.Equal(t, DimTrustModel, dims[1].ID)
	assert.Equal(t, DimAgentRuntime, dims[2].ID)
	assert.Equal(t, DimExtensionArchitecture, dims[3].ID)
	assert.Equal(t, DimMemoryAndContext, dims[4].ID)
	assert.Equal(t, DimMultiAgentRouting, dims[5].ID)
}

// TestFEAT022_FindBySlug_AllDimensions verifies every dimension slug resolves.
func TestFEAT022_FindBySlug_AllDimensions(t *testing.T) {
	allIDs := []ComparativeDimensionID{
		DimSystemScope,
		DimTrustModel,
		DimAgentRuntime,
		DimExtensionArchitecture,
		DimMemoryAndContext,
		DimMultiAgentRouting,
	}
	for _, id := range allIDs {
		t.Run(string(id), func(t *testing.T) {
			p, ok := FindComparativeDimensionBySlug(id)
			require.True(t, ok, "dimension %q must be found", id)
			assert.Equal(t, id, p.ID)
		})
	}
}

// TestFEAT022_FindBySlug_Unknown returns false for an unknown slug.
func TestFEAT022_FindBySlug_Unknown(t *testing.T) {
	_, ok := FindComparativeDimensionBySlug("does_not_exist")
	assert.False(t, ok)
}

// TestFEAT022_SystemAnswer_ClaudeCode verifies each dimension has a non-empty
// ClaudeCode answer.
func TestFEAT022_SystemAnswer_ClaudeCode(t *testing.T) {
	for _, p := range NewComparativeDimensionRegistry() {
		t.Run(string(p.ID), func(t *testing.T) {
			ans := p.SystemAnswer(SystemClaudeCode)
			assert.NotEmpty(t, ans, "ClaudeCodeAnswer must be non-empty for %q", p.ID)
		})
	}
}

// TestFEAT022_SystemAnswer_OpenClaw verifies each dimension has a non-empty
// OpenClaw answer.
func TestFEAT022_SystemAnswer_OpenClaw(t *testing.T) {
	for _, p := range NewComparativeDimensionRegistry() {
		t.Run(string(p.ID), func(t *testing.T) {
			ans := p.SystemAnswer(SystemOpenClaw)
			assert.NotEmpty(t, ans, "OpenClawAnswer must be non-empty for %q", p.ID)
		})
	}
}

// TestFEAT022_SystemAnswer_UnknownSlug returns empty string.
func TestFEAT022_SystemAnswer_UnknownSlug(t *testing.T) {
	p, ok := FindComparativeDimensionBySlug(DimSystemScope)
	require.True(t, ok)
	ans := p.SystemAnswer("unknown_system")
	assert.Empty(t, ans)
}

// TestFEAT022_AllDimensions_HaveLabel verifies every dimension has a Label.
func TestFEAT022_AllDimensions_HaveLabel(t *testing.T) {
	for _, p := range NewComparativeDimensionRegistry() {
		assert.NotEmpty(t, p.Label, "Label must not be empty for %q", p.ID)
	}
}

// TestFEAT022_AllDimensions_HaveDesignQuestion verifies every dimension has a
// DesignQuestion.
func TestFEAT022_AllDimensions_HaveDesignQuestion(t *testing.T) {
	for _, p := range NewComparativeDimensionRegistry() {
		assert.NotEmpty(t, p.DesignQuestion, "DesignQuestion must not be empty for %q", p.ID)
	}
}

// TestFEAT022_AllDimensions_HaveKeyDivergence verifies every dimension has a
// KeyDivergence note.
func TestFEAT022_AllDimensions_HaveKeyDivergence(t *testing.T) {
	for _, p := range NewComparativeDimensionRegistry() {
		assert.NotEmpty(t, p.KeyDivergence, "KeyDivergence must not be empty for %q", p.ID)
	}
}

// TestFEAT022_ConvergenceDimensions verifies that only the dimensions noted in
// §10.2 as having convergence carry a ConvergenceNote.
func TestFEAT022_ConvergenceDimensions(t *testing.T) {
	cases := []struct {
		id          ComparativeDimensionID
		hasConverge bool
	}{
		{DimSystemScope, true},          // both are stackable
		{DimTrustModel, false},          // pure divergence
		{DimAgentRuntime, true},         // both follow ReAct
		{DimExtensionArchitecture, false},
		{DimMemoryAndContext, true},      // both use transparent file-based memory
		{DimMultiAgentRouting, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.id), func(t *testing.T) {
			p, ok := FindComparativeDimensionBySlug(tc.id)
			require.True(t, ok)
			assert.Equal(t, tc.hasConverge, p.HasConvergence(),
				"HasConvergence mismatch for %q", tc.id)
		})
	}
}

// TestFEAT022_SystemScope_Profile verifies the detail of the system_scope row.
func TestFEAT022_SystemScope_Profile(t *testing.T) {
	p, ok := FindComparativeDimensionBySlug(DimSystemScope)
	require.True(t, ok)
	assert.Contains(t, p.ClaudeCodeAnswer, "ephemeral")
	assert.Contains(t, p.OpenClawAnswer, "WebSocket")
	assert.Contains(t, p.KeyDivergence, "session-scoped")
	assert.Contains(t, p.ConvergenceNote, "ACP")
}

// TestFEAT022_TrustModel_Profile verifies the detail of the trust_model row.
func TestFEAT022_TrustModel_Profile(t *testing.T) {
	p, ok := FindComparativeDimensionBySlug(DimTrustModel)
	require.True(t, ok)
	assert.Contains(t, p.ClaudeCodeAnswer, "deny-first")
	assert.Contains(t, strings.ToLower(p.ClaudeCodeAnswer), "ml classifier")
	assert.Contains(t, p.OpenClawAnswer, "gateway")
	assert.Contains(t, p.KeyDivergence, "trust boundary")
	assert.Empty(t, p.ConvergenceNote)
}

// TestFEAT022_AgentRuntime_Profile verifies the detail of the agent_runtime row.
func TestFEAT022_AgentRuntime_Profile(t *testing.T) {
	p, ok := FindComparativeDimensionBySlug(DimAgentRuntime)
	require.True(t, ok)
	assert.Contains(t, p.ClaudeCodeAnswer, "queryLoop")
	assert.Contains(t, p.OpenClawAnswer, "Pi-agent")
	assert.Contains(t, p.KeyDivergence, "ReAct")
}

// TestFEAT022_ExtensionArchitecture_FourMechanisms verifies the "4 mechanisms"
// claim from §10.1 is captured in the Claude Code answer.
func TestFEAT022_ExtensionArchitecture_FourMechanisms(t *testing.T) {
	p, ok := FindComparativeDimensionBySlug(DimExtensionArchitecture)
	require.True(t, ok)
	assert.Contains(t, p.ClaudeCodeAnswer, "MCP")
	assert.Contains(t, p.ClaudeCodeAnswer, "plugins")
	assert.Contains(t, p.ClaudeCodeAnswer, "skills")
	assert.Contains(t, p.ClaudeCodeAnswer, "hooks")
}

// TestFEAT022_MemoryAndContext_FiveLayerPipeline verifies the compaction claim.
func TestFEAT022_MemoryAndContext_FiveLayerPipeline(t *testing.T) {
	p, ok := FindComparativeDimensionBySlug(DimMemoryAndContext)
	require.True(t, ok)
	assert.Contains(t, p.ClaudeCodeAnswer, "5-layer")
	assert.Contains(t, p.OpenClawAnswer, "MEMORY.md")
	assert.Contains(t, p.KeyDivergence, "long-term memory")
}

// TestFEAT022_MultiAgentRouting_MaxNestingDepth verifies the nesting depth claim
// (max 5) is captured in the OpenClaw answer.
func TestFEAT022_MultiAgentRouting_MaxNestingDepth(t *testing.T) {
	p, ok := FindComparativeDimensionBySlug(DimMultiAgentRouting)
	require.True(t, ok)
	assert.Contains(t, p.OpenClawAnswer, "5")
	assert.Contains(t, p.ClaudeCodeAnswer, "worktree")
}

// TestFEAT022_RegistryImmutability verifies that modifying the returned slice
// does not affect the canonical registry.
func TestFEAT022_RegistryImmutability(t *testing.T) {
	first := NewComparativeDimensionRegistry()
	first[0].Label = "MUTATED"
	second := NewComparativeDimensionRegistry()
	assert.NotEqual(t, "MUTATED", second[0].Label)
}

// TestFEAT022_SystemSlugConstants verifies the two system slug constants.
func TestFEAT022_SystemSlugConstants(t *testing.T) {
	assert.Equal(t, SystemSlug("claude_code"), SystemClaudeCode)
	assert.Equal(t, SystemSlug("openclaw"), SystemOpenClaw)
}
