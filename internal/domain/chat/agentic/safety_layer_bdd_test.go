package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD scenarios for §3.5 "Permission and Safety Layers" seven-layer model.
// Each scenario follows Given/When/Then structure.

// TestBDD_SafetyLayer_SevenLayerCount — every agent must face exactly 7 independent
// safety layers per §3.5.
func TestBDD_SafetyLayer_SevenLayerCount(t *testing.T) {
	// Given: a freshly initialised safety-layer registry
	reg := NewSafetyLayerRegistry()

	// When: the consumer asks how many independent layers are registered
	count := reg.Count()

	// Then: exactly seven layers exist — matching the §3.5 numbered list
	assert.Equal(t, 7, count,
		"§3.5 explicitly enumerates seven independent safety layers; any deviation is a spec violation")
}

// TestBDD_SafetyLayer_AllLayersCanBlock — the §3.5 guarantee "any single layer
// can block" requires every layer to have CanBlock=true.
func TestBDD_SafetyLayer_AllLayersCanBlock(t *testing.T) {
	// Given: a registry of all seven §3.5 layers
	reg := NewSafetyLayerRegistry()
	layers := reg.AllLayers()
	require.Len(t, layers, 7)

	// When: each layer's CanBlock property is inspected
	// Then: every layer independently has blocking authority
	for _, l := range layers {
		assert.Truef(t, l.CanBlock,
			"layer %q (index %d) must be able to block — §3.5: 'any single layer can block it'",
			l.ID, l.LayerIndex)
	}
}

// TestBDD_SafetyLayer_ToolPreFilterIsFirst — the first layer is tool pre-filtering,
// which operates before any other check to remove blanket-denied tools.
func TestBDD_SafetyLayer_ToolPreFilterIsFirst(t *testing.T) {
	// Given: the ordered §3.5 layer sequence
	reg := NewSafetyLayerRegistry()

	// When: the layer at position 1 is retrieved
	first, ok := reg.LayerByIndex(1)
	require.True(t, ok, "layer at index 1 must exist")

	// Then: it is the tool pre-filter layer backed by tools.ts
	assert.Equal(t, SafetyLayerToolPreFilter, first.ID)
	assert.Equal(t, "tools.ts", first.SourceArtefact,
		"tool pre-filtering is implemented in tools.ts per §3.5")
}

// TestBDD_SafetyLayer_HookInterceptionIsLast — hook-based interception is the
// final (seventh) layer, allowing hooks to override all preceding decisions.
func TestBDD_SafetyLayer_HookInterceptionIsLast(t *testing.T) {
	// Given: the ordered §3.5 layer sequence
	reg := NewSafetyLayerRegistry()

	// When: the layer at position 7 is retrieved
	last, ok := reg.LayerByIndex(7)
	require.True(t, ok, "layer at index 7 must exist")

	// Then: it is the hook-interception layer backed by types/hooks.ts
	assert.Equal(t, SafetyLayerHookInterception, last.ID)
	assert.Equal(t, "types/hooks.ts", last.SourceArtefact,
		"hook interception is implemented in types/hooks.ts per §3.5")
}

// TestBDD_SafetyLayer_CorePropertiesExported — the two core §3.5 guarantees must
// be accessible as named constants so callers can document their reliance on them.
func TestBDD_SafetyLayer_CorePropertiesExported(t *testing.T) {
	// Given: the §3.5 safety-properties accessor
	// When: the core properties are queried
	props := SafetyProperties()

	// Then: both fundamental guarantees are present
	assert.Len(t, props, 2, "§3.5 states exactly two core guarantees")
	assert.Contains(t, props, SafetyPropertyAllLayersApply,
		"§3.5: 'a request must pass through all applicable layers'")
	assert.Contains(t, props, SafetyPropertyAnySingleLayerBlocks,
		"§3.5: 'any single layer can block it'")
}

// TestBDD_SafetyLayer_SourceArtefactTraceability — every layer must cite the
// TypeScript source artefact from §3.5, enabling a developer to trace each
// safety concern back to its implementation file.
func TestBDD_SafetyLayer_SourceArtefactTraceability(t *testing.T) {
	// Given: the full §3.5 layer catalogue
	reg := NewSafetyLayerRegistry()

	expectedArtefacts := map[SafetyLayerID]string{
		SafetyLayerToolPreFilter:            "tools.ts",
		SafetyLayerDenyFirstRuleEval:        "permissions.ts",
		SafetyLayerPermissionModeConstraint: "types/permissions.ts",
		SafetyLayerAutoModeClassifier:       "yoloClassifier.ts",
		SafetyLayerShellSandbox:             "shouldUseSandbox.ts",
		SafetyLayerNoResumePermissions:      "conversationRecovery.ts",
		SafetyLayerHookInterception:         "types/hooks.ts",
	}

	// When: each layer's source artefact is looked up
	for id, wantArtefact := range expectedArtefacts {
		p, ok := reg.Profile(id)
		require.Truef(t, ok, "layer %q must be registered", id)

		// Then: the artefact matches the §3.5 source citation
		assert.Equalf(t, wantArtefact, p.SourceArtefact,
			"layer %q should cite source artefact %q per §3.5", id, wantArtefact)
	}
}
