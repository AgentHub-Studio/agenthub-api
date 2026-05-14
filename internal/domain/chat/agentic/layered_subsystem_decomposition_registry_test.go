package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// FEAT036 — Unit tests: LayeredSubsystemDecompositionRegistry (§3.3)
// ---------------------------------------------------------------------------

// FEAT036_Unit_01: seed count matches the five §3.3 layers.
func TestFEAT036_Unit_01_SeedCountIsFive(t *testing.T) {
	assert.Equal(t, 5, SeedLayeredSubsystemDecompositionCount)
	assert.Len(t, AllSubsystemLayers(), 5)
}

// FEAT036_Unit_02: all five canonical slugs exist in the seed.
func TestFEAT036_Unit_02_AllSlugsPresent(t *testing.T) {
	slugs := SeedLayeredSubsystemDecompositionSlugs
	require.Len(t, slugs, 5)
	for _, s := range slugs {
		_, ok := FindSubsystemLayerBySlug(s)
		assert.True(t, ok, "slug %q should be findable", s)
	}
}

// FEAT036_Unit_03: LayerOrder values are exactly 1..5, no gaps.
func TestFEAT036_Unit_03_LayerOrderContiguous(t *testing.T) {
	layers := AllSubsystemLayers()
	seen := make(map[int]bool)
	for _, p := range layers {
		assert.False(t, seen[p.LayerOrder], "duplicate LayerOrder %d", p.LayerOrder)
		seen[p.LayerOrder] = true
	}
	for i := 1; i <= 5; i++ {
		assert.True(t, seen[i], "LayerOrder %d missing", i)
	}
}

// FEAT036_Unit_04: AllSubsystemLayers returns profiles in LayerOrder sequence.
func TestFEAT036_Unit_04_AllSubsystemLayersInOrder(t *testing.T) {
	layers := AllSubsystemLayers()
	for i, p := range layers {
		assert.Equal(t, i+1, p.LayerOrder, "profile at index %d has unexpected LayerOrder", i)
	}
}

// FEAT036_Unit_05: surface layer has correct slug, label, and order.
func TestFEAT036_Unit_05_SurfaceLayerProfile(t *testing.T) {
	p, ok := FindSubsystemLayerBySlug(SubsystemLayerSurface)
	require.True(t, ok)
	assert.Equal(t, SubsystemLayerSlug("surface"), p.Slug)
	assert.Equal(t, "Surface layer", p.Label)
	assert.Equal(t, 1, p.LayerOrder)
	assert.Equal(t, "§3.3", p.PDFSection)
	assert.Equal(t, "ingress", p.DirectionInDataFlow)
	assert.Equal(t, "rendering", p.ResponsibilityDomain)
	assert.Equal(t, "consumer", p.ContextPressureRole)
	assert.True(t, p.IsStateless)
	assert.False(t, p.HasIsolatedSubcontext)
	assert.Equal(t, SubsystemLayerSlug(""), p.CommunicatesUpTo)
	assert.Equal(t, SubsystemLayerCore, p.CommunicatesDownTo)
}

// FEAT036_Unit_06: core layer has correct slug, label, and orchestration fields.
func TestFEAT036_Unit_06_CoreLayerProfile(t *testing.T) {
	p, ok := FindSubsystemLayerBySlug(SubsystemLayerCore)
	require.True(t, ok)
	assert.Equal(t, SubsystemLayerSlug("core"), p.Slug)
	assert.Equal(t, "Core layer", p.Label)
	assert.Equal(t, 2, p.LayerOrder)
	assert.Equal(t, "dispatch", p.DirectionInDataFlow)
	assert.Equal(t, "orchestration", p.ResponsibilityDomain)
	assert.Equal(t, "manager", p.ContextPressureRole)
	assert.True(t, p.IsStateless)
	assert.Equal(t, SubsystemLayerSurface, p.CommunicatesUpTo)
	assert.Equal(t, SubsystemLayerSafetyAction, p.CommunicatesDownTo)
	assert.Contains(t, p.KeySourceFiles, "query.ts")
}

