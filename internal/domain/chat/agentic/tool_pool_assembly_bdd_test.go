package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for the §6.2 tool pool assembly five-step pipeline.

func TestBDD_ToolPoolAssemblyPipeline(t *testing.T) {
	reg := NewToolPoolAssemblyRegistry()

	t.Run("Scenario_FiveStepPipelineCoversFullAssembly", func(t *testing.T) {
		// Given §6.2 assembleToolPool() defines exactly five sequential steps
		// When the registry is queried for all steps
		// Then exactly five steps exist with orders 1..5
		assert.Equal(t, 5, len(reg.AllSteps()))
		assert.True(t, IsToolPoolAssemblySequentiallyOrdered())
	})

	t.Run("Scenario_MCPIntegrationComesAfterDenyRulePrefiltering", func(t *testing.T) {
		// Given deny rules must be evaluated on built-ins before MCP tools are merged
		// When step orders are compared
		// Then deny_rule_prefiltering (3) precedes mcp_tool_integration (4)
		denyP, _ := reg.Profile(ToolPoolStepDenyRulePrefiltering)
		mcpP, _ := reg.Profile(ToolPoolStepMCPIntegration)
		assert.Less(t, denyP.StepOrder, mcpP.StepOrder)
		assert.True(t, denyP.AlwaysPrecedesMCP)
		assert.False(t, mcpP.AlwaysPrecedesMCP)
	})

	t.Run("Scenario_MCPIntegrationIsConditionalNotAlwaysActive", func(t *testing.T) {
		// Given MCP integration is skipped when no MCP servers are configured
		// When the always-active flag is checked for mcp_tool_integration
		// Then it is false (step is conditional)
		p, ok := reg.Profile(ToolPoolStepMCPIntegration)
		assert.True(t, ok)
		assert.False(t, p.IsAlwaysActive)
	})

	t.Run("Scenario_DeduplicationIsLastAndAlwaysRuns", func(t *testing.T) {
		// Given deduplication must enforce name-uniqueness after all sources are merged
		// When the deduplication profile is checked
		// Then it has order 5, always runs, and can filter tools
		p, ok := reg.Profile(ToolPoolStepDeduplication)
		assert.True(t, ok)
		assert.Equal(t, 5, p.StepOrder)
		assert.True(t, p.IsAlwaysActive)
		assert.True(t, p.CanFilterTools)
	})

	t.Run("Scenario_BaseEnumerationOnlyAddsNeverFilters", func(t *testing.T) {
		// Given base_tool_enumeration populates the initial pool from the built-in registry
		// When the profile is inspected
		// Then CanFilterTools is false (this step only adds, never removes)
		p, ok := reg.Profile(ToolPoolStepBaseEnumeration)
		assert.True(t, ok)
		assert.False(t, p.CanFilterTools)
		assert.True(t, p.IsAlwaysActive)
	})
}
