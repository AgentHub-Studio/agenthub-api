package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSafetyLayerRegistry_Count verifies there are exactly 7 layers (§3.5 guarantee).
func TestSafetyLayerRegistry_Count(t *testing.T) {
	reg := NewSafetyLayerRegistry()
	assert.Equal(t, 7, reg.Count(), "§3.5 mandates exactly seven independent safety layers")
}

// TestSafetyLayerRegistry_AllLayers_Ordered verifies enumeration order (layer 1..7).
func TestSafetyLayerRegistry_AllLayers_Ordered(t *testing.T) {
	reg := NewSafetyLayerRegistry()
	layers := reg.AllLayers()
	require.Len(t, layers, 7)
	for i, l := range layers {
		assert.Equal(t, i+1, l.LayerIndex, "layer at position %d should have LayerIndex %d", i, i+1)
	}
}

// TestSafetyLayerRegistry_Profile_Known checks every layer ID resolves.
func TestSafetyLayerRegistry_Profile_Known(t *testing.T) {
	reg := NewSafetyLayerRegistry()
	knownIDs := []SafetyLayerID{
		SafetyLayerToolPreFilter,
		SafetyLayerDenyFirstRuleEval,
		SafetyLayerPermissionModeConstraint,
		SafetyLayerAutoModeClassifier,
		SafetyLayerShellSandbox,
		SafetyLayerNoResumePermissions,
		SafetyLayerHookInterception,
	}
	for _, id := range knownIDs {
		p, ok := reg.Profile(id)
		require.Truef(t, ok, "expected Profile(%q) to be found", id)
		assert.Equal(t, id, p.ID)
		assert.NotEmpty(t, p.Label)
		assert.NotEmpty(t, p.Description)
		assert.NotEmpty(t, p.SourceArtefact)
		assert.True(t, p.CanBlock, "all §3.5 layers must be able to block")
	}
}

// TestSafetyLayerRegistry_Profile_Unknown returns false for unknown IDs.
func TestSafetyLayerRegistry_Profile_Unknown(t *testing.T) {
	reg := NewSafetyLayerRegistry()
	_, ok := reg.Profile("nonexistent_layer")
	assert.False(t, ok)
}

// TestSafetyLayerRegistry_IsValidSafetyLayer_Valid verifies all seven IDs.
func TestSafetyLayerRegistry_IsValidSafetyLayer_Valid(t *testing.T) {
	reg := NewSafetyLayerRegistry()
	validIDs := []SafetyLayerID{
		SafetyLayerToolPreFilter,
		SafetyLayerDenyFirstRuleEval,
		SafetyLayerPermissionModeConstraint,
		SafetyLayerAutoModeClassifier,
		SafetyLayerShellSandbox,
		SafetyLayerNoResumePermissions,
		SafetyLayerHookInterception,
	}
	for _, id := range validIDs {
		assert.Truef(t, reg.IsValidSafetyLayer(id), "expected %q to be valid", id)
	}
}

// TestSafetyLayerRegistry_IsValidSafetyLayer_Invalid rejects unknown IDs.
func TestSafetyLayerRegistry_IsValidSafetyLayer_Invalid(t *testing.T) {
	reg := NewSafetyLayerRegistry()
	assert.False(t, reg.IsValidSafetyLayer("bad_layer"))
	assert.False(t, reg.IsValidSafetyLayer(""))
}

// TestSafetyLayerRegistry_LayerByIndex_Valid verifies correct 1-based lookup.
func TestSafetyLayerRegistry_LayerByIndex_Valid(t *testing.T) {
	reg := NewSafetyLayerRegistry()
	cases := []struct {
		index int
		wantID SafetyLayerID
	}{
		{1, SafetyLayerToolPreFilter},
		{2, SafetyLayerDenyFirstRuleEval},
		{3, SafetyLayerPermissionModeConstraint},
		{4, SafetyLayerAutoModeClassifier},
		{5, SafetyLayerShellSandbox},
		{6, SafetyLayerNoResumePermissions},
		{7, SafetyLayerHookInterception},
	}
	for _, tc := range cases {
		p, ok := reg.LayerByIndex(tc.index)
		require.Truef(t, ok, "LayerByIndex(%d) should succeed", tc.index)
		assert.Equalf(t, tc.wantID, p.ID, "LayerByIndex(%d) should return %q", tc.index, tc.wantID)
		assert.Equal(t, tc.index, p.LayerIndex)
	}
}

// TestSafetyLayerRegistry_LayerByIndex_OutOfRange rejects 0 and 8.
func TestSafetyLayerRegistry_LayerByIndex_OutOfRange(t *testing.T) {
	reg := NewSafetyLayerRegistry()
	for _, bad := range []int{0, -1, 8, 100} {
		_, ok := reg.LayerByIndex(bad)
		assert.Falsef(t, ok, "LayerByIndex(%d) should return false", bad)
	}
}

