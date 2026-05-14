package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit tests for AgentLoopInjectionPointRegistry — FEAT030
// Source: §6.1 / Figure 5 of arXiv:2604.14228v1 (Claude Code architecture paper, page 16).

func TestFEAT030_ThreeInjectionPointsExist(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	all := r.AllInjectionPoints()
	assert.Len(t, all, SeedAgentLoopInjectionPointCount,
		"Figure 5 defines exactly 3 injection points")
}

func TestFEAT030_SeedSlugsMatchCanonical(t *testing.T) {
	assert.Equal(t, []string{"assemble", "model", "execute"}, SeedAgentLoopInjectionPointSlugs)
}

func TestFEAT030_FindInjectionPointBySlug_Assemble(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	p, ok := r.FindInjectionPointBySlug("assemble")
	require.True(t, ok)
	assert.Equal(t, AgentLoopInjectionPointAssemble, p.Point)
	assert.Equal(t, "a", p.FigureAnnotation)
	assert.Equal(t, 1, p.LoopPhaseOrder)
	assert.Equal(t, "6.1", p.PDFSection)
}

func TestFEAT030_FindInjectionPointBySlug_Model(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	p, ok := r.FindInjectionPointBySlug("model")
	require.True(t, ok)
	assert.Equal(t, AgentLoopInjectionPointModel, p.Point)
	assert.Equal(t, "b", p.FigureAnnotation)
	assert.Equal(t, 2, p.LoopPhaseOrder)
}

func TestFEAT030_FindInjectionPointBySlug_Execute(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	p, ok := r.FindInjectionPointBySlug("execute")
	require.True(t, ok)
	assert.Equal(t, AgentLoopInjectionPointExecute, p.Point)
	assert.Equal(t, "c", p.FigureAnnotation)
	assert.Equal(t, 3, p.LoopPhaseOrder)
}

func TestFEAT030_FindInjectionPointBySlug_Unknown(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	_, ok := r.FindInjectionPointBySlug("unknown_phase")
	assert.False(t, ok)
}

func TestFEAT030_AllInjectionPointsInPhaseOrder(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	all := r.AllInjectionPoints()
	require.Len(t, all, 3)
	assert.Equal(t, AgentLoopInjectionPointAssemble, all[0].Point)
	assert.Equal(t, AgentLoopInjectionPointModel, all[1].Point)
	assert.Equal(t, AgentLoopInjectionPointExecute, all[2].Point)
}

func TestFEAT030_TotalElementCount(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	all := r.AllElements()
	assert.Len(t, all, SeedAgentLoopInjectionElementCount,
		"Figure 5 tables enumerate 16 named elements across the 3 injection points")
}

func TestFEAT030_AssemblePhaseElementCount(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	els := r.ElementsAt(AgentLoopInjectionPointAssemble)
	// Figure 5 table (a): CLAUDE.md, Skill descriptions, MCP resources & prompts,
	// Output style, UserPromptSubmit hook, SessionStart hook = 6 elements
	assert.Len(t, els, 6)
}

func TestFEAT030_ModelPhaseElementCount(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	els := r.ElementsAt(AgentLoopInjectionPointModel)
	// Figure 5 table (b): Built-in tools, MCP tools, SkillTool, AgentTool = 4 elements
	assert.Len(t, els, 4)
}

func TestFEAT030_ExecutePhaseElementCount(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	els := r.ElementsAt(AgentLoopInjectionPointExecute)
	// Figure 5 table (c): Permission rules, PreToolUse, PostToolUse, Stop,
	// SubagentStop, Notification = 6 elements
	assert.Len(t, els, 6)
}

func TestFEAT030_HookElementsTotal(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	hooks := r.HookElements()
	// assemble: UserPromptSubmit, SessionStart (2)
	// execute:  PreToolUse, PostToolUse, Stop, SubagentStop, Notification (5)
	// = 7 hook elements total
	assert.Len(t, hooks, 7)
}

func TestFEAT030_HookElementsAcrossPhases(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	hooks := r.HookElements()
	assembleHooks, executeHooks := 0, 0
	for _, h := range hooks {
		switch h.InjectionPoint {
		case AgentLoopInjectionPointAssemble:
			assembleHooks++
		case AgentLoopInjectionPointExecute:
			executeHooks++
		}
	}
	assert.Equal(t, 2, assembleHooks, "UserPromptSubmit + SessionStart at assemble")
	assert.Equal(t, 5, executeHooks, "5 hook types at execute")
}

