package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for AgentRolePresetDefaultTemplate seed.
// Maps Claude Code's built-in subagent types (PDF §8) to AgentHub web roles,
// encoding default context mode, effort level, tool permissions, and recommended
// use cases for each role.

func TestBDD_AhCoreAgentRolePresetSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantReceivesFiveRolePresets", func(t *testing.T) {
		// Given a new tenant bootstraps for the first time
		// When the system loads agent role preset templates
		// Then exactly 5 role presets are available covering the §8 subagent types
		assert.Equal(t, 5, SeedExpectedAgentRolePresetRowCount)
		assert.Equal(t, 5, len(SeedExpectedAgentRolePresetSlugs))
	})

	t.Run("Scenario_GeneralAssistantIsDefaultRole", func(t *testing.T) {
		// Given a new agent with no explicit role configured
		// When the system resolves the default role
		// Then the general-assistant preset is used (maps to general-purpose subagent)
		assert.Equal(t, "general-assistant", SeedAgentRoleDefaultSlug)
		found := false
		for _, s := range SeedExpectedAgentRolePresetSlugs {
			if s == SeedAgentRoleDefaultSlug {
				found = true
			}
		}
		assert.True(t, found, "default slug must be in main catalog")
	})

	t.Run("Scenario_PlannerRoleRepresentsPlanSubagentType", func(t *testing.T) {
		// Given the §8 plan subagent type enables structured plan-mode execution
		// When an agent requires planning capability
		// Then the planner preset is selected (source_subagent_type = plan)
		assert.Equal(t, "planner", SeedAgentRolePlannerSlug)
		found := false
		for _, s := range SeedExpectedAgentRolePresetSlugs {
			if s == SeedAgentRolePlannerSlug {
				found = true
			}
		}
		assert.True(t, found, "planner slug must be in main catalog")
	})

	t.Run("Scenario_ReadResearcherIsReadOnlyRole", func(t *testing.T) {
		// Given a research agent must not modify state (explore subagent type)
		// When the system resolves the read-only research role
		// Then the read-researcher preset enforces read-only permission mode
		assert.Equal(t, "read-researcher", SeedAgentRoleReadOnlySlug)
		found := false
		for _, s := range SeedExpectedAgentRolePresetSlugs {
			if s == SeedAgentRoleReadOnlySlug {
				found = true
			}
		}
		assert.True(t, found, "read-only role must be in main catalog")
	})

	t.Run("Scenario_AllFiveSourceSubagentTypesCovered", func(t *testing.T) {
		// Given §8 defines 5 built-in subagent types
		// When the role preset catalog is inspected
		// Then all 5 source types are represented: general-purpose, explore, plan,
		// verification, claude-code-guide
		assert.Equal(t, 5, len(SeedAgentRoleSourceTypes))
		typeSet := map[string]bool{}
		for _, st := range SeedAgentRoleSourceTypes {
			typeSet[st] = true
		}
		assert.True(t, typeSet["general-purpose"])
		assert.True(t, typeSet["explore"])
		assert.True(t, typeSet["plan"])
		assert.True(t, typeSet["verification"])
		assert.True(t, typeSet["claude-code-guide"])
	})

	t.Run("Scenario_AllSlugsMustBeKebabCase", func(t *testing.T) {
		// Given naming conventions require kebab-case slugs
		// When each role slug is validated against the pattern
		// Then all slugs match ^[a-z0-9][a-z0-9-]*[a-z0-9]$
		for _, s := range SeedExpectedAgentRolePresetSlugs {
			assert.True(t, SeedAgentRolePresetSlugRE.MatchString(s),
				"slug %q violates kebab-case pattern", s)
		}
	})
}
