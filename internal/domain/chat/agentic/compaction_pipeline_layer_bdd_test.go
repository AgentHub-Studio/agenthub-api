package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBDD_CompactionPipelineLayer contains BDD-style scenarios that validate the
// §7.3 five-layer compaction pipeline registry against the stated architectural
// properties from the Claude Code paper.
func TestBDD_CompactionPipelineLayer(t *testing.T) {

	// Scenario 1: Pipeline has exactly five layers in canonical order 1→5
	t.Run("Given a new registry When AllLayers is called Then it returns five layers in order 1 to 5", func(t *testing.T) {
		// Given
		r := NewCompactionPipelineLayerRegistry()

		// When
		layers := r.AllLayers()

		// Then
		require.Len(t, layers, 5, "five-layer pipeline must have exactly 5 entries")
		expected := []CompactionPipelineLayer{
			LayerBudgetReduction,
			LayerSnip,
			LayerMicrocompact,
			LayerContextCollapse,
			LayerAutoCompact,
		}
		for i, want := range expected {
			assert.Equal(t, want, layers[i].Layer,
				"layer at position %d must be %s", i+1, want)
			assert.Equal(t, i+1, layers[i].Order,
				"Order field at index %d must equal %d", i, i+1)
		}
	})

	// Scenario 2: Budget reduction is always active (no feature flag, default enabled)
	t.Run("Given layer 1 budget_reduction When inspecting its profile Then it is always active with no feature flag", func(t *testing.T) {
		// Given
		r := NewCompactionPipelineLayerRegistry()

		// When
		p, ok := r.LayerByOrder(1)

		// Then
		require.True(t, ok)
		assert.Equal(t, LayerBudgetReduction, p.Layer)
		assert.True(t, p.DefaultEnabled, "budget_reduction must be enabled by default")
		assert.Empty(t, p.FeatureFlag, "budget_reduction must have no feature flag")
		assert.Equal(t, AggressivenessMinimal, p.Aggressiveness)
	})

	// Scenario 3: Three middle layers are opt-in via feature flags
	t.Run("Given layers 2 3 and 4 When checking opt-in status Then all three require feature flags", func(t *testing.T) {
		// Given
		r := NewCompactionPipelineLayerRegistry()

		// When
		optIn := r.OptInLayers()

		// Then
		require.Len(t, optIn, 3, "exactly three layers must be opt-in")
		for _, p := range optIn {
			assert.False(t, p.DefaultEnabled,
				"opt-in layer %s must have DefaultEnabled=false", p.Layer)
			assert.NotEmpty(t, p.FeatureFlag,
				"opt-in layer %s must carry a feature flag identifier", p.Layer)
		}
		// Verify the expected feature flag names match the paper
		featureFlags := make(map[string]bool)
		for _, p := range optIn {
			featureFlags[p.FeatureFlag] = true
		}
		assert.True(t, featureFlags["HISTORY_SNIP"], "HISTORY_SNIP flag must be present")
		assert.True(t, featureFlags["CACHED_MICROCOMPACT"], "CACHED_MICROCOMPACT flag must be present")
		assert.True(t, featureFlags["CONTEXT_COLLAPSE"], "CONTEXT_COLLAPSE flag must be present")
	})

	// Scenario 4: Auto-compact is enabled by default but carries no feature flag (user-configurable)
	t.Run("Given layer 5 auto_compact When inspecting its profile Then it is default-enabled without a feature flag", func(t *testing.T) {
		// Given
		r := NewCompactionPipelineLayerRegistry()

		// When
		p, ok := r.LayerByOrder(5)

		// Then
		require.True(t, ok)
		assert.Equal(t, LayerAutoCompact, p.Layer)
		assert.True(t, p.DefaultEnabled,
			"auto_compact must be enabled by default")
		assert.Empty(t, p.FeatureFlag,
			"auto_compact has no feature flag (user toggle, not a compile-time flag)")
		assert.Equal(t, AggressivenessMaximum, p.Aggressiveness,
			"auto_compact must be maximum aggressiveness (full model-generated summary)")
	})

	// Scenario 5: Aggressiveness escalates monotonically across layers
	t.Run("Given the five layers in order When comparing aggressiveness Then it never decreases", func(t *testing.T) {
		// Given
		r := NewCompactionPipelineLayerRegistry()
		aggressivenessRank := map[CompactionAggressiveness]int{
			AggressivenessMinimal:    1,
			AggressivenessModerate:   2,
			AggressivenessAggressive: 3,
			AggressivenessMaximum:    4,
		}

		// When
		layers := r.AllLayers()

		// Then
		for i := 1; i < len(layers); i++ {
			prev := aggressivenessRank[layers[i-1].Aggressiveness]
			curr := aggressivenessRank[layers[i].Aggressiveness]
			assert.GreaterOrEqual(t, curr, prev,
				"layer %d aggressiveness (%s, rank %d) must be >= layer %d (%s, rank %d)",
				i+1, layers[i].Aggressiveness, curr,
				i, layers[i-1].Aggressiveness, prev)
		}
	})

	// Scenario 6: CompactionPipelineLayerRegistry is a distinct type from CompactStage
	// Note: the paper reuses some of the same concept names (e.g. "microcompact",
	// "history_snip") across both the 4-stage reactive model (CompactStage) and the
	// 5-layer profile registry (CompactionPipelineLayer). The two types are
	// structurally separate Go types, not interchangeable — even when a string value
	// is shared. This scenario validates that the registry type and the stage type
	// are distinct and that the registry covers all five layers with correct Go types.
	t.Run("Given CompactStage enum and CompactionPipelineLayerRegistry When introspecting types Then registry returns CompactionPipelineLayer not CompactStage", func(t *testing.T) {
		// Given
		r := NewCompactionPipelineLayerRegistry()

		// When
		layers := r.AllLayers()

		// Then — verify all five layers are typed as CompactionPipelineLayer
		require.Len(t, layers, 5)
		for _, p := range layers {
			// Compile-time proof: p.Layer is CompactionPipelineLayer, not CompactStage.
			// Runtime: assert the underlying string is one of the five canonical values.
			assert.True(t, r.IsValidLayer(p.Layer),
				"each profile's Layer field must pass IsValidLayer — it is a typed CompactionPipelineLayer")
		}

		// The registry has more layers than CompactStage (5 vs 4) and includes
		// LayerBudgetReduction and LayerContextCollapse which have no CompactStage peer.
		_, hasBudgetReduction := func() (CompactionLayerProfile, bool) {
			return r.LayerByOrder(1)
		}()
		assert.True(t, hasBudgetReduction, "budget_reduction has no CompactStage equivalent")

		contextCollapsePresent := false
		for _, p := range layers {
			if p.Layer == LayerContextCollapse {
				contextCollapsePresent = true
			}
		}
		assert.True(t, contextCollapsePresent, "context_collapse has no CompactStage equivalent")
	})
}
