package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClaudeMDMemoryType_Constants verifies the four type constant strings match §7.2.
func TestClaudeMDMemoryType_Constants(t *testing.T) {
	assert.Equal(t, ClaudeMDMemoryType("managed"), ClaudeMDMemoryTypeManaged)
	assert.Equal(t, ClaudeMDMemoryType("user"), ClaudeMDMemoryTypeUser)
	assert.Equal(t, ClaudeMDMemoryType("project"), ClaudeMDMemoryTypeProject)
	assert.Equal(t, ClaudeMDMemoryType("local"), ClaudeMDMemoryTypeLocal)
}

// TestClaudeMDMemoryType_LoadOrderAscending validates the structural invariant
// that load orders are 1,2,3,4 in the canonical slice.
func TestClaudeMDMemoryType_LoadOrderAscending(t *testing.T) {
	assert.True(t, ClaudeMDLoadOrderIsAscending(), "load orders must be 1..4 in order")
}

// TestClaudeMDMemoryType_ProfileKnown verifies Profile() returns true for all four types.
func TestClaudeMDMemoryType_ProfileKnown(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	types := []ClaudeMDMemoryType{
		ClaudeMDMemoryTypeManaged,
		ClaudeMDMemoryTypeUser,
		ClaudeMDMemoryTypeProject,
		ClaudeMDMemoryTypeLocal,
	}
	for _, mt := range types {
		p, ok := reg.Profile(mt)
		require.True(t, ok, "Profile(%q) must be known", mt)
		assert.Equal(t, mt, p.Type)
	}
}

// TestClaudeMDMemoryType_ProfileUnknown verifies Profile() returns false for unknown types.
func TestClaudeMDMemoryType_ProfileUnknown(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	_, ok := reg.Profile(ClaudeMDMemoryType("nonexistent"))
	assert.False(t, ok)
}

// TestClaudeMDMemoryType_AllTypes verifies AllTypes returns exactly four entries in load order.
func TestClaudeMDMemoryType_AllTypes(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	all := reg.AllTypes()
	require.Len(t, all, 4)
	assert.Equal(t, ClaudeMDMemoryTypeManaged, all[0])
	assert.Equal(t, ClaudeMDMemoryTypeUser, all[1])
	assert.Equal(t, ClaudeMDMemoryTypeProject, all[2])
	assert.Equal(t, ClaudeMDMemoryTypeLocal, all[3])
}

// TestClaudeMDMemoryType_AllTypesIsDefensiveCopy verifies mutations do not affect the registry.
func TestClaudeMDMemoryType_AllTypesIsDefensiveCopy(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	a := reg.AllTypes()
	a[0] = ClaudeMDMemoryType("corrupted")
	b := reg.AllTypes()
	assert.Equal(t, ClaudeMDMemoryTypeManaged, b[0], "AllTypes must return defensive copies")
}

// TestClaudeMDMemoryType_EffectivePriorityOrder verifies priority order is reversed from load order.
func TestClaudeMDMemoryType_EffectivePriorityOrder(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	prio := reg.EffectivePriorityOrder()
	require.Len(t, prio, 4)
	// Local has highest priority (index 0 in priority order).
	assert.Equal(t, ClaudeMDMemoryTypeLocal, prio[0])
	assert.Equal(t, ClaudeMDMemoryTypeProject, prio[1])
	assert.Equal(t, ClaudeMDMemoryTypeUser, prio[2])
	assert.Equal(t, ClaudeMDMemoryTypeManaged, prio[3])
}

// TestClaudeMDMemoryType_HighestPriority verifies local memory wins.
func TestClaudeMDMemoryType_HighestPriority(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	assert.Equal(t, ClaudeMDMemoryTypeLocal, reg.HighestPriority())
}

// TestClaudeMDMemoryType_LowestPriority verifies managed memory has least attention.
func TestClaudeMDMemoryType_LowestPriority(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	assert.Equal(t, ClaudeMDMemoryTypeManaged, reg.LowestPriority())
}

// TestClaudeMDMemoryType_GitIgnoredTypes verifies only local memory is gitignored.
func TestClaudeMDMemoryType_GitIgnoredTypes(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	gitIgnored := reg.GitIgnoredTypes()
	require.Len(t, gitIgnored, 1)
	assert.Equal(t, ClaudeMDMemoryTypeLocal, gitIgnored[0])
}

