package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD tests for AgentLoopInjectionPointRegistry — FEAT030
// Source: §6.1 / Figure 5 of arXiv:2604.14228v1, page 16.

// TestFEAT030_BDD_AgentLoop_ThreePhasesFormCompleteLoop verifies that the three
// injection points together cover the full agent loop turn as described in Figure 5.
//
//	Given the agent loop pseudocode in Figure 5
//	When I query all injection points
//	Then I receive exactly 3, labeled a/b/c, in phase order assemble→model→execute
func TestFEAT030_BDD_AgentLoop_ThreePhasesFormCompleteLoop(t *testing.T) {
	// Given
	r := NewAgentLoopInjectionPointRegistry()

	// When
	all := r.AllInjectionPoints()

	// Then
	require.Len(t, all, 3, "Figure 5 shows exactly 3 injection points")
	assert.Equal(t, "a", all[0].FigureAnnotation, "first point annotated (a)")
	assert.Equal(t, "b", all[1].FigureAnnotation, "second point annotated (b)")
	assert.Equal(t, "c", all[2].FigureAnnotation, "third point annotated (c)")
	assert.Equal(t, AgentLoopInjectionPointAssemble, all[0].Point)
	assert.Equal(t, AgentLoopInjectionPointModel, all[1].Point)
	assert.Equal(t, AgentLoopInjectionPointExecute, all[2].Point)
}

// TestFEAT030_BDD_AgentLoop_AssemblePhaseControlsWhatModelSees verifies that
// the assemble() phase has the correct Figure 5 table (a) elements.
//
//	Given an agent about to make a model call
//	When the assemble() phase runs
//	Then the context window is shaped by CLAUDE.md, skills, MCP resources,
//	     output style, and the UserPromptSubmit and SessionStart hooks
func TestFEAT030_BDD_AgentLoop_AssemblePhaseControlsWhatModelSees(t *testing.T) {
	// Given
	r := NewAgentLoopInjectionPointRegistry()

	// When
	elements := r.ElementsAt(AgentLoopInjectionPointAssemble)

	// Then
	require.Len(t, elements, 6, "Figure 5 table (a) lists 6 elements")
	names := make(map[string]bool)
	for _, el := range elements {
		names[el.Name] = true
	}
	assert.True(t, names["CLAUDE.md files"], "CLAUDE.md files present at assemble")
	assert.True(t, names["Skill descriptions"], "Skill descriptions present")
	assert.True(t, names["MCP resources & prompts"], "MCP resources & prompts present")
	assert.True(t, names["Output style"], "Output style present")
	assert.True(t, names["UserPromptSubmit hook"], "UserPromptSubmit hook present")
	assert.True(t, names["SessionStart hook"], "SessionStart hook present")
}

// TestFEAT030_BDD_AgentLoop_ModelPhaseControlsToolReachability verifies that
// the model() phase elements match Figure 5 table (b) exactly.
//
//	Given a context window that has been assembled
//	When the model() phase presents the flat tool pool
//	Then the model can reach built-in tools, MCP tools, SkillTool, and AgentTool
func TestFEAT030_BDD_AgentLoop_ModelPhaseControlsToolReachability(t *testing.T) {
	// Given
	r := NewAgentLoopInjectionPointRegistry()

	// When
	elements := r.ElementsAt(AgentLoopInjectionPointModel)

	// Then
	require.Len(t, elements, 4, "Figure 5 table (b) lists 4 elements")
	names := make(map[string]bool)
	for _, el := range elements {
		names[el.Name] = true
	}
	assert.True(t, names["Built-in tools"])
	assert.True(t, names["MCP tools"])
	assert.True(t, names["SkillTool"])
	assert.True(t, names["AgentTool"])
}

