package agentic

// DesignPrincipleID is a slug identifier for one of the thirteen design principles
// from arXiv:2604.14228v1 Table 1 (§2.2 — "Design Principles").
type DesignPrincipleID string

const (
	PrincipleDenyFirstHumanEscalation       DesignPrincipleID = "deny_first_human_escalation"
	PrincipleGraduatedTrustSpectrum         DesignPrincipleID = "graduated_trust_spectrum"
	PrincipleDefenseInDepthLayered          DesignPrincipleID = "defense_in_depth_layered"
	PrincipleExternalizedProgrammablePolicy DesignPrincipleID = "externalized_programmable_policy"
	PrincipleContextAsScarceResource        DesignPrincipleID = "context_as_scarce_resource"
	PrincipleAppendOnlyDurableState         DesignPrincipleID = "append_only_durable_state"
	PrincipleMinimalScaffoldingMaximalHarness DesignPrincipleID = "minimal_scaffolding_maximal_harness"
	PrincipleValuesOverRules                DesignPrincipleID = "values_over_rules"
	PrincipleComposableMultiMechanism       DesignPrincipleID = "composable_multi_mechanism"
	PrincipleReversibilityWeightedRisk      DesignPrincipleID = "reversibility_weighted_risk"
	PrincipleTransparentFileBased           DesignPrincipleID = "transparent_file_based"
	PrincipleIsolatedSubagentBoundaries     DesignPrincipleID = "isolated_subagent_boundaries"
	PrincipleGracefulRecoveryResilience     DesignPrincipleID = "graceful_recovery_resilience"
)

// DesignPrincipleProfile holds the Table 1 data for one design principle.
type DesignPrincipleProfile struct {
	ID              DesignPrincipleID
	Label           string        // human-readable name from Table 1
	ValuesServed    []DesignValue // the design values this principle operationalises
	DesignQuestion  string        // the architectural question the principle answers
	ReferencedSections []string  // PDF section numbers where this principle is traced
}

var designPrincipleProfiles = map[DesignPrincipleID]DesignPrincipleProfile{
	PrincipleDenyFirstHumanEscalation: {
		ID:    PrincipleDenyFirstHumanEscalation,
		Label: "Deny-first with human escalation",
		ValuesServed: []DesignValue{DesignValueHumanAuthority, DesignValueSafety},
		DesignQuestion:  "Should unrecognized actions be allowed, blocked, or escalated to the human?",
		ReferencedSections: []string{"5", "8", "9"},
	},
	PrincipleGraduatedTrustSpectrum: {
		ID:    PrincipleGraduatedTrustSpectrum,
		Label: "Graduated trust spectrum",
		ValuesServed: []DesignValue{DesignValueHumanAuthority, DesignValueAdaptability},
		DesignQuestion:  "Fixed permission level, or a spectrum users traverse over time?",
		ReferencedSections: []string{"5"},
	},
	PrincipleDefenseInDepthLayered: {
		ID:    PrincipleDefenseInDepthLayered,
		Label: "Defense in depth with layered mechanisms",
		ValuesServed: []DesignValue{DesignValueSafety, DesignValueHumanAuthority, DesignValueReliability},
		DesignQuestion:  "Single safety boundary, or multiple overlapping ones using different techniques?",
		ReferencedSections: []string{"3", "5"},
	},
	PrincipleExternalizedProgrammablePolicy: {
		ID:    PrincipleExternalizedProgrammablePolicy,
		Label: "Externalized programmable policy",
		ValuesServed: []DesignValue{DesignValueSafety, DesignValueHumanAuthority, DesignValueAdaptability},
		DesignQuestion:  "Hardcoded policy, or externalized configs with lifecycle hooks?",
		ReferencedSections: []string{"5", "6"},
	},
	PrincipleContextAsScarceResource: {
		ID:    PrincipleContextAsScarceResource,
		Label: "Context as scarce resource with progressive management",
		ValuesServed: []DesignValue{DesignValueReliability, DesignValueCapability},
		DesignQuestion:  "What is the binding resource constraint, and how to manage it: single-pass truncation or graduated pipeline?",
		ReferencedSections: []string{"4", "6", "7", "8"},
	},
	PrincipleAppendOnlyDurableState: {
		ID:    PrincipleAppendOnlyDurableState,
		Label: "Append-only durable state",
		ValuesServed: []DesignValue{DesignValueReliability, DesignValueHumanAuthority},
		DesignQuestion:  "Mutable state, checkpoint snapshots, or append-only logs?",
		ReferencedSections: []string{"4", "9"},
	},
	PrincipleMinimalScaffoldingMaximalHarness: {
		ID:    PrincipleMinimalScaffoldingMaximalHarness,
		Label: "Minimal scaffolding, maximal operational harness",
		ValuesServed: []DesignValue{DesignValueCapability, DesignValueReliability},
		DesignQuestion:  "Invest in scaffolding-side reasoning, or operational infrastructure that lets the model reason freely?",
		ReferencedSections: []string{"3", "4"},
	},
	PrincipleValuesOverRules: {
		ID:    PrincipleValuesOverRules,
		Label: "Values over rules",
		ValuesServed: []DesignValue{DesignValueCapability, DesignValueHumanAuthority},
		DesignQuestion:  "Rigid decision procedures, or contextual judgment backed by deterministic guardrails?",
		ReferencedSections: []string{"3", "5", "7"},
	},
	PrincipleComposableMultiMechanism: {
		ID:    PrincipleComposableMultiMechanism,
		Label: "Composable multi-mechanism extensibility",
		ValuesServed: []DesignValue{DesignValueCapability, DesignValueAdaptability},
		DesignQuestion:  "One unified extension API, or layered mechanisms at different context costs?",
		ReferencedSections: []string{"6"},
	},
	PrincipleReversibilityWeightedRisk: {
		ID:    PrincipleReversibilityWeightedRisk,
		Label: "Reversibility-weighted risk assessment",
		ValuesServed: []DesignValue{DesignValueCapability, DesignValueSafety},
		DesignQuestion:  "Same oversight for all actions, or lighter for reversible and read-only ones?",
		ReferencedSections: []string{"4", "5", "8"},
	},
	PrincipleTransparentFileBased: {
		ID:    PrincipleTransparentFileBased,
		Label: "Transparent file-based configuration and memory",
		ValuesServed: []DesignValue{DesignValueAdaptability, DesignValueHumanAuthority},
		DesignQuestion:  "Opaque database, embedding-based retrieval, or user-visible version-controllable files?",
		ReferencedSections: []string{"7"},
	},
	PrincipleIsolatedSubagentBoundaries: {
		ID:    PrincipleIsolatedSubagentBoundaries,
		Label: "Isolated subagent boundaries",
		ValuesServed: []DesignValue{DesignValueReliability, DesignValueSafety, DesignValueCapability},
		DesignQuestion:  "Subagents share the parent's context and permissions, or operate in isolation?",
		ReferencedSections: []string{"8"},
	},
	PrincipleGracefulRecoveryResilience: {
		ID:    PrincipleGracefulRecoveryResilience,
		Label: "Graceful recovery and resilience",
		ValuesServed: []DesignValue{DesignValueReliability, DesignValueCapability},
		DesignQuestion:  "Fail hard on errors, or silently recover and reserve human attention for unrecoverable situations?",
		ReferencedSections: []string{"4", "5"},
	},
}

