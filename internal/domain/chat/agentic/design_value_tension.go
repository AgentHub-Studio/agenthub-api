package agentic

// DesignValueTensionID is a slug identifier for one of the five Table 4 tensions
// (arXiv:2604.14228v1, §11.2 — "Design Value Tensions").
type DesignValueTensionID string

const (
	// TensionAuthoritySafety — approval fatigue vs. protection (93% fatigue rate).
	TensionAuthoritySafety DesignValueTensionID = "authority_safety"
	// TensionSafetyCapability — defense depth vs. reachable capability (>50 blocked subcommands).
	TensionSafetyCapability DesignValueTensionID = "safety_capability"
	// TensionAdaptabilitySafety — adaptive trust vs. prompt-injection attack surface.
	TensionAdaptabilitySafety DesignValueTensionID = "adaptability_safety"
	// TensionCapabilityAdaptability — proactivity vs. unexpected context disruption.
	TensionCapabilityAdaptability DesignValueTensionID = "capability_adaptability"
	// TensionCapabilityReliability — task-completion velocity vs. cross-session coherence.
	TensionCapabilityReliability DesignValueTensionID = "capability_reliability"
)

// DesignValueTensionProfile is the structured profile of one Table 4 tension.
type DesignValueTensionProfile struct {
	ID              DesignValueTensionID
	Value1          DesignValue
	Value2          DesignValue
	TensionLabel    string // short human-readable label from Table 4
	EvidenceSummary string // empirical evidence note cited in §11.2
}

var designValueTensionProfiles = map[DesignValueTensionID]DesignValueTensionProfile{
	TensionAuthoritySafety: {
		ID:              TensionAuthoritySafety,
		Value1:          DesignValueHumanAuthority,
		Value2:          DesignValueSafety,
		TensionLabel:    "Approval fatigue vs. protection",
		EvidenceSummary: "93% approval fatigue rate — frequent confirmations erode oversight quality while deny-first safety requires them.",
	},
	TensionSafetyCapability: {
		ID:              TensionSafetyCapability,
		Value1:          DesignValueSafety,
		Value2:          DesignValueCapability,
		TensionLabel:    "Performance vs. defense depth",
		EvidenceSummary: "Deny-first defaults block >50 subcommands that bypass deny checks; safe defaults directly reduce reachable capability.",
	},
	TensionAdaptabilitySafety: {
		ID:              TensionAdaptabilitySafety,
		Value1:          DesignValueAdaptability,
		Value2:          DesignValueSafety,
		TensionLabel:    "Extensibility vs. attack surface",
		EvidenceSummary: "Pre-trust window exploits — adaptive trust grants enable prompt-injection attacks before deny rules engage.",
	},
	TensionCapabilityAdaptability: {
		ID:              TensionCapabilityAdaptability,
		Value1:          DesignValueCapability,
		Value2:          DesignValueAdaptability,
		TensionLabel:    "Proactivity vs. disruption",
		EvidenceSummary: "Proactive autonomous actions increase task completion but risk disrupting user context or session state unexpectedly.",
	},
	TensionCapabilityReliability: {
		ID:              TensionCapabilityReliability,
		Value1:          DesignValueCapability,
		Value2:          DesignValueReliability,
		TensionLabel:    "Velocity vs. coherence",
		EvidenceSummary: "Maximising autonomous task completion degrades cross-session consistency and predictable behaviour.",
	},
}

// DesignValueTensionSequence is the canonical Table 4 row ordering.
var DesignValueTensionSequence = []DesignValueTensionID{
	TensionAuthoritySafety,
	TensionSafetyCapability,
	TensionAdaptabilitySafety,
	TensionCapabilityAdaptability,
	TensionCapabilityReliability,
}

// DesignValueTensionRegistry provides structured access to the five Table 4 tensions.
type DesignValueTensionRegistry struct{}

// NewDesignValueTensionRegistry returns a ready-to-use registry.
func NewDesignValueTensionRegistry() *DesignValueTensionRegistry {
	return &DesignValueTensionRegistry{}
}

// Profile returns the tension profile for the given ID. Returns false if unknown.
func (r *DesignValueTensionRegistry) Profile(id DesignValueTensionID) (DesignValueTensionProfile, bool) {
	p, ok := designValueTensionProfiles[id]
	return p, ok
}

// AllTensions returns all five profiles in Table 4 row order.
func (r *DesignValueTensionRegistry) AllTensions() []DesignValueTensionProfile {
	result := make([]DesignValueTensionProfile, len(DesignValueTensionSequence))
	for i, id := range DesignValueTensionSequence {
		result[i] = designValueTensionProfiles[id]
	}
	return result
}

// TensionsInvolvingValue returns tensions where value appears as Value1 or Value2.
func (r *DesignValueTensionRegistry) TensionsInvolvingValue(v DesignValue) []DesignValueTensionProfile {
	var result []DesignValueTensionProfile
	for _, id := range DesignValueTensionSequence {
		p := designValueTensionProfiles[id]
		if p.Value1 == v || p.Value2 == v {
			result = append(result, p)
		}
	}
	return result
}

// IsValidTension returns true if the ID is one of the five Table 4 tensions.
func (r *DesignValueTensionRegistry) IsValidTension(id DesignValueTensionID) bool {
	_, ok := designValueTensionProfiles[id]
	return ok
}
