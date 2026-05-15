package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtensionMechanism_FourMechanisms(t *testing.T) {
	assert.Equal(t, 4, len(ExtensionMechanismSequence))
}

func TestExtensionMechanism_SequenceIsOrderedByCost(t *testing.T) {
	assert.True(t, IsExtensionMechanismSequencedByCost())
}

func TestExtensionMechanismProfileRegistry_Profile_Hooks(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()
	p, ok := reg.Profile(ExtensionMechanismHooks)
	assert.True(t, ok)
	assert.Equal(t, ExtensionContextCostMicro, p.ContextCostCategory)
	assert.Equal(t, ExtensionInsertionExecute, p.InsertionPoint)
	assert.True(t, p.IsZeroCostByDefault, "hooks are zero context cost by default (Table 2)")
	assert.False(t, p.CoversAllInsertPoints)
}

func TestExtensionMechanismProfileRegistry_Profile_Skills(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()
	p, ok := reg.Profile(ExtensionMechanismSkills)
	assert.True(t, ok)
	assert.Equal(t, ExtensionContextCostSmall, p.ContextCostCategory)
	assert.Equal(t, ExtensionInsertionAssemble, p.InsertionPoint)
	assert.False(t, p.IsZeroCostByDefault)
}

func TestExtensionMechanismProfileRegistry_Profile_Plugins(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()
	p, ok := reg.Profile(ExtensionMechanismPlugins)
	assert.True(t, ok)
	assert.Equal(t, ExtensionContextCostMedium, p.ContextCostCategory)
	assert.Equal(t, ExtensionInsertionAll, p.InsertionPoint)
	assert.True(t, p.CoversAllInsertPoints, "plugins fire at all three loop injection points (Table 2)")
}

func TestExtensionMechanismProfileRegistry_Profile_MCPServers(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()
	p, ok := reg.Profile(ExtensionMechanismMCPServers)
	assert.True(t, ok)
	assert.Equal(t, ExtensionContextCostLarge, p.ContextCostCategory)
	assert.Equal(t, ExtensionInsertionModel, p.InsertionPoint)
	assert.False(t, p.CoversAllInsertPoints)
}

func TestExtensionMechanismProfileRegistry_Profile_UnknownReturnsFalse(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()
	_, ok := reg.Profile("unknown")
	assert.False(t, ok)
}

func TestExtensionMechanismProfileRegistry_AllMechanisms_IsDefensiveCopy(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()
	all := reg.AllMechanisms()
	all[0] = "mutated"
	assert.Equal(t, ExtensionMechanismHooks, reg.AllMechanisms()[0])
}

func TestExtensionMechanismProfileRegistry_AllMechanisms_HooksIsFirst(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()
	assert.Equal(t, ExtensionMechanismHooks, reg.AllMechanisms()[0])
}

func TestExtensionMechanismProfileRegistry_AllMechanisms_MCPIsLast(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()
	all := reg.AllMechanisms()
	assert.Equal(t, ExtensionMechanismMCPServers, all[len(all)-1])
}

func TestExtensionMechanismProfileRegistry_ZeroCostMechanisms_OnlyHooks(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()
	zc := reg.ZeroCostMechanisms()
	assert.Equal(t, 1, len(zc))
	assert.Equal(t, ExtensionMechanismHooks, zc[0])
}

func TestExtensionMechanismProfileRegistry_MechanismsAtInsertionPoint_Assemble(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()
	// skills + plugins(all)
	ms := reg.MechanismsAtInsertionPoint(ExtensionInsertionAssemble)
	assert.Equal(t, 2, len(ms))
	assert.Contains(t, ms, ExtensionMechanismSkills)
	assert.Contains(t, ms, ExtensionMechanismPlugins)
}

func TestExtensionMechanismProfileRegistry_MechanismsAtInsertionPoint_Model(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()
	// mcp_servers + plugins(all)
	ms := reg.MechanismsAtInsertionPoint(ExtensionInsertionModel)
	assert.Equal(t, 2, len(ms))
	assert.Contains(t, ms, ExtensionMechanismMCPServers)
	assert.Contains(t, ms, ExtensionMechanismPlugins)
}

func TestExtensionMechanismProfileRegistry_MechanismsAtInsertionPoint_Execute(t *testing.T) {
	reg := NewExtensionMechanismProfileRegistry()
	// hooks + plugins(all)
	ms := reg.MechanismsAtInsertionPoint(ExtensionInsertionExecute)
	assert.Equal(t, 2, len(ms))
	assert.Contains(t, ms, ExtensionMechanismHooks)
	assert.Contains(t, ms, ExtensionMechanismPlugins)
}