// DesignPrincipleSequence is the canonical Table 1 row ordering.
var DesignPrincipleSequence = []DesignPrincipleID{
	PrincipleDenyFirstHumanEscalation,
	PrincipleGraduatedTrustSpectrum,
	PrincipleDefenseInDepthLayered,
	PrincipleExternalizedProgrammablePolicy,
	PrincipleContextAsScarceResource,
	PrincipleAppendOnlyDurableState,
	PrincipleMinimalScaffoldingMaximalHarness,
	PrincipleValuesOverRules,
	PrincipleComposableMultiMechanism,
	PrincipleReversibilityWeightedRisk,
	PrincipleTransparentFileBased,
	PrincipleIsolatedSubagentBoundaries,
	PrincipleGracefulRecoveryResilience,
}

// DesignPrincipleRegistry provides structured access to the thirteen Table 1 principles.
type DesignPrincipleRegistry struct{}

// NewDesignPrincipleRegistry returns a ready-to-use registry.
func NewDesignPrincipleRegistry() *DesignPrincipleRegistry {
	return &DesignPrincipleRegistry{}
}

// Profile returns the Table 1 profile for the given principle ID. Returns false if unknown.
func (r *DesignPrincipleRegistry) Profile(id DesignPrincipleID) (DesignPrincipleProfile, bool) {
	p, ok := designPrincipleProfiles[id]
	return p, ok
}

// AllPrinciples returns all thirteen profiles in Table 1 row order.
func (r *DesignPrincipleRegistry) AllPrinciples() []DesignPrincipleProfile {
	result := make([]DesignPrincipleProfile, len(DesignPrincipleSequence))
	for i, id := range DesignPrincipleSequence {
		result[i] = designPrincipleProfiles[id]
	}
	return result
}

// PrinciplesServingValue returns all principles where value appears in ValuesServed.
func (r *DesignPrincipleRegistry) PrinciplesServingValue(v DesignValue) []DesignPrincipleProfile {
	var result []DesignPrincipleProfile
	for _, id := range DesignPrincipleSequence {
		p := designPrincipleProfiles[id]
		for _, sv := range p.ValuesServed {
			if sv == v {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

// IsValidPrinciple returns true if the ID is one of the thirteen Table 1 principles.
func (r *DesignPrincipleRegistry) IsValidPrinciple(id DesignPrincipleID) bool {
	_, ok := designPrincipleProfiles[id]
	return ok
}