// FEAT036_Unit_07: safety/action layer has isolated subcontext capability.
func TestFEAT036_Unit_07_SafetyActionLayerIsolation(t *testing.T) {
	p, ok := FindSubsystemLayerBySlug(SubsystemLayerSafetyAction)
	require.True(t, ok)
	assert.Equal(t, SubsystemLayerSlug("safety_action"), p.Slug)
	assert.Equal(t, 3, p.LayerOrder)
	assert.True(t, p.HasIsolatedSubcontext,
		"safety/action layer must support isolated subcontext (subagent spawning)")
	assert.False(t, p.IsStateless)
	assert.Equal(t, "gatekeeper", p.ContextPressureRole)
	assert.Equal(t, "gate_and_route", p.DirectionInDataFlow)
	assert.Contains(t, p.KeySourceFiles, "AgentTool.tsx")
	assert.Contains(t, p.KeySourceFiles, "permissions.ts")
}

// FEAT036_Unit_08: state layer is stateful and is the memory producer.
func TestFEAT036_Unit_08_StateLayerStateful(t *testing.T) {
	p, ok := FindSubsystemLayerBySlug(SubsystemLayerState)
	require.True(t, ok)
	assert.Equal(t, SubsystemLayerSlug("state"), p.Slug)
	assert.Equal(t, 4, p.LayerOrder)
	assert.False(t, p.IsStateless)
	assert.Equal(t, "producer", p.ContextPressureRole)
	assert.Equal(t, "memoized_state", p.DirectionInDataFlow)
	assert.Contains(t, p.KeySourceFiles, "sessionStorage.ts")
	assert.Contains(t, p.KeySourceFiles, "claudemd.ts")
}

// FEAT036_Unit_09: backend layer is the execution egress with no downstream.
func TestFEAT036_Unit_09_BackendLayerEgress(t *testing.T) {
	p, ok := FindSubsystemLayerBySlug(SubsystemLayerBackend)
	require.True(t, ok)
	assert.Equal(t, SubsystemLayerSlug("backend"), p.Slug)
	assert.Equal(t, 5, p.LayerOrder)
	assert.Equal(t, "egress", p.DirectionInDataFlow)
	assert.Equal(t, "execution", p.ResponsibilityDomain)
	assert.Equal(t, "executor", p.ContextPressureRole)
	assert.True(t, p.IsStateless)
	assert.Equal(t, SubsystemLayerSlug(""), p.CommunicatesDownTo)
	assert.Equal(t, SubsystemLayerState, p.CommunicatesUpTo)
	assert.Contains(t, p.KeySourceFiles, "BashTool.tsx")
}

// FEAT036_Unit_10: FindSubsystemLayerBySlug returns false for unknown slug.
func TestFEAT036_Unit_10_FindUnknownSlugReturnsFalse(t *testing.T) {
	_, ok := FindSubsystemLayerBySlug("nonexistent_layer")
	assert.False(t, ok)
}

// FEAT036_Unit_11: IsValidSubsystemLayerSlug returns true for all known slugs.
func TestFEAT036_Unit_11_ValidSlugAllKnown(t *testing.T) {
	for _, s := range SeedLayeredSubsystemDecompositionSlugs {
		assert.True(t, IsValidSubsystemLayerSlug(s))
	}
}

// FEAT036_Unit_12: IsValidSubsystemLayerSlug returns false for unknown slug.
func TestFEAT036_Unit_12_ValidSlugUnknown(t *testing.T) {
	assert.False(t, IsValidSubsystemLayerSlug("middleware"))
}

// FEAT036_Unit_13: SubsystemLayerByOrder resolves each order 1..5.
func TestFEAT036_Unit_13_LayerByOrderAllValid(t *testing.T) {
	for i := 1; i <= 5; i++ {
		p, ok := SubsystemLayerByOrder(i)
		require.True(t, ok, "order %d should resolve", i)
		assert.Equal(t, i, p.LayerOrder)
	}
}

// FEAT036_Unit_14: SubsystemLayerByOrder returns false for out-of-range orders.
func TestFEAT036_Unit_14_LayerByOrderOutOfRange(t *testing.T) {
	for _, bad := range []int{0, 6, -1, 100} {
		_, ok := SubsystemLayerByOrder(bad)
		assert.False(t, ok, "order %d should not resolve", bad)
	}
}

// FEAT036_Unit_15: LayersWithIsolatedSubcontext returns exactly one layer (safety/action).
func TestFEAT036_Unit_15_IsolatedSubcontextOnlySafetyAction(t *testing.T) {
	isolated := LayersWithIsolatedSubcontext()
	require.Len(t, isolated, 1)
	assert.Equal(t, SubsystemLayerSafetyAction, isolated[0].Slug)
}

