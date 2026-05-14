package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000091 capability agent templates adapted from
// Claude Code agent types for AgentHub web.

func TestBDD_CapabilityAgentTemplateSeed(t *testing.T) {
	t.Run("Scenario_ThreeCapabilityAgentsAdaptedFromClaudeCodeAgentTypes", func(t *testing.T) {
		// Given Claude Code has three built-in agent types:
		//   general-purpose, Explore, and Plan
		// When the capability agent seed constants are inspected
		// Then exactly three agents exist, each adapted from a Claude Code archetype:
		//   core-researcher  → general-purpose (open-ended research)
		//   core-analyst     → Explore (structured document exploration)
		//   core-planner     → Plan (goal decomposition + task tracking)
		assert.Equal(t, 3, SeedCapabilityAgentCount)
		assert.Equal(t, 3, len(SeedCapabilityAgentSlugs))

		slugSet := map[string]bool{}
		for _, s := range SeedCapabilityAgentSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet["core-researcher"],
			"core-researcher adapts general-purpose Claude Code agent type")
		assert.True(t, slugSet["core-analyst"],
			"core-analyst adapts Explore Claude Code agent type")
		assert.True(t, slugSet["core-planner"],
			"core-planner adapts Plan Claude Code agent type")
	})

	t.Run("Scenario_EachCapabilityAgentBindsExactlyOneSkill", func(t *testing.T) {
		// Given each capability agent is a thin wrapper around a single
		//       cohesive skill from migration 000090
		// When binding constants are inspected
		// Then SeedCapabilityAgentSkillBindingCount == SeedCapabilityAgentCount
		//      (1 binding per agent — no agent accumulates multiple skills)
		assert.Equal(t, SeedCapabilityAgentCount, SeedCapabilityAgentSkillBindingCount,
			"each agent binds exactly 1 skill → binding count = agent count")

		// And each bound skill slug is in the migration 000090 skill catalog.
		m90Skills := map[string]bool{}
		for _, s := range SeedCapabilitySkillSlugs {
			m90Skills[s] = true
		}
		assert.True(t, m90Skills[SeedResearcherSkillSlug],
			"researcher skill %q must be from migration 000090", SeedResearcherSkillSlug)
		assert.True(t, m90Skills[SeedAnalystSkillSlug],
			"analyst skill %q must be from migration 000090", SeedAnalystSkillSlug)
		assert.True(t, m90Skills[SeedPlannerSkillSlug],
			"planner skill %q must be from migration 000090", SeedPlannerSkillSlug)
	})

	t.Run("Scenario_CapabilityAgentsAreAssistantTypeNotSpecialist", func(t *testing.T) {
		// Given specialist agents (e.g. core-kb-builder) are configured
		//       for platform management tasks
		// When capability agents are compared against the type closed set
		// Then all three are ASSISTANT type — interactive, user-facing,
		//      not domain-expert management agents
		assert.Equal(t, "ASSISTANT", SeedCapabilityAgentType,
			"capability agents must be ASSISTANT type (interactive/user-facing)")

		// And the SPECIALIST type is NOT used for capability agents.
		assert.NotEqual(t, "SPECIALIST", SeedCapabilityAgentType,
			"capability agents must NOT be SPECIALIST type")
	})

	t.Run("Scenario_AllCapabilityAgentSlugsUseCorePrefixNamespace", func(t *testing.T) {
		// Given platform agents are namespaced under the "core-" prefix
		//       to prevent collision with tenant-managed agents
		// When all capability agent slugs are inspected
		// Then every slug begins with "core-"
		for _, slug := range SeedCapabilityAgentSlugs {
			assert.True(t, strings.HasPrefix(slug, "core-"),
				"capability agent slug %q must use core- prefix namespace", slug)
		}

		// And no slug is empty or contains invalid URL characters.
		for _, slug := range SeedCapabilityAgentSlugs {
			assert.NotEmpty(t, slug)
			for _, r := range slug {
				ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
				assert.True(t, ok, "slug %q has invalid char %q", slug, r)
			}
		}
	})

	t.Run("Scenario_CapabilitySkillSlugsReferencesMigration000090Skills", func(t *testing.T) {
		// Given migration 000091 agents must bind only skills that already exist
		//       (seeded in migration 000090 — capability skill-tool templates)
		// When the three bound skill slugs are checked against SeedCapabilitySkillSlugs
		// Then all three resolve to known migration 000090 entries
		m90Set := map[string]bool{}
		for _, s := range SeedCapabilitySkillSlugs {
			m90Set[s] = true
		}

		boundSkills := []string{
			SeedResearcherSkillSlug,
			SeedAnalystSkillSlug,
			SeedPlannerSkillSlug,
		}
		for _, sk := range boundSkills {
			assert.True(t, m90Set[sk],
				"bound skill slug %q must be in SeedCapabilitySkillSlugs (migration 000090)", sk)
		}

		// And the three bound skills collectively cover the full SeedCapabilitySkillSlugs set.
		assert.Equal(t, len(SeedCapabilitySkillSlugs), len(boundSkills),
			"all 3 migration 000090 skills must each be bound to exactly 1 capability agent")
	})
}
