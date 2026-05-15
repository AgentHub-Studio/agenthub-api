package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToolPoolAssemblySequence_HasFiveSteps(t *testing.T) {
	assert.Equal(t, 5, len(ToolPoolAssemblySequence))
}

func TestToolPoolAssemblySequence_IsSequentiallyOrdered(t *testing.T) {
	assert.True(t, IsToolPoolAssemblySequentiallyOrdered())
}

func TestToolPoolAssemblyRegistry_Profile_BaseEnumeration(t *testing.T) {
	reg := NewToolPoolAssemblyRegistry()
	p, ok := reg.Profile(ToolPoolStepBaseEnumeration)
	assert.True(t, ok)
	assert.Equal(t, 1, p.StepOrder)
	assert.True(t, p.IsAlwaysActive)
	assert.False(t, p.CanFilterTools)
	assert.True(t, p.AlwaysPrecedesMCP)
}

func TestToolPoolAssemblyRegistry_Profile_ModeFiltering(t *testing.T) {
	reg := NewToolPoolAssemblyRegistry()
	p, ok := reg.Profile(ToolPoolStepModeFiltering)
	assert.True(t, ok)
	assert.Equal(t, 2, p.StepOrder)
	assert.True(t, p.CanFilterTools)
	assert.True(t, p.AlwaysPrecedesMCP)
}

func TestToolPoolAssemblyRegistry_Profile_DenyRulePrefiltering(t *testing.T) {
	reg := NewToolPoolAssemblyRegistry()
	p, ok := reg.Profile(ToolPoolStepDenyRulePrefiltering)
	assert.True(t, ok)
	assert.Equal(t, 3, p.StepOrder)
	assert.True(t, p.CanFilterTools)
	assert.True(t, p.AlwaysPrecedesMCP)
}

func TestToolPoolAssemblyRegistry_Profile_MCPIntegration(t *testing.T) {
	reg := NewToolPoolAssemblyRegistry()
	p, ok := reg.Profile(ToolPoolStepMCPIntegration)
	assert.True(t, ok)
	assert.Equal(t, 4, p.StepOrder)
	assert.False(t, p.IsAlwaysActive, "mcp_tool_integration skipped when no MCP servers present")
	assert.False(t, p.CanFilterTools, "MCP integration only adds tools, never removes")
	assert.False(t, p.AlwaysPrecedesMCP)
}

func TestToolPoolAssemblyRegistry_Profile_Deduplication(t *testing.T) {
	reg := NewToolPoolAssemblyRegistry()
	p, ok := reg.Profile(ToolPoolStepDeduplication)
	assert.True(t, ok)
	assert.Equal(t, 5, p.StepOrder)
	assert.True(t, p.IsAlwaysActive)
	assert.True(t, p.CanFilterTools)
}

func TestToolPoolAssemblyRegistry_Profile_UnknownReturnsFalse(t *testing.T) {
	reg := NewToolPoolAssemblyRegistry()
	_, ok := reg.Profile("does_not_exist")
	assert.False(t, ok)
}

func TestToolPoolAssemblyRegistry_AllSteps_OrderPreserved(t *testing.T) {
	reg := NewToolPoolAssemblyRegistry()
	steps := reg.AllSteps()
	assert.Equal(t, 5, len(steps))
	assert.Equal(t, ToolPoolStepBaseEnumeration, steps[0])
	assert.Equal(t, ToolPoolStepDeduplication, steps[4])
}

func TestToolPoolAssemblyRegistry_AllSteps_IsDefensiveCopy(t *testing.T) {
	reg := NewToolPoolAssemblyRegistry()
	steps := reg.AllSteps()
	steps[0] = "mutated"
	assert.Equal(t, ToolPoolStepBaseEnumeration, reg.AllSteps()[0])
}

func TestToolPoolAssemblyRegistry_FilteringSteps_ThreeSteps(t *testing.T) {
	reg := NewToolPoolAssemblyRegistry()
	fs := reg.FilteringSteps()
	assert.Equal(t, 3, len(fs))
	assert.Contains(t, fs, ToolPoolStepModeFiltering)
	assert.Contains(t, fs, ToolPoolStepDenyRulePrefiltering)
	assert.Contains(t, fs, ToolPoolStepDeduplication)
}

func TestToolPoolAssemblyRegistry_PreMCPSteps_ThreeSteps(t *testing.T) {
	reg := NewToolPoolAssemblyRegistry()
	pre := reg.PreMCPSteps()
	assert.Equal(t, 3, len(pre))
	assert.Contains(t, pre, ToolPoolStepBaseEnumeration)
	assert.Contains(t, pre, ToolPoolStepModeFiltering)
	assert.Contains(t, pre, ToolPoolStepDenyRulePrefiltering)
	assert.NotContains(t, pre, ToolPoolStepMCPIntegration)
}

func TestToolPoolAssemblyRegistry_AlwaysActiveSteps_FourSteps(t *testing.T) {
	reg := NewToolPoolAssemblyRegistry()
	aa := reg.AlwaysActiveSteps()
	assert.Equal(t, 4, len(aa))
	assert.NotContains(t, aa, ToolPoolStepMCPIntegration)
}