// FEAT036_Unit_16: StatefulLayers returns exactly two layers (safety/action + state).
func TestFEAT036_Unit_16_StatefulLayersCount(t *testing.T) {
	stateful := StatefulLayers()
	require.Len(t, stateful, 2)
	slugs := make([]SubsystemLayerSlug, 0, 2)
	for _, p := range stateful {
		slugs = append(slugs, p.Slug)
	}
	assert.Contains(t, slugs, SubsystemLayerSafetyAction)
	assert.Contains(t, slugs, SubsystemLayerState)
}

// FEAT036_Unit_17: StatelessLayers returns exactly three layers (surface, core, backend).
func TestFEAT036_Unit_17_StatelessLayersCount(t *testing.T) {
	stateless := StatelessLayers()
	require.Len(t, stateless, 3)
	slugs := make([]SubsystemLayerSlug, 0, 3)
	for _, p := range stateless {
		slugs = append(slugs, p.Slug)
	}
	assert.Contains(t, slugs, SubsystemLayerSurface)
	assert.Contains(t, slugs, SubsystemLayerCore)
	assert.Contains(t, slugs, SubsystemLayerBackend)
}

// FEAT036_Unit_18: LayersByResponsibilityDomain returns one profile per distinct domain.
func TestFEAT036_Unit_18_LayersByResponsibilityDomain(t *testing.T) {
	cases := map[string]SubsystemLayerSlug{
		"rendering":        SubsystemLayerSurface,
		"orchestration":    SubsystemLayerCore,
		"safety_and_tools": SubsystemLayerSafetyAction,
		"state_and_memory": SubsystemLayerState,
		"execution":        SubsystemLayerBackend,
	}
	for domain, expectedSlug := range cases {
		results := LayersByResponsibilityDomain(domain)
		require.Len(t, results, 1, "domain %q should match exactly one layer", domain)
		assert.Equal(t, expectedSlug, results[0].Slug)
	}
}

// FEAT036_Unit_19: LayersByContextPressureRole returns one profile per distinct role.
func TestFEAT036_Unit_19_LayersByContextPressureRole(t *testing.T) {
	cases := map[string]SubsystemLayerSlug{
		"consumer":   SubsystemLayerSurface,
		"manager":    SubsystemLayerCore,
		"gatekeeper": SubsystemLayerSafetyAction,
		"producer":   SubsystemLayerState,
		"executor":   SubsystemLayerBackend,
	}
	for role, expectedSlug := range cases {
		results := LayersByContextPressureRole(role)
		require.Len(t, results, 1, "role %q should match exactly one layer", role)
		assert.Equal(t, expectedSlug, results[0].Slug)
	}
}

// FEAT036_Unit_20: DataFlowSpine returns all 5 layers in order.
func TestFEAT036_Unit_20_DataFlowSpineInOrder(t *testing.T) {
	spine := DataFlowSpine()
	require.Len(t, spine, 5)
	for i, p := range spine {
		assert.Equal(t, i+1, p.LayerOrder, "spine[%d] has wrong LayerOrder", i)
	}
}

// FEAT036_Unit_21: DownstreamNeighbour chains correctly through the spine.
func TestFEAT036_Unit_21_DownstreamNeighbourChain(t *testing.T) {
	chain := []SubsystemLayerSlug{
		SubsystemLayerSurface,
		SubsystemLayerCore,
		SubsystemLayerSafetyAction,
		SubsystemLayerState,
	}
	expected := []SubsystemLayerSlug{
		SubsystemLayerCore,
		SubsystemLayerSafetyAction,
		SubsystemLayerState,
		SubsystemLayerBackend,
	}
	for i, slug := range chain {
		next, ok := DownstreamNeighbour(slug)
		require.True(t, ok, "layer %q should have a downstream neighbour", slug)
		assert.Equal(t, expected[i], next.Slug)
	}
}

// FEAT036_Unit_22: DownstreamNeighbour of backend returns false (no downstream).
func TestFEAT036_Unit_22_BackendHasNoDownstream(t *testing.T) {
	_, ok := DownstreamNeighbour(SubsystemLayerBackend)
	assert.False(t, ok)
}

