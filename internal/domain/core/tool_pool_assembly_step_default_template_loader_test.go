package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedToolPoolAssemblyStep_ExpectedRowCount(t *testing.T) {
	assert.Equal(t, 5, SeedExpectedToolPoolAssemblyStepRowCount)
}

func TestSeedToolPoolAssemblyStep_SlugCountMatchesRowCount(t *testing.T) {
	assert.Equal(t, SeedExpectedToolPoolAssemblyStepRowCount, len(SeedExpectedToolPoolAssemblyStepSlugs))
}

func TestSeedToolPoolAssemblyStep_SlugsContainBaseEnumeration(t *testing.T) {
	assert.Contains(t, SeedExpectedToolPoolAssemblyStepSlugs, "base_tool_enumeration")
}

func TestSeedToolPoolAssemblyStep_SlugsContainModeFiltering(t *testing.T) {
	assert.Contains(t, SeedExpectedToolPoolAssemblyStepSlugs, "mode_filtering")
}

func TestSeedToolPoolAssemblyStep_SlugsContainDenyRulePrefiltering(t *testing.T) {
	assert.Contains(t, SeedExpectedToolPoolAssemblyStepSlugs, "deny_rule_prefiltering")
}

func TestSeedToolPoolAssemblyStep_SlugsContainMCPIntegration(t *testing.T) {
	assert.Contains(t, SeedExpectedToolPoolAssemblyStepSlugs, "mcp_tool_integration")
}

func TestSeedToolPoolAssemblyStep_SlugsContainDeduplication(t *testing.T) {
	assert.Contains(t, SeedExpectedToolPoolAssemblyStepSlugs, "deduplication")
}

func TestSeedToolPoolAssemblyStep_ThreeFilteringSteps(t *testing.T) {
	assert.Equal(t, 3, len(SeedToolPoolAssemblyFilteringStepSlugs))
}

func TestSeedToolPoolAssemblyStep_ThreePreMCPSteps(t *testing.T) {
	assert.Equal(t, 3, len(SeedToolPoolAssemblyPreMCPStepSlugs))
}

func TestSeedToolPoolAssemblyStep_OneConditionalStep(t *testing.T) {
	assert.Equal(t, 1, len(SeedToolPoolAssemblyConditionalStepSlugs))
	assert.Contains(t, SeedToolPoolAssemblyConditionalStepSlugs, "mcp_tool_integration")
}

func TestSeedToolPoolAssemblyStep_MCPIntegrationNotInPreMCP(t *testing.T) {
	for _, s := range SeedToolPoolAssemblyPreMCPStepSlugs {
		assert.NotEqual(t, "mcp_tool_integration", s)
	}
}

func TestSeedToolPoolAssemblyStep_FilteringSlugsAreSubsetOfAll(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedToolPoolAssemblyStepSlugs {
		all[s] = true
	}
	for _, s := range SeedToolPoolAssemblyFilteringStepSlugs {
		assert.True(t, all[s], "filtering slug %q not in canonical list", s)
	}
}
