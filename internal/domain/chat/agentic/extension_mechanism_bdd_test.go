package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for the Table 2 extension mechanism taxonomy.
// Table 2: "What each extension mechanism uniquely provides."

func TestBDD_ExtensionMechanismTable2(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()

	t.Run("Scenario_FourMechanismsWithGraduatedContextCost", func(t *testing.T) {
		// Given Table 2 defines exactly four extension mechanisms
		// When mechanisms are listed in context-cost order
		// Then hooks < skills < plugins < mcp_servers and IsSequencedByCost = true
		assert.Equal(t, 4, len(reg.AllMechanisms()))
		assert.True(t, IsExtensionMechanismSequencedByCost())
		all := reg.AllMechanisms()
		assert.Equal(t, ExtensionMechanismHooks, all[0])
		assert.Equal(t, ExtensionMechanismMCPServers, all[3])
	})

	t.Run("Scenario_HooksAreZeroCostAndFireAtExecute", func(t *testing.T) {
		// Given hooks provide "zero context cost by default" per Table 2
		// When the hooks profile is inspected
		// Then IsZeroCostByDefault=true and InsertionPoint=execute
		p, ok := reg.Profile(ExtensionMechanismHooks)
		assert.True(t, ok)
		assert.True(t, p.IsZeroCostByDefault)
		assert.Equal(t, ExtensionInsertionExecute, p.InsertionPoint)
	})

	t.Run("Scenario_PluginsCoverAllThreeInsertionPoints", func(t *testing.T) {
		// Given plugins serve as a distribution layer across all three loop phases
		// When the plugins profile is inspected
		// Then CoversAllInsertPoints=true and InsertionPoint=all
		p, ok := reg.Profile(ExtensionMechanismPlugins)
		assert.True(t, ok)
		assert.True(t, p.CoversAllInsertPoints)
		assert.Equal(t, ExtensionInsertionAll, p.InsertionPoint)
	})

	t.Run("Scenario_MCPServersHaveHighestContextCostDueToToolSchemas", func(t *testing.T) {
		// Given MCP servers add full tool schemas to the model's context (Table 2: "High (tool schemas)")
		// When MCP server profile is inspected
		// Then ContextCostCategory=large and InsertionPoint=model
		p, ok := reg.Profile(ExtensionMechanismMCPServers)
		assert.True(t, ok)
		assert.Equal(t, ExtensionContextCostLarge, p.ContextCostCategory)
		assert.Equal(t, ExtensionInsertionModel, p.InsertionPoint)
	})

	t.Run("Scenario_PluginsAppearAtEveryInsertionPointQuery", func(t *testing.T) {
		// Given plugins bundle any component type and therefore touch all three phases
		// When querying mechanisms at each insertion point
		// Then plugins appear in all three results
		for _, ip := range []ExtensionInsertionPoint{ExtensionInsertionAssemble, ExtensionInsertionModel, ExtensionInsertionExecute} {
			ms := reg.MechanismsAtInsertionPoint(ip)
			assert.Contains(t, ms, ExtensionMechanismPlugins, "plugins must appear at %s", ip)
		}
	})
}
