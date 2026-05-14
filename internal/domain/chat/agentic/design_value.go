package agentic

import (
	"errors"
	"sort"
)

// DesignValue represents one of the five recurring design values for
// production coding agents described in PDF arXiv:2604.14228v1 §13.
//
// The five values are in tension with each other. A DesignValueWeightVector
// assigns priorities so that a caller can score competing trade-offs and
// decide which value wins in a specific context.
type DesignValue string

const (
	// DesignValueHumanAuthority — human decision authority; agent yields
	// ambiguous or high-stakes decisions to a human review step.
	DesignValueHumanAuthority DesignValue = "human_authority"

	// DesignValueSafety — deny-first, reversibility-weighted risk;
	// prefer cautious actions even at the cost of capability.
	DesignValueSafety DesignValue = "safety"

	// DesignValueReliability — reliable execution; predictable, auditable,
	// coherent behaviour across turns (avoid silent failures).
	DesignValueReliability DesignValue = "reliability"

	// DesignValueCapability — capability amplification; maximise task
	// completion and the agent's ability to act autonomously.
	DesignValueCapability DesignValue = "capability"

	// DesignValueAdaptability — contextual adaptability; tune behaviour to
	// operator/user context (tenant config, permission mode, session state).
	DesignValueAdaptability DesignValue = "adaptability"
)

// AllDesignValues is the canonical ordered list.
var AllDesignValues = []DesignValue{
	DesignValueHumanAuthority,
	DesignValueSafety,
	DesignValueReliability,
	DesignValueCapability,
	DesignValueAdaptability,
}

// DesignValueTension represents a trade-off between two design values.
// From Table 4 (arXiv:2604.14228v1, p. 34): concrete tensions observed
// in production coding-agent deployments.
type DesignValueTension struct {
	Value1      DesignValue
	Value2      DesignValue
	Description string
}

// KnownValueTensions enumerates the tensions from Table 4.
var KnownValueTensions = []DesignValueTension{
	{
		Value1:      DesignValueHumanAuthority,
		Value2:      DesignValueSafety,
		Description: "Approval fatigue (93% fatigue rate) vs. deny-first safety — frequent confirms erode human oversight quality.",
	},
	{
		Value1:      DesignValueSafety,
		Value2:      DesignValueCapability,
		Description: "Deny-first safety limits >50 subcommands that bypass deny checks — safe defaults reduce reachable capability.",
	},
	{
		Value1:      DesignValueAdaptability,
		Value2:      DesignValueSafety,
		Description: "Pre-trust window exploits — adaptive trust grants enable prompt-injection attacks before deny rules engage.",
	},
	{
		Value1:      DesignValueCapability,
		Value2:      DesignValueReliability,
		Description: "Velocity vs. coherence — maximising autonomous task completion degrades cross-session consistency.",
	},
}

// DesignValueWeightVector assigns a priority weight (0.0–1.0) to each
// design value. Weights need not sum to 1.0; Normalize() makes them do so.
type DesignValueWeightVector struct {
	weights map[DesignValue]float64
}

// NewDesignValueWeightVector creates a vector with all weights at 0.
func NewDesignValueWeightVector() *DesignValueWeightVector {
	w := make(map[DesignValue]float64, len(AllDesignValues))
	for _, v := range AllDesignValues {
		w[v] = 0
	}
	return &DesignValueWeightVector{weights: w}
}

// DefaultBalancedWeightVector returns equal weights (0.2 each).
func DefaultBalancedWeightVector() *DesignValueWeightVector {
	dv := NewDesignValueWeightVector()
	for _, v := range AllDesignValues {
		dv.weights[v] = 0.2
	}
	return dv
}

// SafetyFirstWeightVector returns a vector prioritising safety over capability.
func SafetyFirstWeightVector() *DesignValueWeightVector {
	dv := NewDesignValueWeightVector()
	dv.weights[DesignValueSafety] = 0.40
	dv.weights[DesignValueHumanAuthority] = 0.30
	dv.weights[DesignValueReliability] = 0.15
	dv.weights[DesignValueAdaptability] = 0.10
	dv.weights[DesignValueCapability] = 0.05
	return dv
}

// CapabilityFirstWeightVector returns a vector prioritising capability.
func CapabilityFirstWeightVector() *DesignValueWeightVector {
	dv := NewDesignValueWeightVector()
	dv.weights[DesignValueCapability] = 0.40
	dv.weights[DesignValueAdaptability] = 0.25
	dv.weights[DesignValueReliability] = 0.20
	dv.weights[DesignValueSafety] = 0.10
	dv.weights[DesignValueHumanAuthority] = 0.05
	return dv
}

// Set assigns a weight to a design value. Returns an error if the value
// is unknown or the weight is outside [0, 1].
func (dv *DesignValueWeightVector) Set(value DesignValue, weight float64) error {
	if weight < 0 || weight > 1 {
		return errors.New("design_value: weight must be in [0, 1]")
	}
	if _, ok := dv.weights[value]; !ok {
		return errors.New("design_value: unknown design value")
	}
	dv.weights[value] = weight
	return nil
}

// Get returns the weight for a design value.
func (dv *DesignValueWeightVector) Get(value DesignValue) float64 {
	return dv.weights[value]
}

// Dominant returns the highest-weighted design value. Ties broken by
// canonical order (AllDesignValues).
func (dv *DesignValueWeightVector) Dominant() DesignValue {
	var best DesignValue
	bestW := -1.0
	for _, v := range AllDesignValues {
		w := dv.weights[v]
		if w > bestW {
			bestW = w
			best = v
		}
	}
	return best
}

// Normalize scales all weights so that they sum to 1.0.
// If all weights are 0, it sets equal weights.
func (dv *DesignValueWeightVector) Normalize() {
	total := 0.0
	for _, w := range dv.weights {
		total += w
	}
	if total == 0 {
		eq := 1.0 / float64(len(AllDesignValues))
		for _, v := range AllDesignValues {
			dv.weights[v] = eq
		}
		return
	}
	for v, w := range dv.weights {
		dv.weights[v] = w / total
	}
}

// Sum returns the total of all weights.
func (dv *DesignValueWeightVector) Sum() float64 {
	total := 0.0
	for _, w := range dv.weights {
		total += w
	}
	return total
}

// RankedTension pairs a tension with a combined priority score.
type RankedTension struct {
	Tension DesignValueTension
	Score   float64 // sum of weights for both values in the tension
}

// RankTensions scores each tension by summing the weights of its two values.
// Higher score = both values are high priority = harder trade-off.
// Returns tensions sorted descending by score.
func (dv *DesignValueWeightVector) RankTensions(tensions []DesignValueTension) []RankedTension {
	ranked := make([]RankedTension, len(tensions))
	for i, t := range tensions {
		ranked[i] = RankedTension{
			Tension: t,
			Score:   dv.weights[t.Value1] + dv.weights[t.Value2],
		}
	}
	sort.SliceStable(ranked, func(a, b int) bool {
		return ranked[a].Score > ranked[b].Score
	})
	return ranked
}

// ScoreTension returns the combined weight of the two values in a single tension.
func (dv *DesignValueWeightVector) ScoreTension(t DesignValueTension) float64 {
	return dv.weights[t.Value1] + dv.weights[t.Value2]
}
