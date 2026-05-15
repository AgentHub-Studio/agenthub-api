package agentic

// CompactionPipelineLayer is a typed string identifying one of the five compaction
// layers in the §7.3 graduated compaction pipeline.
//
// The five layers are applied in order (1→5), escalating aggressiveness only when
// cheaper strategies prove insufficient ("lazy-degradation principle").
//
// Inspired by Claude Code's compact.ts five-layer pipeline.
type CompactionPipelineLayer string

const (
	// LayerBudgetReduction is layer 1: per-tool-result size limits.
	// Always active — no feature flag required.
	LayerBudgetReduction CompactionPipelineLayer = "budget_reduction"

	// LayerSnip is layer 2: lightweight trimming of older history.
	// Gated by the HISTORY_SNIP feature flag.
	LayerSnip CompactionPipelineLayer = "snip"

	// LayerMicrocompact is layer 3: fine-grained cache-aware compression.
	// Gated by the CACHED_MICROCOMPACT feature flag.
	LayerMicrocompact CompactionPipelineLayer = "microcompact"

	// LayerContextCollapse is layer 4: read-time virtual projection over history.
	// Gated by the CONTEXT_COLLAPSE feature flag.
	LayerContextCollapse CompactionPipelineLayer = "context_collapse"

	// LayerAutoCompact is layer 5: full model-generated summary.
	// Enabled by default; the user may disable it via configuration.
	LayerAutoCompact CompactionPipelineLayer = "auto_compact"
)

// CompactionAggressiveness classifies how disruptive a compaction layer is
// with respect to context fidelity.
type CompactionAggressiveness string

const (
	// AggressivenessMinimal applies the least-disruptive transformation;
	// information loss is negligible.
	AggressivenessMinimal CompactionAggressiveness = "minimal"

	// AggressivenessModerate applies moderate transformation;
	// some older context may be lost.
	AggressivenessModerate CompactionAggressiveness = "moderate"

	// AggressivenessAggressive applies aggressive transformation;
	// significant older context is discarded or compressed.
	AggressivenessAggressive CompactionAggressiveness = "aggressive"

	// AggressivenessMaximum applies maximum transformation;
	// the full history is replaced with a model-generated summary.
	AggressivenessMaximum CompactionAggressiveness = "maximum"
)

// CompactionLayerProfile describes the static characteristics of one compaction layer.
//
// Profiles are immutable and registered in [CompactionPipelineLayerRegistry].
type CompactionLayerProfile struct {
	// Layer is the canonical identifier for this layer.
	Layer CompactionPipelineLayer

	// Order is the canonical position in the pipeline (1 = first applied, 5 = last).
	Order int

	// DefaultEnabled indicates whether the layer is active without explicit opt-in.
	// LayerBudgetReduction is always active (not a toggle).
	// LayerAutoCompact defaults to true but the user may disable it.
	// Layers 2–4 default to false (gated by feature flags).
	DefaultEnabled bool

	// FeatureFlag is the string identifier of the feature flag that gates this layer.
	// Empty string means the layer has no feature flag (always active or user-toggle).
	FeatureFlag string

	// Aggressiveness classifies the information-loss impact of this layer.
	Aggressiveness CompactionAggressiveness

	// TriggerTokenThresholdPct is the approximate context-window fill fraction
	// at which this layer becomes relevant (0.0–1.0).
	// 0.0 means "always evaluated regardless of window pressure".
	TriggerTokenThresholdPct float64

	// OutputSizeReductionPct is the estimated percentage reduction in effective
	// context size that this layer achieves (0.0–1.0).
	OutputSizeReductionPct float64

	// Description is a human-readable summary of what this layer does.
	Description string
}

