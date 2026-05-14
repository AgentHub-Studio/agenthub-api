package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agentic "github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// Unit tests for §5.2 PermissionHandlerPath typed registry (FEAT-019).
// Source: arXiv:2604.14228v1, Section 5.2 "The Authorization Pipeline".

// --- Constants ---

func TestPermissionHandlerPathConstants_FourDistinctValues(t *testing.T) {
	paths := []agentic.PermissionHandlerPath{
		agentic.PermissionHandlerPathCoordinator,
		agentic.PermissionHandlerPathSwarmWorker,
		agentic.PermissionHandlerPathSpeculativeClassifier,
		agentic.PermissionHandlerPathInteractive,
	}
	set := map[agentic.PermissionHandlerPath]bool{}
	for _, p := range paths {
		set[p] = true
	}
	assert.Len(t, set, 4, "§5.2 defines exactly four handler paths")
}

func TestPermissionHandlerPathConstants_NonEmptyTokens(t *testing.T) {
	assert.NotEmpty(t, string(agentic.PermissionHandlerPathCoordinator))
	assert.NotEmpty(t, string(agentic.PermissionHandlerPathSwarmWorker))
	assert.NotEmpty(t, string(agentic.PermissionHandlerPathSpeculativeClassifier))
	assert.NotEmpty(t, string(agentic.PermissionHandlerPathInteractive))
}

// --- Registry construction ---

func TestNewPermissionHandlerPathRegistry_ReturnsNonNil(t *testing.T) {
	r := agentic.NewPermissionHandlerPathRegistry()
	assert.NotNil(t, r)
}

// --- AllPaths ---

func TestPermissionHandlerPathRegistry_AllPaths_ReturnsFour(t *testing.T) {
	r := agentic.NewPermissionHandlerPathRegistry()
	all := r.AllPaths()
	assert.Len(t, all, 4, "registry must expose exactly four paths")
}

func TestPermissionHandlerPathRegistry_AllPaths_IsDefensiveCopy(t *testing.T) {
	r := agentic.NewPermissionHandlerPathRegistry()
	a := r.AllPaths()
	b := r.AllPaths()
	assert.Equal(t, a, b, "both copies must contain the same elements")
	// Mutate a and verify b is unaffected.
	if len(a) > 0 {
		a[0] = "mutated"
		assert.NotEqual(t, a[0], b[0], "AllPaths must return a defensive copy")
	}
}

func TestPermissionHandlerPathRegistry_AllPaths_FirstIsCoordinator(t *testing.T) {
	// §5.2 lists Coordinator as path 1 (most automated).
	r := agentic.NewPermissionHandlerPathRegistry()
	all := r.AllPaths()
	require.NotEmpty(t, all)
	assert.Equal(t, agentic.PermissionHandlerPathCoordinator, all[0],
		"coordinator must be the first (most-automated) path")
}

func TestPermissionHandlerPathRegistry_AllPaths_LastIsInteractive(t *testing.T) {
	// §5.2 lists Interactive as path 4 (the fallback).
	r := agentic.NewPermissionHandlerPathRegistry()
	all := r.AllPaths()
	require.NotEmpty(t, all)
	assert.Equal(t, agentic.PermissionHandlerPathInteractive, all[len(all)-1],
		"interactive must be the last (fallback) path")
}

// --- Profile ---

func TestPermissionHandlerPathRegistry_Profile_CoordinatorProfile(t *testing.T) {
	r := agentic.NewPermissionHandlerPathRegistry()
	p, ok := r.Profile(agentic.PermissionHandlerPathCoordinator)
	require.True(t, ok, "coordinator must have a profile")
	assert.Equal(t, 1, p.AutomationLevel, "coordinator is the most automated path")
	assert.True(t, p.SupportsAutomatedResolution)
	assert.True(t, p.IsBackgroundAgentPath)
	assert.False(t, p.RequiresUserDialog)
}

func TestPermissionHandlerPathRegistry_Profile_InteractiveProfile(t *testing.T) {
	r := agentic.NewPermissionHandlerPathRegistry()
	p, ok := r.Profile(agentic.PermissionHandlerPathInteractive)
	require.True(t, ok)
	assert.Equal(t, 4, p.AutomationLevel, "interactive is the least automated path")
	assert.True(t, p.RequiresUserDialog, "interactive requires the user dialog")
	assert.False(t, p.SupportsAutomatedResolution)
	assert.False(t, p.IsBackgroundAgentPath)
}

func TestPermissionHandlerPathRegistry_Profile_SpeculativeClassifierGatedByFlag(t *testing.T) {
	// §5.2: "When BASH_CLASSIFIER is enabled…"
	r := agentic.NewPermissionHandlerPathRegistry()
	p, ok := r.Profile(agentic.PermissionHandlerPathSpeculativeClassifier)
	require.True(t, ok)
	assert.Equal(t, "BASH_CLASSIFIER", p.FeatureFlag,
		"speculative classifier must be gated by the BASH_CLASSIFIER feature flag")
}

