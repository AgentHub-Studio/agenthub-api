package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreAgentSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInherits12Specialists", func(t *testing.T) {
		// Given a fresh tenant browses the agent catalog,
		// When ah_core agents are loaded,
		// Then 12 specialist agents appear so the tenant has
		//      ready-made specialists for every major workflow
		//      (assistant, builders for agent/tool/KB/MCP/API,
		//      specialists for skills/tools/KB/agents/pipeline/exec).
		assert.Equal(t, 12, len(SeedExpectedAgentSlugs),
			"12 specialists per spec §21.4")
	})

	t.Run("Scenario_AssistantIsTheGeneralPurposeEntryPoint", func(t *testing.T) {
		// Given users who don't know which specialist they need,
		// When the catalog is inspected,
		// Then exactly ONE ASSISTANT-type agent exists (core-assistant)
		//      as the fallback / entry point. Others are SPECIALIST.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedAgentSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet[SeedAssistantSlug])
		assert.Equal(t, "core-assistant", SeedAssistantSlug,
			"single assistant slug is core-assistant — UI default")
	})

	t.Run("Scenario_AllSpecialistsUseCorePrefixForNamespaceIsolation", func(t *testing.T) {
		// Given tenants register CUSTOM agents alongside platform,
		// When the seed is inspected,
		// Then every slug uses "core-" prefix.
		for _, s := range SeedExpectedAgentSlugs {
			assert.True(t, strings.HasPrefix(s, "core-"),
				"slug %q must use core- prefix", s)
		}
	})

	t.Run("Scenario_BuildersExistForEveryEntityCategory", func(t *testing.T) {
		// Given users need specialist help to build core entities
		//       (PDF Section 6 — build helpers reduce config friction),
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedAgentSlugs {
			seedSet[s] = true
		}
		// Builders for the 4 most-asked entities:
		assert.True(t, seedSet["core-agent-builder"], "agent builder required")
		assert.True(t, seedSet["core-tool-builder"], "tool builder required")
		assert.True(t, seedSet["core-kb-builder"], "KB builder required")
		assert.True(t, seedSet["core-mcp-configurator"], "MCP configurator required")
		assert.True(t, seedSet["core-api-importer"], "API importer required (auto-generates tools)")
	})

	t.Run("Scenario_SpecialistsExistForEveryReadAndAuditSurface", func(t *testing.T) {
		// Given users need to inspect, audit, debug existing entities,
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedAgentSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["core-skills-specialist"])
		assert.True(t, seedSet["core-tools-specialist"])
		assert.True(t, seedSet["core-kb-specialist"])
		assert.True(t, seedSet["core-agents-specialist"])
		assert.True(t, seedSet["core-execution-specialist"],
			"execution specialist required for debugging runs")
	})

	t.Run("Scenario_DeprecatedPipelineSpecialistIsKeptForLegacyCustomers", func(t *testing.T) {
		// Given pipelines are deprecated since 2026-04-02 (ADR-012)
		//       but legacy tenants still have pipeline rows to inspect,
		// When the catalog is inspected,
		// Then core-pipeline-specialist remains as READ-ONLY entry point.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedAgentSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet[SeedDeprecatedAgentSlug],
			"deprecated pipeline specialist must remain (read-only legacy support)")
	})

	t.Run("Scenario_AssistantBindsAllSkillsForUniversalCapability", func(t *testing.T) {
		// Given core-assistant is the catch-all entry point,
		// When the binding policy is inspected,
		// Then assistant cross-joins ALL ah_core.skills (no slug filter)
		//      — produces N bindings where N = #(skills) = 7.
		assert.Equal(t, 7, SeedExpectedAssistantSkillBindingsCount,
			"assistant gets all 7 skills via cross-join — universal capability contract")
	})

	t.Run("Scenario_AgentTypesAreClosedTwoCategories", func(t *testing.T) {
		// Given UI filters by agent_type (ASSISTANT vs SPECIALIST),
		// When the type set is inspected,
		// Then exactly 2 valid types exist.
		assert.Equal(t, 2, len(SeedExpectedAgentTypes))
		set := map[string]bool{}
		for _, tt := range SeedExpectedAgentTypes {
			set[tt] = true
		}
		assert.True(t, set["ASSISTANT"])
		assert.True(t, set["SPECIALIST"])
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		// Given external systems bind to count = 12,
		assert.Equal(t, 12, len(SeedExpectedAgentSlugs))
	})

	t.Run("Scenario_NoDuplicateOrEmptySlugs", func(t *testing.T) {
		seen := map[string]bool{}
		for _, s := range SeedExpectedAgentSlugs {
			assert.NotEmpty(t, s)
			assert.False(t, seen[s], "duplicate slug %q", s)
			seen[s] = true
		}
	})
}