// FEAT036_Unit_23: UpstreamNeighbour chains correctly through the spine.
func TestFEAT036_Unit_23_UpstreamNeighbourChain(t *testing.T) {
	chain := []SubsystemLayerSlug{
		SubsystemLayerBackend,
		SubsystemLayerState,
		SubsystemLayerSafetyAction,
		SubsystemLayerCore,
	}
	expected := []SubsystemLayerSlug{
		SubsystemLayerState,
		SubsystemLayerSafetyAction,
		SubsystemLayerCore,
		SubsystemLayerSurface,
	}
	for i, slug := range chain {
		prev, ok := UpstreamNeighbour(slug)
		require.True(t, ok, "layer %q should have an upstream neighbour", slug)
		assert.Equal(t, expected[i], prev.Slug)
	}
}

// FEAT036_Unit_24: UpstreamNeighbour of surface returns false (no upstream).
func TestFEAT036_Unit_24_SurfaceHasNoUpstream(t *testing.T) {
	_, ok := UpstreamNeighbour(SubsystemLayerSurface)
	assert.False(t, ok)
}

// FEAT036_Unit_25: AllSubsystemLayers returns a copy — mutations do not affect the seed.
func TestFEAT036_Unit_25_AllSubsystemLayersCopy(t *testing.T) {
	layers := AllSubsystemLayers()
	layers[0].Label = "MUTATED"
	fresh := AllSubsystemLayers()
	assert.NotEqual(t, "MUTATED", fresh[0].Label,
		"AllSubsystemLayers must return an independent copy")
}

// ---------------------------------------------------------------------------
// FEAT036 — BDD tests: LayeredSubsystemDecompositionRegistry (§3.3)
// ---------------------------------------------------------------------------

// TestFEAT036_BDD_SurfaceLayerIngressBehaviour
// Given §3.3 defines the surface layer as the ingress of the data-flow spine
// When the caller queries the surface layer profile
// Then DirectionInDataFlow must be "ingress", CommunicatesUpTo must be empty,
// and the AgentHub equivalent must reference the HTTP handler layer.
func TestFEAT036_BDD_SurfaceLayerIngressBehaviour(t *testing.T) {
	// Given
	p, ok := FindSubsystemLayerBySlug(SubsystemLayerSurface)
	require.True(t, ok, "surface layer must exist in the registry")

	// When/Then
	assert.Equal(t, "ingress", p.DirectionInDataFlow,
		"surface layer must be the ingress point of the data-flow spine")
	assert.Equal(t, SubsystemLayerSlug(""), p.CommunicatesUpTo,
		"surface layer has no upstream neighbour — it is the first layer")
	assert.NotEmpty(t, p.AgentHubLayerEquivalent,
		"surface layer must have an AgentHub equivalent documented")
	// The HTTP handler reference is mandatory in the AgentHub mapping.
	assert.Contains(t, p.AgentHubLayerEquivalent, "handler",
		"AgentHub mapping must reference the HTTP handler layer")
}

// TestFEAT036_BDD_SafetyActionLayerIsOnlyIsolationProvider
// Given §3.3 identifies subagent spawning as a safety/action layer concern
// When LayersWithIsolatedSubcontext is called
// Then exactly one layer is returned and it is the safety/action layer.
func TestFEAT036_BDD_SafetyActionLayerIsOnlyIsolationProvider(t *testing.T) {
	// Given
	isolatedLayers := LayersWithIsolatedSubcontext()

	// When/Then
	assert.Len(t, isolatedLayers, 1,
		"only the safety/action layer should support isolated subcontext")
	if len(isolatedLayers) > 0 {
		assert.Equal(t, SubsystemLayerSafetyAction, isolatedLayers[0].Slug,
			"the single isolated-subcontext layer must be safety_action")
	}
}

// TestFEAT036_BDD_CoreLayerKeySourceFilesIncludeQueryLoop
// Given §3.3 states that queryLoop() (query.ts) implements the iterative agent loop
// When the core layer profile is retrieved
// Then query.ts must appear in KeySourceFiles and the subtitle must reference the agent loop.
func TestFEAT036_BDD_CoreLayerKeySourceFilesIncludeQueryLoop(t *testing.T) {
	// Given
	p, ok := FindSubsystemLayerBySlug(SubsystemLayerCore)
	require.True(t, ok)

	// When/Then
	assert.Contains(t, p.KeySourceFiles, "query.ts",
		"core layer must cite query.ts (queryLoop) as a key source file")
	assert.Contains(t, p.SubtitleInPaper, "agent loop",
		"core layer subtitle must reference the agent loop")
}

