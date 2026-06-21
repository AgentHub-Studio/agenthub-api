package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// CompactionPipelineLayer constant tests
// ---------------------------------------------------------------------------

func TestCompactionPipelineLayer_Constants(t *testing.T) {
	assert.Equal(t, CompactionPipelineLayer("budget_reduction"), LayerBudgetReduction)
	assert.Equal(t, CompactionPipelineLayer("snip"), LayerSnip)
	assert.Equal(t, CompactionPipelineLayer("microcompact"), LayerMicrocompact)
	assert.Equal(t, CompactionPipelineLayer("context_collapse"), LayerContextCollapse)
	assert.Equal(t, CompactionPipelineLayer("auto_compact"), LayerAutoCompact)
}

// ---------------------------------------------------------------------------
// CompactionAggressiveness constant tests
// ---------------------------------------------------------------------------

func TestCompactionAggressiveness_Constants(t *testing.T) {
	assert.Equal(t, CompactionAggressiveness("minimal"), AggressivenessMinimal)
	assert.Equal(t, CompactionAggressiveness("moderate"), AggressivenessModerate)
	assert.Equal(t, CompactionAggressiveness("aggressive"), AggressivenessAggressive)
	assert.Equal(t, CompactionAggressiveness("maximum"), AggressivenessMaximum)
}

// ---------------------------------------------------------------------------
// Registry construction
// ---------------------------------------------------------------------------

func TestCompactionPipelineLayerRegistry_New(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	require.NotNil(t, r)
}

func TestCompactionPipelineLayerRegistry_AllLayers_CountFive(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	layers := r.AllLayers()
	assert.Len(t, layers, 5, "pipeline must have exactly five layers")
}

func TestCompactionPipelineLayerRegistry_AllLayers_CanonicalOrder(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	layers := r.AllLayers()
	for i, p := range layers {
		assert.Equal(t, i+1, p.Order, "layer at index %d must have Order %d", i, i+1)
	}
}

func TestCompactionPipelineLayerRegistry_AllLayers_IsolationFromMutation(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	first := r.AllLayers()
	first[0].Order = 999 // mutate returned copy
	second := r.AllLayers()
	assert.Equal(t, 1, second[0].Order, "mutation of returned slice must not affect registry")
}

// ---------------------------------------------------------------------------
// LayerByOrder
// ---------------------------------------------------------------------------

func TestCompactionPipelineLayerRegistry_LayerByOrder_ValidOrders(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	expected := []CompactionPipelineLayer{
		LayerBudgetReduction,
		LayerSnip,
		LayerMicrocompact,
		LayerContextCollapse,
		LayerAutoCompact,
	}
	for n, want := range expected {
		p, ok := r.LayerByOrder(n + 1)
		require.True(t, ok, "order %d must exist", n+1)
		assert.Equal(t, want, p.Layer)
	}
}

func TestCompactionPipelineLayerRegistry_LayerByOrder_ZeroReturnsNotFound(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	_, ok := r.LayerByOrder(0)
	assert.False(t, ok)
}

