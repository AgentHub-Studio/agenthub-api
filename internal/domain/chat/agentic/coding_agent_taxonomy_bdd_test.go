package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD scenarios for CodingAgentTaxonomy.
// Maps §13.1 Table 5 (four coding-tool categories) to AgentHub web platform positioning.

func TestBDD_CodingAgentTaxonomy(t *testing.T) {
	t.Run("Scenario_FourCategoriesCoverFullAutonomySpectrum", func(t *testing.T) {
		// Given the §13.1 taxonomy of AI coding tools
		// When the registry is queried for all categories
		// Then exactly 4 categories span the spectrum from passive to autonomous
		r := NewCodingAgentTaxonomyRegistry()
		cats := r.AllCategories()
		assert.Equal(t, 4, len(cats),
			"Table 5 defines exactly 4 coding agent categories")
		// Gradient index must be 0 through 3 in order
		for i, cat := range cats {
			p, _ := r.Profile(cat)
			assert.Equal(t, TaxonomyGradientIndex(i), p.GradientIndex,
				"category at position %d must have gradient index %d", i, i)
		}
	})

	t.Run("Scenario_AgentHubPositionedAsChatIntegrated", func(t *testing.T) {
		// Given AgentHub is a web-first multi-tenant agent platform
		// When the taxonomy is consulted for AgentHub's deployment category
		// Then chat_integrated is the primary match (web chat interface, tool-use, no IDE coupling)
		r := NewCodingAgentTaxonomyRegistry()
		p, found := r.Profile(AgentCategoryChatIntegrated)
		require.True(t, found)
		assert.True(t, p.IsAgentHubTarget)
		agentHubSystems := p.ExampleSystems
		found = false
		for _, ex := range agentHubSystems {
			if ex == "AgentHub" {
				found = true
			}
		}
		assert.True(t, found, "AgentHub must be listed under chat_integrated examples")
	})

	t.Run("Scenario_BackgroundAgentsExtendToAgenticCLI", func(t *testing.T) {
		// Given AgentHub supports autonomous/background agents (KAIROS tier)
		// When those agents run tool-use loops without direct IDE coupling
		// Then agentic_cli is also an AgentHub target category
		r := NewCodingAgentTaxonomyRegistry()
		targets := r.AgentHubTargetCategories()
		require.Equal(t, 2, len(targets))
		assert.Contains(t, targets, AgentCategoryChatIntegrated)
		assert.Contains(t, targets, AgentCategoryAgenticCLI)
		// Verify agentic_cli uses permission_gates isolation (consistent with our capability tiers)
		agenticProfile, _ := r.Profile(AgentCategoryAgenticCLI)
		assert.Equal(t, AgentIsolationPermissionGates, agenticProfile.IsolationModel,
			"agentic_cli isolation matches AgentHub's permission mode gradient")
	})

	t.Run("Scenario_FullyAutonomousRequiresSandboxNotOfferedByDefault", func(t *testing.T) {
		// Given fully_autonomous systems use container sandboxing
		// And AgentHub does not currently provision container sandboxes per agent
		// Then fully_autonomous is NOT an AgentHub target category
		r := NewCodingAgentTaxonomyRegistry()
		p, found := r.Profile(AgentCategoryFullyAutonomous)
		require.True(t, found)
		assert.False(t, p.IsAgentHubTarget,
			"fully_autonomous requires container sandbox not in AgentHub scope")
		assert.Equal(t, AgentIsolationSandbox, p.IsolationModel)
	})

	t.Run("Scenario_AgenticCLIIsMoreAutonomousThanChatIntegrated", func(t *testing.T) {
		// Given the §13.1 autonomy gradient
		// When comparing agentic_cli with chat_integrated
		// Then agentic_cli ranks higher in autonomous action degree
		r := NewCodingAgentTaxonomyRegistry()
		assert.True(t,
			r.IsMoreAutonomousThan(AgentCategoryAgenticCLI, AgentCategoryChatIntegrated),
			"agentic_cli must rank above chat_integrated in the autonomy gradient")
	})
}