// TestClaudeMDMemoryType_CheckedInTypes verifies only project memory is checked in.
func TestClaudeMDMemoryType_CheckedInTypes(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	checkedIn := reg.CheckedInTypes()
	require.Len(t, checkedIn, 1)
	assert.Equal(t, ClaudeMDMemoryTypeProject, checkedIn[0])
}

// TestClaudeMDMemoryType_StrategyStatic verifies managed and user use static strategy.
func TestClaudeMDMemoryType_StrategyStatic(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	static := reg.TypesWithStrategy("static")
	require.Len(t, static, 2)
	assert.Equal(t, ClaudeMDMemoryTypeManaged, static[0])
	assert.Equal(t, ClaudeMDMemoryTypeUser, static[1])
}

// TestClaudeMDMemoryType_StrategyDiscovery verifies project and local use discovery strategy.
func TestClaudeMDMemoryType_StrategyDiscovery(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	discovery := reg.TypesWithStrategy("discovery")
	require.Len(t, discovery, 2)
	assert.Equal(t, ClaudeMDMemoryTypeProject, discovery[0])
	assert.Equal(t, ClaudeMDMemoryTypeLocal, discovery[1])
}

// TestClaudeMDMemoryType_IsValidClaudeMDMemoryType validates known and unknown types.
func TestClaudeMDMemoryType_IsValidClaudeMDMemoryType(t *testing.T) {
	assert.True(t, IsValidClaudeMDMemoryType(ClaudeMDMemoryTypeManaged))
	assert.True(t, IsValidClaudeMDMemoryType(ClaudeMDMemoryTypeUser))
	assert.True(t, IsValidClaudeMDMemoryType(ClaudeMDMemoryTypeProject))
	assert.True(t, IsValidClaudeMDMemoryType(ClaudeMDMemoryTypeLocal))
	assert.False(t, IsValidClaudeMDMemoryType(ClaudeMDMemoryType("")))
	assert.False(t, IsValidClaudeMDMemoryType(ClaudeMDMemoryType("admin")))
}

// TestClaudeMDMemoryType_SystemScoped verifies only managed is system-scoped.
func TestClaudeMDMemoryType_SystemScoped(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	all := reg.AllTypes()
	for _, mt := range all {
		p, _ := reg.Profile(mt)
		if mt == ClaudeMDMemoryTypeManaged {
			assert.True(t, p.IsSystemScoped, "managed must be system-scoped")
		} else {
			assert.False(t, p.IsSystemScoped, "%q must not be system-scoped", mt)
		}
	}
}

// TestClaudeMDMemoryType_UserScoped verifies user and local are user-scoped.
func TestClaudeMDMemoryType_UserScoped(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	userScoped := map[ClaudeMDMemoryType]bool{}
	for _, mt := range reg.AllTypes() {
		p, _ := reg.Profile(mt)
		userScoped[mt] = p.IsUserScoped
	}
	assert.True(t, userScoped[ClaudeMDMemoryTypeUser])
	assert.True(t, userScoped[ClaudeMDMemoryTypeLocal])
	assert.False(t, userScoped[ClaudeMDMemoryTypeManaged])
	assert.False(t, userScoped[ClaudeMDMemoryTypeProject])
}

// TestClaudeMDMemoryType_ExamplePathsNonEmpty verifies all types have at least one example path.
func TestClaudeMDMemoryType_ExamplePathsNonEmpty(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	for _, mt := range reg.AllTypes() {
		p, _ := reg.Profile(mt)
		assert.NotEmpty(t, p.ExamplePaths, "type %q must have example paths", mt)
	}
}

// TestClaudeMDMemoryType_ManagedExamplePath verifies managed has the Linux policy path.
func TestClaudeMDMemoryType_ManagedExamplePath(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	p, _ := reg.Profile(ClaudeMDMemoryTypeManaged)
	assert.Contains(t, p.ExamplePaths, "/etc/claude-code/CLAUDE.md")
}

// TestClaudeMDMemoryType_UserExamplePath verifies user has the home-dir path.
func TestClaudeMDMemoryType_UserExamplePath(t *testing.T) {
	reg := NewClaudeMDMemoryTypeRegistry()
	p, _ := reg.Profile(ClaudeMDMemoryTypeUser)
	assert.Contains(t, p.ExamplePaths, "~/.claude/CLAUDE.md")
}
