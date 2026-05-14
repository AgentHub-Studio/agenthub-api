package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreSkillSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsBaselineSkills", func(t *testing.T) {
		// Given a fresh tenant browses the skill picker,
		// When ah_core skills are loaded,
		// Then ≥6 baseline platform-mgmt skills appear so an agent
		//      can manage entities (agents/skills/tools/kb/mcp/exec/settings)
		//      from day one without configuration.
		assert.GreaterOrEqual(t, len(SeedExpectedSkillSlugs), 6,
			"fresh tenant must inherit at least 6 baseline platform skills")
	})

	t.Run("Scenario_AllSkillsUseCorePrefixForNamespaceIsolation", func(t *testing.T) {
		// Given tenants can register CUSTOM skills alongside platform
		//       defaults — namespace collision is a real risk,
		// When the seed catalog is inspected,
		// Then every slug starts with "core-" so a tenant skill named
		//      "skills-management" does NOT collide with platform.
		for _, s := range SeedExpectedSkillSlugs {
			assert.True(t, strings.HasPrefix(s, "core-"),
				"slug %q must use core- prefix", s)
		}
	})

	t.Run("Scenario_SkillsAreOrganizedAroundManagementSurfaces", func(t *testing.T) {
		// Given the platform exposes 7 first-class entities (PDF
		//       Section 6.1 — agents/skills/tools/KB/mcp/executions/settings),
		// When the seed is inspected,
		// Then one skill per management surface exists.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedSkillSlugs {
			seedSet[s] = true
		}
		expected := []string{
			"core-agents-management",
			"core-skills-management",
			"core-tools-management",
			"core-kb-management",
			"core-execution-management",
			"core-mcp-management",
			"core-platform-settings",
		}
		for _, s := range expected {
			assert.True(t, seedSet[s],
				"management surface %q must have a dedicated skill", s)
		}
	})

	t.Run("Scenario_SkillsAreInPlatformCategoryForUIFiltering", func(t *testing.T) {
		// Given the UI picker filters skills by category,
		// When the seed contract is inspected,
		// Then all seeded skills are in 'platform' category — distinct
		//      from tenant-custom skills (which use 'general' or custom).
		assert.Equal(t, "platform", SeedExpectedSkillCategory,
			"seed = platform — separates platform-mgmt from tenant-custom")
	})

	t.Run("Scenario_SkillBindingsConnectToToolCatalog", func(t *testing.T) {
		// Given skills are abstract — they need tools to actually do
		//       work (PDF Section 6.1 — skill = capability + tool list),
		// When the seed binding count is inspected,
		// Then ≥20 bindings exist (covers ~3 tools per skill on average).
		assert.GreaterOrEqual(t, SeedExpectedSkillBindingsCount, 20,
			"baseline must wire skills to many tools — abstract skills = no use")
	})

	t.Run("Scenario_InlineContextModeKeepsInstructionsCheap", func(t *testing.T) {
		// Given context mode 'inline' = instructions in system prompt
		//       (lowest latency, highest token cost) vs 'reference'
		//       (lazy load, more tokens but loaded only when invoked),
		// When the seed default is inspected,
		// Then inline is chosen — these are PLATFORM-MGMT skills used
		//      frequently; loading lazily would add latency on every call.
		assert.Equal(t, "inline", SeedExpectedSkillContextMode,
			"platform skills use inline (frequent invocation, low-latency contract)")
	})

	t.Run("Scenario_SkillCountIsStableAcrossRefactors", func(t *testing.T) {
		// Given external systems bind to the canonical count,
		// When the count is inspected,
		// Then 7 skills exist (changes iff migration changed too).
		assert.Equal(t, 7, len(SeedExpectedSkillSlugs),
			"refactor guard: 7 skills expected")
	})

	t.Run("Scenario_NoSkillIsADuplicateNorEmpty", func(t *testing.T) {
		// Given duplicates would break the FindBySlug contract,
		// When the slug set is inspected,
		// Then all slugs are unique and non-empty.
		seen := map[string]bool{}
		for _, s := range SeedExpectedSkillSlugs {
			assert.NotEmpty(t, s, "slug must not be empty")
			assert.False(t, seen[s], "slug %q is duplicated", s)
			seen[s] = true
		}
	})
}