// TestFEAT036_BDD_DataFlowSpineFormsContinuousChain
// Given §3.3 describes a left-to-right spine: surface → core → safety/action → state → backend
// When each layer's CommunicatesDownTo is followed starting from surface
// Then every layer in the spine is reachable without gaps and backend terminates the chain.
func TestFEAT036_BDD_DataFlowSpineFormsContinuousChain(t *testing.T) {
	// Given
	expectedSpine := []SubsystemLayerSlug{
		SubsystemLayerSurface,
		SubsystemLayerCore,
		SubsystemLayerSafetyAction,
		SubsystemLayerState,
		SubsystemLayerBackend,
	}

	// When — walk the chain starting from surface
	current := SubsystemLayerSurface
	walked := []SubsystemLayerSlug{current}
	for {
		next, ok := DownstreamNeighbour(current)
		if !ok {
			break
		}
		walked = append(walked, next.Slug)
		current = next.Slug
		if len(walked) > 10 {
			t.Fatal("data-flow spine walk did not terminate")
		}
	}

	// Then
	assert.Equal(t, expectedSpine, walked,
		"walking CommunicatesDownTo from surface must traverse the exact §3.3 spine")
	assert.Equal(t, SubsystemLayerBackend, walked[len(walked)-1],
		"the spine must terminate at the backend layer")
}

// TestFEAT036_BDD_StateLayerIsProducerAndStateful
// Given §3.3 describes context assembly as a memoized state loader that produces state
// for the upstream layers
// When the state layer profile is queried
// Then it must be marked stateful (IsStateless=false) and have ContextPressureRole="producer".
func TestFEAT036_BDD_StateLayerIsProducerAndStateful(t *testing.T) {
	// Given
	p, ok := FindSubsystemLayerBySlug(SubsystemLayerState)
	require.True(t, ok)

	// When/Then
	assert.False(t, p.IsStateless,
		"state layer must be stateful — it holds CLAUDE.md memory, session transcripts, runtime state")
	assert.Equal(t, "producer", p.ContextPressureRole,
		"state layer is the producer of context for the rest of the system")
	assert.Contains(t, p.KeySourceFiles, "sessionStorage.ts",
		"sessionStorage.ts is the canonical §3.3 state-layer source file for session persistence")
}

// TestFEAT036_BDD_LayerByOrderMatchesSlugOrder
// Given the LayerOrder fields are authoritative for spine position
// When SubsystemLayerByOrder is used to retrieve each layer 1..5
// Then the retrieved profile slugs must match the SeedLayeredSubsystemDecompositionSlugs order.
func TestFEAT036_BDD_LayerByOrderMatchesSlugOrder(t *testing.T) {
	// Given
	expectedOrder := SeedLayeredSubsystemDecompositionSlugs

	// When/Then
	for i, expectedSlug := range expectedOrder {
		p, ok := SubsystemLayerByOrder(i + 1)
		require.True(t, ok, "SubsystemLayerByOrder(%d) must succeed", i+1)
		assert.Equal(t, expectedSlug, p.Slug,
			"layer at order %d must have slug %q", i+1, expectedSlug)
	}
}

// TestFEAT036_BDD_AllFiveContextPressureRolesAreRepresented
// Given §3.3 five layers address context pressure in distinct roles
// When LayersByContextPressureRole is queried for each of the five expected roles
// Then each role must map to exactly one distinct layer covering the full set.
func TestFEAT036_BDD_AllFiveContextPressureRolesAreRepresented(t *testing.T) {
	// Given — the five roles named after §3.3 context-pressure responsibilities
	roles := []string{"consumer", "manager", "gatekeeper", "producer", "executor"}

	// When/Then
	coveredSlugs := make(map[SubsystemLayerSlug]bool)
	for _, role := range roles {
		results := LayersByContextPressureRole(role)
		assert.Len(t, results, 1, "role %q must match exactly one layer", role)
		if len(results) == 1 {
			slug := results[0].Slug
			assert.False(t, coveredSlugs[slug],
				"slug %q already claimed by another role — roles must be unique per layer", slug)
			coveredSlugs[slug] = true
		}
	}
	assert.Len(t, coveredSlugs, 5, "all five layers must each have a distinct context-pressure role")
}