func TestPermissionHandlerPathRegistry_Profile_UnknownPathReturnsFalse(t *testing.T) {
	r := agentic.NewPermissionHandlerPathRegistry()
	_, ok := r.Profile("not_a_real_path")
	assert.False(t, ok, "unknown path must return (zero, false)")
}

// --- AutomatedPaths ---

func TestPermissionHandlerPathRegistry_AutomatedPaths_ThreeAutomated(t *testing.T) {
	// Coordinator, SwarmWorker, SpeculativeClassifier all support automated resolution.
	r := agentic.NewPermissionHandlerPathRegistry()
	automated := r.AutomatedPaths()
	assert.Len(t, automated, 3,
		"exactly three paths support automated resolution per §5.2")
}

func TestPermissionHandlerPathRegistry_AutomatedPaths_DoesNotContainInteractive(t *testing.T) {
	r := agentic.NewPermissionHandlerPathRegistry()
	for _, p := range r.AutomatedPaths() {
		assert.NotEqual(t, agentic.PermissionHandlerPathInteractive, p,
			"interactive path does not support automated resolution")
	}
}

// --- BackgroundAgentPaths ---

func TestPermissionHandlerPathRegistry_BackgroundAgentPaths_CoordinatorAndSwarm(t *testing.T) {
	// §5.2: "coordinator and similar background-agent paths await automated checks
	// before showing the dialog."
	r := agentic.NewPermissionHandlerPathRegistry()
	bg := r.BackgroundAgentPaths()
	assert.Len(t, bg, 2)
	pathSet := map[agentic.PermissionHandlerPath]bool{}
	for _, p := range bg {
		pathSet[p] = true
	}
	assert.True(t, pathSet[agentic.PermissionHandlerPathCoordinator])
	assert.True(t, pathSet[agentic.PermissionHandlerPathSwarmWorker])
}

// --- FallbackPath / MostAutomatedPath ---

func TestPermissionHandlerPathRegistry_FallbackPath_IsInteractive(t *testing.T) {
	r := agentic.NewPermissionHandlerPathRegistry()
	assert.Equal(t, agentic.PermissionHandlerPathInteractive, r.FallbackPath(),
		"§5.2: Interactive is the fallback path")
}

func TestPermissionHandlerPathRegistry_MostAutomatedPath_IsCoordinator(t *testing.T) {
	r := agentic.NewPermissionHandlerPathRegistry()
	assert.Equal(t, agentic.PermissionHandlerPathCoordinator, r.MostAutomatedPath(),
		"§5.2: Coordinator is listed first (most automated)")
}

// --- PathsRequiringFeatureFlag ---

func TestPermissionHandlerPathRegistry_PathsRequiringFeatureFlag_OnlySpeculative(t *testing.T) {
	r := agentic.NewPermissionHandlerPathRegistry()
	flagged := r.PathsRequiringFeatureFlag()
	require.Len(t, flagged, 1)
	assert.Equal(t, agentic.PermissionHandlerPathSpeculativeClassifier, flagged[0])
}

// --- IsValidPermissionHandlerPath ---

func TestIsValidPermissionHandlerPath_ValidPaths(t *testing.T) {
	valid := []agentic.PermissionHandlerPath{
		agentic.PermissionHandlerPathCoordinator,
		agentic.PermissionHandlerPathSwarmWorker,
		agentic.PermissionHandlerPathSpeculativeClassifier,
		agentic.PermissionHandlerPathInteractive,
	}
	for _, p := range valid {
		assert.True(t, agentic.IsValidPermissionHandlerPath(p),
			"known path %q must be valid", p)
	}
}

func TestIsValidPermissionHandlerPath_InvalidPath(t *testing.T) {
	assert.False(t, agentic.IsValidPermissionHandlerPath("unknown"))
}

// --- AutomationOrder structural invariant ---

func TestPermissionHandlerPathAutomationOrderIsAscending(t *testing.T) {
	assert.True(t, agentic.PermissionHandlerPathAutomationOrderIsAscending(),
		"automation levels must be strictly 1..4 in canonical order")
}

// --- SwarmWorker profile completeness ---

func TestPermissionHandlerPathRegistry_Profile_SwarmWorkerProfile(t *testing.T) {
	r := agentic.NewPermissionHandlerPathRegistry()
	p, ok := r.Profile(agentic.PermissionHandlerPathSwarmWorker)
	require.True(t, ok)
	assert.Equal(t, 2, p.AutomationLevel)
	assert.True(t, p.SupportsAutomatedResolution)
	assert.True(t, p.IsBackgroundAgentPath)
	assert.Empty(t, p.FeatureFlag, "swarm_worker requires no feature flag")
}

// --- Description completeness ---

func TestPermissionHandlerPathRegistry_AllProfilesHaveDescriptions(t *testing.T) {
	r := agentic.NewPermissionHandlerPathRegistry()
	for _, path := range r.AllPaths() {
		prof, ok := r.Profile(path)
		require.True(t, ok)
		assert.NotEmpty(t, prof.Description,
			"profile for %q must have a non-empty description", path)
	}
}