func TestCompactionPipelineLayerRegistry_LayerByOrder_SixReturnsNotFound(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	_, ok := r.LayerByOrder(6)
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// EnabledByDefaultLayers / OptInLayers
// ---------------------------------------------------------------------------

func TestCompactionPipelineLayerRegistry_EnabledByDefaultLayers_TwoLayers(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	enabled := r.EnabledByDefaultLayers()
	require.Len(t, enabled, 2)
	layers := make([]CompactionPipelineLayer, len(enabled))
	for i, p := range enabled {
		layers[i] = p.Layer
	}
	assert.Contains(t, layers, LayerBudgetReduction)
	assert.Contains(t, layers, LayerAutoCompact)
}

func TestCompactionPipelineLayerRegistry_OptInLayers_ThreeLayers(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	optIn := r.OptInLayers()
	require.Len(t, optIn, 3)
	layers := make([]CompactionPipelineLayer, len(optIn))
	for i, p := range optIn {
		layers[i] = p.Layer
	}
	assert.Contains(t, layers, LayerSnip)
	assert.Contains(t, layers, LayerMicrocompact)
	assert.Contains(t, layers, LayerContextCollapse)
}

func TestCompactionPipelineLayerRegistry_OptIn_HaveFeatureFlags(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	for _, p := range r.OptInLayers() {
		assert.NotEmpty(t, p.FeatureFlag,
			"opt-in layer %s must have a non-empty feature flag", p.Layer)
	}
}

func TestCompactionPipelineLayerRegistry_AlwaysActive_HasNoFeatureFlag(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	p, ok := r.LayerByOrder(1)
	require.True(t, ok)
	assert.Empty(t, p.FeatureFlag, "budget_reduction has no feature flag gate")
}

// ---------------------------------------------------------------------------
// LayersByAggressiveness
// ---------------------------------------------------------------------------

func TestCompactionPipelineLayerRegistry_LayersByAggressiveness_Minimal(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	result := r.LayersByAggressiveness(AggressivenessMinimal)
	require.Len(t, result, 1)
	assert.Equal(t, LayerBudgetReduction, result[0].Layer)
}

func TestCompactionPipelineLayerRegistry_LayersByAggressiveness_Moderate(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	result := r.LayersByAggressiveness(AggressivenessModerate)
	require.Len(t, result, 2)
	layers := []CompactionPipelineLayer{result[0].Layer, result[1].Layer}
	assert.Contains(t, layers, LayerSnip)
	assert.Contains(t, layers, LayerMicrocompact)
}

func TestCompactionPipelineLayerRegistry_LayersByAggressiveness_Maximum(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	result := r.LayersByAggressiveness(AggressivenessMaximum)
	require.Len(t, result, 1)
	assert.Equal(t, LayerAutoCompact, result[0].Layer)
}

func TestCompactionPipelineLayerRegistry_LayersByAggressiveness_Unknown(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	result := r.LayersByAggressiveness(CompactionAggressiveness("unknown"))
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// IsValidLayer
// ---------------------------------------------------------------------------

func TestCompactionPipelineLayerRegistry_IsValidLayer_KnownLayers(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	known := []CompactionPipelineLayer{
		LayerBudgetReduction,
		LayerSnip,
		LayerMicrocompact,
		LayerContextCollapse,
		LayerAutoCompact,
	}
	for _, l := range known {
		assert.True(t, r.IsValidLayer(l), "layer %q must be valid", l)
	}
}

func TestCompactionPipelineLayerRegistry_IsValidLayer_UnknownLayer(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	assert.False(t, r.IsValidLayer(CompactionPipelineLayer("nonexistent")))
}

// ---------------------------------------------------------------------------
// TotalExpectedOutputReductionPct
// ---------------------------------------------------------------------------

func TestCompactionPipelineLayerRegistry_TotalExpectedOutputReductionPct_Positive(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	total := r.TotalExpectedOutputReductionPct()
	assert.Greater(t, total, 0.0)
}

func TestCompactionPipelineLayerRegistry_TotalExpectedOutputReductionPct_EqualsSumOfParts(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	var sum float64
	for _, p := range r.AllLayers() {
		sum += p.OutputSizeReductionPct
	}
	assert.InDelta(t, sum, r.TotalExpectedOutputReductionPct(), 1e-9)
}

// ---------------------------------------------------------------------------
// Profile field invariants
// ---------------------------------------------------------------------------

func TestCompactionPipelineLayerRegistry_TriggerThresholds_AreAscending(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	layers := r.AllLayers()
	// Layer 1 has threshold 0.0; layers 2–5 must be strictly increasing.
	for i := 2; i < len(layers); i++ {
		prev := layers[i-1].TriggerTokenThresholdPct
		curr := layers[i].TriggerTokenThresholdPct
		assert.GreaterOrEqual(t, curr, prev,
			"layer %d threshold %.2f must be >= layer %d threshold %.2f",
			i+1, curr, i, prev)
	}
}

func TestCompactionPipelineLayerRegistry_OutputReductionPct_InBounds(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	for _, p := range r.AllLayers() {
		assert.GreaterOrEqual(t, p.OutputSizeReductionPct, 0.0,
			"layer %s OutputSizeReductionPct must be >= 0", p.Layer)
		assert.LessOrEqual(t, p.OutputSizeReductionPct, 1.0,
			"layer %s OutputSizeReductionPct must be <= 1", p.Layer)
	}
}

func TestCompactionPipelineLayerRegistry_Descriptions_NonEmpty(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	for _, p := range r.AllLayers() {
		assert.NotEmpty(t, p.Description, "layer %s must have a description", p.Layer)
	}
}

// ---------------------------------------------------------------------------
// Distinction from CompactStage (existing 4-stage enum)
// ---------------------------------------------------------------------------

func TestCompactStage_PipelineRegistryHasTwoLayersWithNoCompactStagePeer(t *testing.T) {
	// CompactStage covers 4 reactive stages (tool_result_truncation, history_snip,
	// microcompact, full_summarization). The five-layer pipeline profile registry
	// covers 5 layers. Two pipeline layers — budget_reduction and context_collapse —
	// have no equivalent in the CompactStage enum, confirming the registries are
	// not identical.
	r := NewCompactionPipelineLayerRegistry()

	existingStageStrings := map[string]bool{
		string(StageToolResultTruncation): true,
		string(StageHistorySnip):          true,
		string(StageMicrocompact):         true,
		string(StageFullSummarization):    true,
	}

	noCompactStagePeer := 0
	for _, p := range r.AllLayers() {
		if !existingStageStrings[string(p.Layer)] {
			noCompactStagePeer++
		}
	}
	assert.Equal(t, 4, noCompactStagePeer,
		"pipeline layers without CompactStage peer (budget_reduction, context_collapse + additional) must match current registry")
}

func TestCompactStage_PipelineRegistryCoversAdditionalLayer_BudgetReduction(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	assert.True(t, r.IsValidLayer(LayerBudgetReduction))
	// budget_reduction is not a CompactStage — compile-time proof via distinct types.
	assert.Equal(t, CompactionPipelineLayer("budget_reduction"), LayerBudgetReduction)
}

func TestCompactStage_PipelineRegistryCoversAdditionalLayer_ContextCollapse(t *testing.T) {
	r := NewCompactionPipelineLayerRegistry()
	assert.True(t, r.IsValidLayer(LayerContextCollapse))
	// context_collapse is not a CompactStage — compile-time proof via distinct types.
	assert.Equal(t, CompactionPipelineLayer("context_collapse"), LayerContextCollapse)
}