// compactionPipelineProfiles is the canonical ordered registry of all five layers.
var compactionPipelineProfiles = []CompactionLayerProfile{
	{
		Layer:                    LayerBudgetReduction,
		Order:                    1,
		DefaultEnabled:           true,
		FeatureFlag:              "",
		Aggressiveness:           AggressivenessMinimal,
		TriggerTokenThresholdPct: 0.0,
		OutputSizeReductionPct:   0.10,
		Description:              "Per-tool-result size limits; always active; least disruptive.",
	},
	{
		Layer:                    LayerSnip,
		Order:                    2,
		DefaultEnabled:           false,
		FeatureFlag:              "HISTORY_SNIP",
		Aggressiveness:           AggressivenessModerate,
		TriggerTokenThresholdPct: 0.75,
		OutputSizeReductionPct:   0.20,
		Description:              "Lightweight trimming of older conversation history.",
	},
	{
		Layer:                    LayerMicrocompact,
		Order:                    3,
		DefaultEnabled:           false,
		FeatureFlag:              "CACHED_MICROCOMPACT",
		Aggressiveness:           AggressivenessModerate,
		TriggerTokenThresholdPct: 0.80,
		OutputSizeReductionPct:   0.25,
		Description:              "Fine-grained cache-aware compression; emits a boundary marker.",
	},
	{
		Layer:                    LayerContextCollapse,
		Order:                    4,
		DefaultEnabled:           false,
		FeatureFlag:              "CONTEXT_COLLAPSE",
		Aggressiveness:           AggressivenessAggressive,
		TriggerTokenThresholdPct: 0.85,
		OutputSizeReductionPct:   0.40,
		Description:              "Read-time virtual projection over history; no user-visible output.",
	},
	{
		Layer:                    LayerAutoCompact,
		Order:                    5,
		DefaultEnabled:           true,
		FeatureFlag:              "",
		Aggressiveness:           AggressivenessMaximum,
		TriggerTokenThresholdPct: 0.90,
		OutputSizeReductionPct:   0.70,
		Description:              "Full model-generated summary; replaces entire history; user-configurable.",
	},
}

// CompactionPipelineLayerRegistry provides read-only access to the five-layer
// compaction pipeline profiles defined in §7.3 of the Claude Code architecture paper.
//
// The registry is distinct from [CompactStage], which covers four reactive stages
// without per-layer profiles, feature flags, or aggressiveness metadata.
type CompactionPipelineLayerRegistry struct {
	profiles []CompactionLayerProfile
}

// NewCompactionPipelineLayerRegistry returns a registry initialised with the five
// canonical §7.3 compaction layer profiles.
func NewCompactionPipelineLayerRegistry() *CompactionPipelineLayerRegistry {
	// defensive copy so callers cannot mutate the package-level slice.
	cp := make([]CompactionLayerProfile, len(compactionPipelineProfiles))
	copy(cp, compactionPipelineProfiles)
	return &CompactionPipelineLayerRegistry{profiles: cp}
}

// AllLayers returns all five profiles in canonical pipeline order (1→5).
func (r *CompactionPipelineLayerRegistry) AllLayers() []CompactionLayerProfile {
	result := make([]CompactionLayerProfile, len(r.profiles))
	copy(result, r.profiles)
	return result
}

// LayerByOrder returns the profile whose Order equals n (1–5).
// Returns the zero value and false if n is out of range.
func (r *CompactionPipelineLayerRegistry) LayerByOrder(n int) (CompactionLayerProfile, bool) {
	for _, p := range r.profiles {
		if p.Order == n {
			return p, true
		}
	}
	return CompactionLayerProfile{}, false
}

// EnabledByDefaultLayers returns profiles where DefaultEnabled is true.
// Under default configuration this is LayerBudgetReduction and LayerAutoCompact.
func (r *CompactionPipelineLayerRegistry) EnabledByDefaultLayers() []CompactionLayerProfile {
	var out []CompactionLayerProfile
	for _, p := range r.profiles {
		if p.DefaultEnabled {
			out = append(out, p)
		}
	}
	return out
}

// OptInLayers returns profiles where DefaultEnabled is false (gated by feature flags).
// Under default configuration this is Snip, Microcompact, and ContextCollapse.
func (r *CompactionPipelineLayerRegistry) OptInLayers() []CompactionLayerProfile {
	var out []CompactionLayerProfile
	for _, p := range r.profiles {
		if !p.DefaultEnabled {
			out = append(out, p)
		}
	}
	return out
}

// LayersByAggressiveness returns all profiles whose Aggressiveness matches a.
// Results are returned in canonical pipeline order.
func (r *CompactionPipelineLayerRegistry) LayersByAggressiveness(a CompactionAggressiveness) []CompactionLayerProfile {
	var out []CompactionLayerProfile
	for _, p := range r.profiles {
		if p.Aggressiveness == a {
			out = append(out, p)
		}
	}
	return out
}

// IsValidLayer reports whether l is a recognised CompactionPipelineLayer value.
func (r *CompactionPipelineLayerRegistry) IsValidLayer(l CompactionPipelineLayer) bool {
	for _, p := range r.profiles {
		if p.Layer == l {
			return true
		}
	}
	return false
}

// TotalExpectedOutputReductionPct returns the sum of OutputSizeReductionPct across
// all five layers, representing the theoretical maximum context reduction when every
// layer is applied in sequence.
func (r *CompactionPipelineLayerRegistry) TotalExpectedOutputReductionPct() float64 {
	var total float64
	for _, p := range r.profiles {
		total += p.OutputSizeReductionPct
	}
	return total
}
