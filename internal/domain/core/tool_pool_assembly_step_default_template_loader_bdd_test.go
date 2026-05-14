package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for the §6.2 tool pool assembly step seed in ah_core.

func TestBDD_AhCoreToolPoolAssemblyStepSeed(t *testing.T) {
	t.Run("Scenario_FiveStepsCoverFullAssemblyPipeline", func(t *testing.T) {
		// Given §6.2 assembleToolPool() defines exactly five sequential steps
		// When the seed constants are inspected
		// Then five slugs exist matching the canonical names
		assert.Equal(t, 5, SeedExpectedToolPoolAssemblyStepRowCount)
		assert.Contains(t, SeedExpectedToolPoolAssemblyStepSlugs, "base_tool_enumeration")
		assert.Contains(t, SeedExpectedToolPoolAssemblyStepSlugs, "deduplication")
	})

	t.Run("Scenario_MCPIntegrationIsOnlyConditionalStep", func(t *testing.T) {
		// Given MCP integration is skipped when no MCP servers are present
		// When the conditional step list is checked
		// Then only mcp_tool_integration is conditional
		assert.Equal(t, 1, len(SeedToolPoolAssemblyConditionalStepSlugs))
		assert.Equal(t, "mcp_tool_integration", SeedToolPoolAssemblyConditionalStepSlugs[0])
	})

	t.Run("Scenario_ThreeStepsPrecedeMCPIntegration", func(t *testing.T) {
		// Given deny rules must be applied to built-ins before MCP tools are merged
		// When the pre-MCP steps are listed
		// Then base_tool_enumeration, mode_filtering, deny_rule_prefiltering precede MCP
		assert.Equal(t, 3, len(SeedToolPoolAssemblyPreMCPStepSlugs))
		assert.Contains(t, SeedToolPoolAssemblyPreMCPStepSlugs, "deny_rule_prefiltering")
		assert.NotContains(t, SeedToolPoolAssemblyPreMCPStepSlugs, "mcp_tool_integration")
	})

	t.Run("Scenario_ThreeStepsCanFilterTools", func(t *testing.T) {
		// Given mode_filtering, deny_rule_prefiltering, deduplication reduce the tool set
		// When filtering steps are counted
		// Then exactly three steps have can_filter_tools=true
		assert.Equal(t, 3, len(SeedToolPoolAssemblyFilteringStepSlugs))
		assert.Contains(t, SeedToolPoolAssemblyFilteringStepSlugs, "mode_filtering")
		assert.Contains(t, SeedToolPoolAssemblyFilteringStepSlugs, "deny_rule_prefiltering")
		assert.Contains(t, SeedToolPoolAssemblyFilteringStepSlugs, "deduplication")
	})

	t.Run("Scenario_BaseEnumerationDoesNotFilterOnlyAdds", func(t *testing.T) {
		// Given base_tool_enumeration only populates the pool, never removes entries
		// When the filtering slug list is checked
		// Then base_tool_enumeration is absent from filtering steps
		for _, s := range SeedToolPoolAssemblyFilteringStepSlugs {
			assert.NotEqual(t, "base_tool_enumeration", s)
		}
	})
}
