package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000090 capability tools + skills adapted from
// Claude Code for AgentHub web.

func TestBDD_CapabilitySkillToolSeed(t *testing.T) {
	t.Run("Scenario_SevenCapabilityToolsAdaptedFromClaudeCode", func(t *testing.T) {
		// Given Claude Code has seven built-in tools adapted for web:
		//   WebSearch, WebFetch, Grep, Read, TodoWrite, TodoRead, Agent
		// When the capability seed constants are inspected
		// Then exactly seven tool slugs exist
		assert.Equal(t, 7, SeedCapabilityToolCount)
		assert.Equal(t, 7, len(SeedCapabilityToolSlugs))
	})

	t.Run("Scenario_ThreeCapabilitySkillsGroupToolsIntoWorkflows", func(t *testing.T) {
		// Given the seven tools are grouped into three coherent workflows
		// When the capability skill constants are inspected
		// Then exactly three skill slugs exist
		assert.Equal(t, 3, SeedCapabilitySkillCount)
		assert.Equal(t, 3, len(SeedCapabilitySkillSlugs))
	})

	t.Run("Scenario_EachCapabilitySkillHasExactlyTwoBoundTools", func(t *testing.T) {
		// Given each capability skill binds exactly two tools (primary + secondary)
		// When per-skill tool slug lists are counted
		// Then each has exactly 2 and the total is 6
		assert.Equal(t, 2, len(SeedWebResearchToolSlugs))
		assert.Equal(t, 2, len(SeedDocAnalysisToolSlugs))
		assert.Equal(t, 2, len(SeedTaskWorkflowToolSlugs))
		assert.Equal(t, 6, SeedCapabilitySkillBindingCount)
	})

	t.Run("Scenario_DocumentSearchIsOnlyNonHTTPTool", func(t *testing.T) {
		// Given core-doc-search uses DOCUMENT_SEARCH type (not HTTP)
		//   to enable semantic vector search over knowledge bases
		// When tool types are listed
		// Then exactly two distinct types exist (HTTP + DOCUMENT_SEARCH)
		assert.Equal(t, 2, len(SeedCapabilityToolTypes))
		assert.Contains(t, SeedCapabilityToolTypes, "DOCUMENT_SEARCH")
		assert.Equal(t, "core-doc-search", SeedDocumentSearchToolSlug)
	})

	t.Run("Scenario_WebResearchCombinesSearchAndFetch", func(t *testing.T) {
		// Given core-web-research needs both search (find) and fetch (read full page)
		// When SeedWebResearchToolSlugs is inspected
		// Then both core-web-search and core-web-fetch are present
		assert.Contains(t, SeedWebResearchToolSlugs, "core-web-search")
		assert.Contains(t, SeedWebResearchToolSlugs, "core-web-fetch")
	})

	t.Run("Scenario_CapabilitySkillCategoryDistinctFromPlatform", func(t *testing.T) {
		// Given capability skills perform user-facing work
		//   while platform skills manage platform resources
		// When categories are compared
		// Then capability category differs from the existing platform category
		assert.NotEqual(t, SeedCapabilitySkillCategory, SeedExpectedSkillCategory)
		assert.Equal(t, "capability", SeedCapabilitySkillCategory)
	})

	t.Run("Scenario_SubagentRunAdaptsClaudeCodeAgentSpawning", func(t *testing.T) {
		// Given Claude Code's Agent tool spawns subagents
		//   and AgentHub web exposes this via POST /api/chat/sessions
		// When the subagent-run tool slug is inspected
		// Then it exists in the capability tool list
		assert.Contains(t, SeedCapabilityToolSlugs, "core-subagent-run")
	})
}
