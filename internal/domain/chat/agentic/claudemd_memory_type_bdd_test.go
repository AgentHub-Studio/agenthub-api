package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD scenarios for §7.2 CLAUDE.md Memory Type Hierarchy.
// Each test follows Given / When / Then structure.

// TestBDD_ClaudeMDMemoryType_LoadOrderProducesCorrectPriority models the core
// §7.2 principle: later-loaded files receive more model attention.
//
// Given: a registry with four memory types in load order
// When:  effective priority order is requested
// Then:  local memory has the highest priority (appears first)
//        and managed memory has the lowest priority (appears last)
func TestBDD_ClaudeMDMemoryType_LoadOrderProducesCorrectPriority(t *testing.T) {
	// Given
	reg := NewClaudeMDMemoryTypeRegistry()

	// When
	prio := reg.EffectivePriorityOrder()

	// Then
	require.NotEmpty(t, prio, "priority order must not be empty")
	assert.Equal(t, ClaudeMDMemoryTypeLocal, prio[0],
		"local memory (loaded last) must have highest priority")
	assert.Equal(t, ClaudeMDMemoryTypeManaged, prio[len(prio)-1],
		"managed memory (loaded first) must have lowest priority")
}

// TestBDD_ClaudeMDMemoryType_GitIgnoredTypeIsPrivate models the §7.2 design
// intent for local memory: private instructions never committed to the repo.
//
// Given: a developer wants private project-specific overrides
// When:  the local memory type profile is inspected
// Then:  it is gitignored, not checked in, and user-scoped
func TestBDD_ClaudeMDMemoryType_GitIgnoredTypeIsPrivate(t *testing.T) {
	// Given
	reg := NewClaudeMDMemoryTypeRegistry()

	// When
	p, ok := reg.Profile(ClaudeMDMemoryTypeLocal)

	// Then
	require.True(t, ok, "local memory type must be known")
	assert.True(t, p.IsGitIgnored, "local memory must be gitignored")
	assert.False(t, p.IsCheckedIn, "local memory must not be checked in")
	assert.True(t, p.IsUserScoped, "local memory must be user-scoped")
}

// TestBDD_ClaudeMDMemoryType_ProjectMemoryIsVersionControlled models the §7.2
// expectation that team-shared instructions live in the repository.
//
// Given: a team wants shared agent instructions committed to version control
// When:  the project memory type profile is inspected
// Then:  it is checked in, not gitignored, and uses discovery load strategy
func TestBDD_ClaudeMDMemoryType_ProjectMemoryIsVersionControlled(t *testing.T) {
	// Given
	reg := NewClaudeMDMemoryTypeRegistry()

	// When
	p, ok := reg.Profile(ClaudeMDMemoryTypeProject)

	// Then
	require.True(t, ok, "project memory type must be known")
	assert.True(t, p.IsCheckedIn, "project memory must be checked in")
	assert.False(t, p.IsGitIgnored, "project memory must not be gitignored")
	assert.Equal(t, "discovery", p.LoadStrategy,
		"project memory uses directory-walk discovery")
}

// TestBDD_ClaudeMDMemoryType_ManagedMemoryIsOSAdministered models the §7.2
// use-case for enterprise/fleet management where admins set OS-level policy.
//
// Given: an enterprise administrator provisions system-wide Claude policies
// When:  the managed memory type profile is inspected
// Then:  it is system-scoped, not user-scoped, uses static strategy,
//        and its example path starts from /etc
func TestBDD_ClaudeMDMemoryType_ManagedMemoryIsOSAdministered(t *testing.T) {
	// Given
	reg := NewClaudeMDMemoryTypeRegistry()

	// When
	p, ok := reg.Profile(ClaudeMDMemoryTypeManaged)

	// Then
	require.True(t, ok, "managed memory type must be known")
	assert.True(t, p.IsSystemScoped, "managed memory must be system-scoped")
	assert.False(t, p.IsUserScoped, "managed memory must not be user-scoped")
	assert.Equal(t, "static", p.LoadStrategy,
		"managed memory uses a fixed, static path")
	require.NotEmpty(t, p.ExamplePaths)
	assert.Contains(t, p.ExamplePaths[0], "/etc",
		"managed memory path must be under /etc on Linux")
}

// TestBDD_ClaudeMDMemoryType_DiscoveryTypesScaleWithProjectDepth models the
// §7.2 principle that files nearer the CWD load later and win higher priority,
// enabling per-subdirectory overrides.
//
// Given: a monorepo with CLAUDE.md files at multiple directory levels
// When:  types with discovery strategy are identified
// Then:  both project and local types use discovery
//        and local has higher load order than project (appears later)
func TestBDD_ClaudeMDMemoryType_DiscoveryTypesScaleWithProjectDepth(t *testing.T) {
	// Given
	reg := NewClaudeMDMemoryTypeRegistry()

	// When
	discoveryTypes := reg.TypesWithStrategy("discovery")
	projectProfile, _ := reg.Profile(ClaudeMDMemoryTypeProject)
	localProfile, _ := reg.Profile(ClaudeMDMemoryTypeLocal)

	// Then
	require.Len(t, discoveryTypes, 2, "exactly two types use discovery strategy")
	assert.Greater(t, localProfile.LoadOrder, projectProfile.LoadOrder,
		"local must load after project so it wins model attention in the same directory")
}