// TestFEAT030_BDD_AgentLoop_ExecutePhaseGatesEveryToolCall verifies that
// the execute() phase has exactly the Figure 5 table (c) lifecycle controls.
//
//	Given the model has emitted a tool_call action
//	When the execute() phase processes it
//	Then permission rules and 5 hook types gate and annotate the execution
func TestFEAT030_BDD_AgentLoop_ExecutePhaseGatesEveryToolCall(t *testing.T) {
	// Given
	r := NewAgentLoopInjectionPointRegistry()

	// When
	elements := r.ElementsAt(AgentLoopInjectionPointExecute)

	// Then
	require.Len(t, elements, 6, "Figure 5 table (c) lists 6 elements")
	names := make(map[string]bool)
	for _, el := range elements {
		names[el.Name] = true
	}
	assert.True(t, names["Permission rules"])
	assert.True(t, names["PreToolUse hook"])
	assert.True(t, names["PostToolUse hook"])
	assert.True(t, names["Stop hook"])
	assert.True(t, names["SubagentStop hook"])
	assert.True(t, names["Notification hook"])
}

// TestFEAT030_BDD_AgentLoop_HooksHaveZeroDefaultContextCost verifies the
// §6.3 Table 2 design principle: hooks consume no context tokens by default.
//
//	Given the graduated context-cost ordering from §6.3
//	When I query all hook elements
//	Then every hook has ContextCost == "zero"
func TestFEAT030_BDD_AgentLoop_HooksHaveZeroDefaultContextCost(t *testing.T) {
	// Given / When
	r := NewAgentLoopInjectionPointRegistry()
	hooks := r.HookElements()

	// Then
	require.NotEmpty(t, hooks)
	for _, h := range hooks {
		assert.Equal(t, "zero", h.ContextCost,
			"hook %q must have zero context cost (§6.3)", h.Name)
	}
}

// TestFEAT030_BDD_AgentLoop_MCPServersContributeAtTwoPhases verifies that
// MCP servers participate at both assemble() (resources/prompts) and model() (tools).
//
//	Given the composable multi-mechanism extensibility principle (§6.1)
//	When I query MCP-mechanism elements
//	Then they appear at both the assemble and model injection points
func TestFEAT030_BDD_AgentLoop_MCPServersContributeAtTwoPhases(t *testing.T) {
	// Given / When
	r := NewAgentLoopInjectionPointRegistry()
	mcpEls := r.ElementsByMechanism("mcp")

	// Then
	phaseSet := map[AgentLoopInjectionPoint]bool{}
	for _, el := range mcpEls {
		phaseSet[el.InjectionPoint] = true
	}
	assert.True(t, phaseSet[AgentLoopInjectionPointAssemble],
		"MCP resources & prompts contribute at assemble()")
	assert.True(t, phaseSet[AgentLoopInjectionPointModel],
		"MCP tools contribute at model()")
	assert.False(t, phaseSet[AgentLoopInjectionPointExecute],
		"MCP servers do not contribute at execute()")
}

// TestFEAT030_BDD_AgentLoop_StructuralInvariantsHold validates the two structural
// invariants coded as pure boolean functions: phase-order monotonicity and
// Figure 5 annotation letters.
//
//	Given the Figure 5 diagram and §6.1 caption
//	When structural invariant validators are called
//	Then both return true confirming the seeded data is internally consistent
func TestFEAT030_BDD_AgentLoop_StructuralInvariantsHold(t *testing.T) {
	assert.True(t, AgentLoopInjectionPointPhaseOrderIsAscending(),
		"phase orders 1→2→3 must be strictly increasing")
	assert.True(t, AgentLoopFigureAnnotationsAreCanonical(),
		"Figure 5 annotations must be exactly a, b, c")
}

// TestFEAT030_BDD_AgentLoop_SkillsContributeAtTwoPhases verifies that
// skills participate at both assemble() (descriptions) and model() (SkillTool).
//
//	Given the skill extension mechanism (§6.1)
//	When I query skill-mechanism elements
//	Then they appear at both assemble (descriptions) and model (SkillTool)
func TestFEAT030_BDD_AgentLoop_SkillsContributeAtTwoPhases(t *testing.T) {
	// Given / When
	r := NewAgentLoopInjectionPointRegistry()
	skillEls := r.ElementsByMechanism("skill")

	// Then
	phaseSet := map[AgentLoopInjectionPoint]bool{}
	for _, el := range skillEls {
		phaseSet[el.InjectionPoint] = true
	}
	assert.True(t, phaseSet[AgentLoopInjectionPointAssemble],
		"skill descriptions contribute at assemble()")
	assert.True(t, phaseSet[AgentLoopInjectionPointModel],
		"SkillTool contributes at model()")
}