func TestFEAT030_MCPElementsContributeToMultiplePhases(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	mcpEls := r.ElementsByMechanism("mcp")
	// assemble: MCP resources & prompts; model: MCP tools — must span both
	require.GreaterOrEqual(t, len(mcpEls), 2)
	phaseSet := map[AgentLoopInjectionPoint]bool{}
	for _, el := range mcpEls {
		phaseSet[el.InjectionPoint] = true
	}
	assert.True(t, phaseSet[AgentLoopInjectionPointAssemble])
	assert.True(t, phaseSet[AgentLoopInjectionPointModel])
}

func TestFEAT030_ZeroCostElementsAreAllHooksOrPermRules(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	zeroEls := r.ElementsByContextCost("zero")
	require.NotEmpty(t, zeroEls)
	for _, el := range zeroEls {
		isHookOrPerm := el.IsHook || el.Name == "Permission rules"
		assert.True(t, isHookOrPerm,
			"zero-cost element %q must be a hook or permission rules (§6.3)", el.Name)
	}
}

func TestFEAT030_LowestCostInjectionPointIsExecute(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	assert.Equal(t, AgentLoopInjectionPointExecute, r.LowestCostInjectionPoint(),
		"§6.3 Table 2: hooks at execute() have zero context cost by default")
}

func TestFEAT030_ExecutePhaseTypicalContextCostIsZero(t *testing.T) {
	p, ok := agentLoopInjectionPointProfiles[AgentLoopInjectionPointExecute]
	require.True(t, ok)
	assert.Equal(t, "zero", p.TypicalContextCost)
}

func TestFEAT030_ModelPhaseTypicalContextCostIsHigh(t *testing.T) {
	p, ok := agentLoopInjectionPointProfiles[AgentLoopInjectionPointModel]
	require.True(t, ok)
	assert.Equal(t, "high", p.TypicalContextCost,
		"MCP tool schemas at model() have high context cost (§6.3 Table 2)")
}

func TestFEAT030_PhaseOrderIsAscendingInvariant(t *testing.T) {
	assert.True(t, AgentLoopInjectionPointPhaseOrderIsAscending(),
		"Phase orders must be strictly increasing: 1→2→3")
}

func TestFEAT030_FigureAnnotationsAreCanonical(t *testing.T) {
	assert.True(t, AgentLoopFigureAnnotationsAreCanonical(),
		"Figure 5 annotation letters must be a, b, c in order")
}

func TestFEAT030_IsValidInjectionPointSlug_Valid(t *testing.T) {
	for _, slug := range SeedAgentLoopInjectionPointSlugs {
		assert.True(t, IsValidInjectionPointSlug(slug), "slug %q should be valid", slug)
	}
}

func TestFEAT030_IsValidInjectionPointSlug_Invalid(t *testing.T) {
	for _, bad := range []string{"", "run", "dispatch", "ASSEMBLE"} {
		assert.False(t, IsValidInjectionPointSlug(bad), "slug %q should be invalid", bad)
	}
}

func TestFEAT030_AllElementsIsDefensiveCopy(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	a := r.AllElements()
	b := r.AllElements()
	require.Equal(t, a, b)
	if len(a) > 0 {
		a[0].Name = "mutated"
		fresh := r.AllElements()
		assert.NotEqual(t, "mutated", fresh[0].Name,
			"AllElements should return a defensive copy")
	}
}

func TestFEAT030_BuiltinMechanismPresentAtAllPhases(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	builtins := r.ElementsByMechanism("builtin")
	phaseSet := map[AgentLoopInjectionPoint]bool{}
	for _, el := range builtins {
		phaseSet[el.InjectionPoint] = true
	}
	assert.True(t, phaseSet[AgentLoopInjectionPointAssemble],
		"builtin CLAUDE.md files contribute at assemble")
	assert.True(t, phaseSet[AgentLoopInjectionPointModel],
		"built-in tools + AgentTool contribute at model")
	assert.True(t, phaseSet[AgentLoopInjectionPointExecute],
		"permission rules contribute at execute")
}

func TestFEAT030_ControlsSummaryNonEmpty(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	for _, p := range r.AllInjectionPoints() {
		assert.NotEmpty(t, p.ControlsSummary,
			"injection point %q must have a controls summary", p.Point)
	}
}

func TestFEAT030_ElementNamesNonEmpty(t *testing.T) {
	r := NewAgentLoopInjectionPointRegistry()
	for _, el := range r.AllElements() {
		assert.NotEmpty(t, el.Name)
		assert.NotEmpty(t, el.WhatItDoes)
	}
}