// TestSafetyLayerRegistry_LayersBySourceArtefact matches known files.
func TestSafetyLayerRegistry_LayersBySourceArtefact(t *testing.T) {
	reg := NewSafetyLayerRegistry()

	cases := []struct {
		artefact  string
		wantCount int
		wantID    SafetyLayerID
	}{
		{"tools.ts", 1, SafetyLayerToolPreFilter},
		{"permissions.ts", 1, SafetyLayerDenyFirstRuleEval},
		{"types/permissions.ts", 1, SafetyLayerPermissionModeConstraint},
		{"yoloClassifier.ts", 1, SafetyLayerAutoModeClassifier},
		{"shouldUseSandbox.ts", 1, SafetyLayerShellSandbox},
		{"conversationRecovery.ts", 1, SafetyLayerNoResumePermissions},
		{"types/hooks.ts", 1, SafetyLayerHookInterception},
	}
	for _, tc := range cases {
		got := reg.LayersBySourceArtefact(tc.artefact)
		require.Lenf(t, got, tc.wantCount, "artefact %q should match %d layer(s)", tc.artefact, tc.wantCount)
		assert.Equal(t, tc.wantID, got[0].ID)
	}
}

// TestSafetyLayerRegistry_LayersBySourceArtefact_Unknown returns empty slice.
func TestSafetyLayerRegistry_LayersBySourceArtefact_Unknown(t *testing.T) {
	reg := NewSafetyLayerRegistry()
	got := reg.LayersBySourceArtefact("nonexistent.ts")
	assert.Empty(t, got)
}

// TestSafetyProperties verifies the two §3.5 core guarantees are exported.
func TestSafetyProperties(t *testing.T) {
	props := SafetyProperties()
	assert.Len(t, props, 2)
	assert.Contains(t, props, SafetyPropertyAllLayersApply)
	assert.Contains(t, props, SafetyPropertyAnySingleLayerBlocks)
}

// TestSafetyLayerRegistry_AllLayers_AllCanBlock verifies the §3.5 guarantee
// that every layer independently has the ability to block a request.
func TestSafetyLayerRegistry_AllLayers_AllCanBlock(t *testing.T) {
	reg := NewSafetyLayerRegistry()
	for _, l := range reg.AllLayers() {
		assert.Truef(t, l.CanBlock, "layer %q (index %d) must have CanBlock=true", l.ID, l.LayerIndex)
	}
}

// TestSafetyLayerRegistry_AllLayers_UniqueIndexes verifies no two layers share
// the same LayerIndex.
func TestSafetyLayerRegistry_AllLayers_UniqueIndexes(t *testing.T) {
	reg := NewSafetyLayerRegistry()
	seen := map[int]SafetyLayerID{}
	for _, l := range reg.AllLayers() {
		prev, dup := seen[l.LayerIndex]
		assert.Falsef(t, dup, "LayerIndex %d used by both %q and %q", l.LayerIndex, prev, l.ID)
		seen[l.LayerIndex] = l.ID
	}
}

// TestSafetyLayerRegistry_AllLayers_NonEmptyLabels verifies all labels are set.
func TestSafetyLayerRegistry_AllLayers_NonEmptyLabels(t *testing.T) {
	reg := NewSafetyLayerRegistry()
	for _, l := range reg.AllLayers() {
		assert.NotEmptyf(t, l.Label, "layer %q must have a non-empty label", l.ID)
		assert.NotEmptyf(t, l.Description, "layer %q must have a non-empty description", l.ID)
	}
}

// TestSafetyLayerID_Constants verifies string values of exported constants.
func TestSafetyLayerID_Constants(t *testing.T) {
	assert.Equal(t, SafetyLayerID("tool_pre_filter"), SafetyLayerToolPreFilter)
	assert.Equal(t, SafetyLayerID("deny_first_rule_eval"), SafetyLayerDenyFirstRuleEval)
	assert.Equal(t, SafetyLayerID("permission_mode_constraint"), SafetyLayerPermissionModeConstraint)
	assert.Equal(t, SafetyLayerID("auto_mode_classifier"), SafetyLayerAutoModeClassifier)
	assert.Equal(t, SafetyLayerID("shell_sandbox"), SafetyLayerShellSandbox)
	assert.Equal(t, SafetyLayerID("no_resume_permissions"), SafetyLayerNoResumePermissions)
	assert.Equal(t, SafetyLayerID("hook_interception"), SafetyLayerHookInterception)
}
