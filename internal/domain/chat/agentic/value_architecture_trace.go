package agentic

// ValueArchitectureTrace models the §2.3 "From Values to Architecture" mapping
// from arXiv:2604.14228v1.  Each human design value traces through a set of
// design principles to concrete architectural decisions and the components that
// implement them.  The §2.4 cross-cutting evaluative lens (Long-term Capability
// Preservation) is modelled separately as EvaluativeLens.
//
// The five traces are sourced verbatim from §2.3 bullet list (page 5, PDF):
//
//   - Human Decision Authority → deny-first evaluation, graduated trust spectrum,
//     append-only state, externalized programmable policy, values-over-rules (§5–7, 9)
//   - Safety, Security, and Privacy → defense in depth, deny-first defaults,
//     reversibility-weighted assessment, externalized policy, isolated subagent
//     boundaries (§5, 8)
//   - Reliable Execution → context-as-scarce-resource, append-only durable state,
//     graceful recovery, isolated subagent boundaries, defense in depth (§4, 7–9)
//   - Capability Amplification → minimal scaffolding, composable extensibility,
//     reversibility-weighted risk, context management, graceful recovery (§4–6)
//   - Contextual Adaptability → transparent file-based config, composable
//     extensibility, graduated trust spectrum, externalized programmable policy (§5–7)

// ArchitecturalDecision describes one concrete implementation choice that a
// design principle motivates.  The ComponentRef field names the source-level
// artefact (TypeScript or Go identifier) from the Claude Code implementation
// described in the PDF.
type ArchitecturalDecision struct {
	// Label is a short human-readable name for the decision.
	Label string
	// Description explains the design choice and why the value motivates it.
	Description string
	// PrincipleID is the Table 1 design principle that grounds this decision.
	PrincipleID DesignPrincipleID
	// ComponentRef is the canonical source-level artefact that implements
	// this decision (e.g. "permissions.ts", "queryLoop()", "sessionStorage.ts").
	ComponentRef string
	// PDFSections lists the PDF section numbers where this decision is analysed.
	PDFSections []string
}

// ValueArchitectureTrace is the full §2.3 mapping for one design value:
// the value motivates a set of principles, which each motivate architectural
// decisions.
type ValueArchitectureTrace struct {
	// Value is the design value this trace is rooted in.
	Value DesignValue
	// MotivatedPrinciples lists the Table 1 principle IDs that this value
	// primarily drives (may overlap with other values' traces).
	MotivatedPrinciples []DesignPrincipleID
	// Decisions is the ordered list of concrete architectural decisions that
	// follow from the value through its principles.
	Decisions []ArchitecturalDecision
}

// EvaluativeLensID identifies a cross-cutting evaluative lens applied on top
// of the five primary values (§2.4).
type EvaluativeLensID string

const (
	// LensLongTermCapabilityPreservation is the §2.4 evaluative lens that
	// asks whether short-term convenience comes at the cost of long-term human
	// understanding, codebase coherence, and developer pipeline health.
	LensLongTermCapabilityPreservation EvaluativeLensID = "long_term_capability_preservation"
)

// EvaluativeLens models a §2.4 cross-cutting concern that is applied across
// all five values rather than driving architectural choices in its own right.
type EvaluativeLens struct {
	ID          EvaluativeLensID
	Label       string
	Description string
	// EvaluatedValues are the primary values this lens scrutinises.
	EvaluatedValues []DesignValue
	// PDFSection is the section where the lens is introduced.
	PDFSection string
	// EmpiricalBasis summarises the empirical evidence cited for the concern.
	EmpiricalBasis string
}

// AllEvaluativeLenses is the canonical set of cross-cutting lenses (§2.4).
var AllEvaluativeLenses = []EvaluativeLens{
	{
		ID:    LensLongTermCapabilityPreservation,
		Label: "Long-term Capability Preservation",
		Description: "Whether the architecture preserves long-term human capability. " +
			"A question applied across all five values in §11: does short-term amplification " +
			"come at the cost of long-term human understanding, codebase coherence, and " +
			"the developer pipeline?",
		EvaluatedValues: []DesignValue{
			DesignValueHumanAuthority,
			DesignValueSafety,
			DesignValueReliability,
			DesignValueCapability,
			DesignValueAdaptability,
		},
		PDFSection: "2.4",
		EmpiricalBasis: "Anthropic's internal survey of 132 engineers (Huang et al., 2025) documents a " +
			"'paradox of supervision' in which overreliance on AI risks atrophying supervisory skills. " +
			"Independent research (Shen and Tamkin, 2026) finds developers score 17% lower on " +
			"comprehension tests in AI-assisted conditions.",
	},
}

// valueArchitectureTraces is the §2.3 mapping keyed by DesignValue.
var valueArchitectureTraces = map[DesignValue]ValueArchitectureTrace{
	DesignValueHumanAuthority: {
		Value: DesignValueHumanAuthority,
		MotivatedPrinciples: []DesignPrincipleID{
			PrincipleDenyFirstHumanEscalation,
			PrincipleGraduatedTrustSpectrum,
			PrincipleAppendOnlyDurableState,
			PrincipleExternalizedProgrammablePolicy,
			PrincipleValuesOverRules,
		},
		Decisions: []ArchitecturalDecision{
			{
				Label:        "Deny-first permission evaluation",
				Description:  "Unrecognised actions are denied and escalated to the human rather than silently allowed, preserving the human's power to choose.",
				PrincipleID:  PrincipleDenyFirstHumanEscalation,
				ComponentRef: "permissions.ts",
				PDFSections:  []string{"5", "8", "9"},
			},
			{
				Label:        "Graduated trust spectrum traversal",
				Description:  "Users start at a low-trust mode and accumulate trust over sessions; the system tracks their traversal rather than assigning a fixed level.",
				PrincipleID:  PrincipleGraduatedTrustSpectrum,
				ComponentRef: "permissions.ts",
				PDFSections:  []string{"5"},
			},
			{
				Label:        "Append-only auditable history",
				Description:  "Session transcripts are append-only JSON so humans can audit every action after the fact even if they were not present during execution.",
				PrincipleID:  PrincipleAppendOnlyDurableState,
				ComponentRef: "sessionStorage.ts",
				PDFSections:  []string{"4", "9"},
			},
			{
				Label:        "Externalized lifecycle hooks",
				Description:  "Policy is expressed in externalized config files and lifecycle hooks (PreToolUse, PostToolUse), not hard-coded, so operators can override defaults.",
				PrincipleID:  PrincipleExternalizedProgrammablePolicy,
				ComponentRef: "types/hooks.ts",
				PDFSections:  []string{"5", "6"},
			},
			{
				Label:        "Values-based contextual judgment",
				Description:  "The agent applies contextual judgment backed by deterministic guardrails rather than rigid rule matching, so authority is exercised through good values not blind rules.",
				PrincipleID:  PrincipleValuesOverRules,
				ComponentRef: "yoloClassifier.ts",
				PDFSections:  []string{"3", "5", "7"},
			},
		},
	},

	DesignValueSafety: {
		Value: DesignValueSafety,
		MotivatedPrinciples: []DesignPrincipleID{
			PrincipleDefenseInDepthLayered,
			PrincipleDenyFirstHumanEscalation,
			PrincipleReversibilityWeightedRisk,
			PrincipleExternalizedProgrammablePolicy,
			PrincipleIsolatedSubagentBoundaries,
		},
		Decisions: []ArchitecturalDecision{
			{
				Label:        "Five-layer defense in depth",
				Description:  "Permission rules, PreToolUse hooks, ML auto-mode classifier, and optional shell sandbox apply in parallel — any one layer can block an action.",
				PrincipleID:  PrincipleDefenseInDepthLayered,
				ComponentRef: "permissions.ts + shouldUseSandbox.ts",
				PDFSections:  []string{"3", "5"},
			},
			{
				Label:        "Deny-first defaults",
				Description:  "Deny rules override ask rules override allow rules; unrecognised actions are escalated rather than allowed, protecting against prompt injection.",
				PrincipleID:  PrincipleDenyFirstHumanEscalation,
				ComponentRef: "permissions.ts",
				PDFSections:  []string{"5"},
			},
			{
				Label:        "Reversibility-weighted oversight",
				Description:  "Read-only and reversible operations receive lighter oversight; destructive or irreversible operations require explicit approval.",
				PrincipleID:  PrincipleReversibilityWeightedRisk,
				ComponentRef: "permissions.ts",
				PDFSections:  []string{"4", "5", "8"},
			},
			{
				Label:        "Externalized policy for security configs",
				Description:  "Security posture is configurable via operator-managed config files, enabling rapid patching without code changes.",
				PrincipleID:  PrincipleExternalizedProgrammablePolicy,
				ComponentRef: "types/hooks.ts",
				PDFSections:  []string{"5", "6"},
			},
			{
				Label:        "Isolated subagent context and permissions",
				Description:  "Subagents spawn with an isolated context window and do not inherit the parent's permission state, containing potential compromise.",
				PrincipleID:  PrincipleIsolatedSubagentBoundaries,
				ComponentRef: "AgentTool.tsx + runAgent.ts",
				PDFSections:  []string{"8"},
			},
		},
	},

	DesignValueReliability: {
		Value: DesignValueReliability,
		MotivatedPrinciples: []DesignPrincipleID{
			PrincipleContextAsScarceResource,
			PrincipleAppendOnlyDurableState,
			PrincipleGracefulRecoveryResilience,
			PrincipleIsolatedSubagentBoundaries,
			PrincipleDefenseInDepthLayered,
		},
		Decisions: []ArchitecturalDecision{
			{
				Label:        "Five-layer compaction pipeline",
				Description:  "Budget reduction → snip → microcompact → context collapse → auto-compact execute sequentially before every model call, preserving coherence across context window boundaries.",
				PrincipleID:  PrincipleContextAsScarceResource,
				ComponentRef: "query.ts:365-453",
				PDFSections:  []string{"4", "6", "7", "8"},
			},
			{
				Label:        "Append-only session transcripts",
				Description:  "Session state is stored as append-only JSONL, enabling resume, fork, and rewind without loss of prior context.",
				PrincipleID:  PrincipleAppendOnlyDurableState,
				ComponentRef: "sessionStorage.ts + history.ts",
				PDFSections:  []string{"4", "9"},
			},
			{
				Label:        "Graceful error recovery",
				Description:  "Errors are silently recovered where possible; human attention is reserved for unrecoverable situations via structured error escalation.",
				PrincipleID:  PrincipleGracefulRecoveryResilience,
				ComponentRef: "query.ts",
				PDFSections:  []string{"4", "5"},
			},
			{
				Label:        "Summary-only subagent returns",
				Description:  "Subagents return only a summary to the parent, not their full transcript, preventing context explosion that would degrade reliability.",
				PrincipleID:  PrincipleIsolatedSubagentBoundaries,
				ComponentRef: "runAgent.ts",
				PDFSections:  []string{"7", "8"},
			},
			{
				Label:        "Defense in depth for reliable execution",
				Description:  "Layered safety mechanisms ensure that a single component failure does not produce incorrect or harmful outputs silently.",
				PrincipleID:  PrincipleDefenseInDepthLayered,
				ComponentRef: "permissions.ts",
				PDFSections:  []string{"3", "5"},
			},
		},
	},

	DesignValueCapability: {
		Value: DesignValueCapability,
		MotivatedPrinciples: []DesignPrincipleID{
			PrincipleMinimalScaffoldingMaximalHarness,
			PrincipleComposableMultiMechanism,
			PrincipleReversibilityWeightedRisk,
			PrincipleContextAsScarceResource,
			PrincipleGracefulRecoveryResilience,
		},
		Decisions: []ArchitecturalDecision{
			{
				Label:        "Thin reasoning layer (1.6% AI logic)",
				Description:  "Only ~1.6% of the codebase is AI decision logic; 98.4% is operational infrastructure. The model reasons freely inside a rich harness rather than constrained scaffolding.",
				PrincipleID:  PrincipleMinimalScaffoldingMaximalHarness,
				ComponentRef: "query.ts",
				PDFSections:  []string{"3", "4"},
			},
			{
				Label:        "Composable four-mechanism extension surface",
				Description:  "MCP, plugins, skills, and hooks each operate at different context costs, allowing capability expansion without one-size-fits-all trade-offs.",
				PrincipleID:  PrincipleComposableMultiMechanism,
				ComponentRef: "tools.ts + types/hooks.ts",
				PDFSections:  []string{"6"},
			},
			{
				Label:        "Lighter oversight for reversible operations",
				Description:  "Read-only and reversible tool calls receive lighter permission checks, increasing throughput for safe operations.",
				PrincipleID:  PrincipleReversibilityWeightedRisk,
				ComponentRef: "permissions.ts",
				PDFSections:  []string{"4", "5", "8"},
			},
			{
				Label:        "Deferred tool schemas for context economy",
				Description:  "Tool schemas are loaded lazily to minimise context pressure, preserving the window for high-value task content.",
				PrincipleID:  PrincipleContextAsScarceResource,
				ComponentRef: "tools.ts",
				PDFSections:  []string{"4", "6", "7"},
			},
			{
				Label:        "Silent recovery to maximise task completion",
				Description:  "Transient errors are recovered silently; capability is not sacrificed to surface every minor fault.",
				PrincipleID:  PrincipleGracefulRecoveryResilience,
				ComponentRef: "query.ts",
				PDFSections:  []string{"4", "5"},
			},
		},
	},

	DesignValueAdaptability: {
		Value: DesignValueAdaptability,
		MotivatedPrinciples: []DesignPrincipleID{
			PrincipleTransparentFileBased,
			PrincipleComposableMultiMechanism,
			PrincipleGraduatedTrustSpectrum,
			PrincipleExternalizedProgrammablePolicy,
		},
		Decisions: []ArchitecturalDecision{
			{
				Label:        "File-based configuration and memory",
				Description:  "CLAUDE.md and memory files are user-visible, version-controllable plain text so operators can inspect and adjust agent behaviour without opaque database changes.",
				PrincipleID:  PrincipleTransparentFileBased,
				ComponentRef: "CLAUDE.md + memory files",
				PDFSections:  []string{"7"},
			},
			{
				Label:        "Four-layer composable extensibility",
				Description:  "MCP, plugins, skills, and hooks allow context-specific capability expansion at different cost tiers, adapting to each project's needs.",
				PrincipleID:  PrincipleComposableMultiMechanism,
				ComponentRef: "tools.ts + types/hooks.ts",
				PDFSections:  []string{"6"},
			},
			{
				Label:        "Session-persistent trust accumulation",
				Description:  "Auto-approve rates evolve from ~20% at session 50 to >40% at session 750, reflecting co-constructed trust rather than fixed trust states.",
				PrincipleID:  PrincipleGraduatedTrustSpectrum,
				ComponentRef: "permissions.ts",
				PDFSections:  []string{"5"},
			},
			{
				Label:        "Operator-managed lifecycle hooks",
				Description:  "PreToolUse, PostToolUse and Stop hooks allow operators to inject project-specific policy without forking the agent codebase.",
				PrincipleID:  PrincipleExternalizedProgrammablePolicy,
				ComponentRef: "types/hooks.ts",
				PDFSections:  []string{"5", "6"},
			},
		},
	},
}

// ValueArchitectureTraceRegistry provides §2.3 + §2.4 lookup.
type ValueArchitectureTraceRegistry struct{}

// NewValueArchitectureTraceRegistry returns a ready-to-use registry.
func NewValueArchitectureTraceRegistry() *ValueArchitectureTraceRegistry {
	return &ValueArchitectureTraceRegistry{}
}

// TraceForValue returns the full §2.3 trace for the given design value.
// Returns false if the value is unknown.
func (r *ValueArchitectureTraceRegistry) TraceForValue(v DesignValue) (ValueArchitectureTrace, bool) {
	t, ok := valueArchitectureTraces[v]
	return t, ok
}

// AllTraces returns all five §2.3 traces in AllDesignValues canonical order.
func (r *ValueArchitectureTraceRegistry) AllTraces() []ValueArchitectureTrace {
	result := make([]ValueArchitectureTrace, 0, len(AllDesignValues))
	for _, v := range AllDesignValues {
		if t, ok := valueArchitectureTraces[v]; ok {
			result = append(result, t)
		}
	}
	return result
}

// DecisionsForPrinciple returns all architectural decisions across all five
// traces that are grounded in the given principle ID.
func (r *ValueArchitectureTraceRegistry) DecisionsForPrinciple(id DesignPrincipleID) []ArchitecturalDecision {
	var result []ArchitecturalDecision
	for _, v := range AllDesignValues {
		t := valueArchitectureTraces[v]
		for _, d := range t.Decisions {
			if d.PrincipleID == id {
				result = append(result, d)
			}
		}
	}
	return result
}

// DecisionsForValue returns the architectural decisions for the given design value.
func (r *ValueArchitectureTraceRegistry) DecisionsForValue(v DesignValue) []ArchitecturalDecision {
	t, ok := valueArchitectureTraces[v]
	if !ok {
		return nil
	}
	return t.Decisions
}

// EvaluativeLenses returns all §2.4 cross-cutting evaluative lenses.
func (r *ValueArchitectureTraceRegistry) EvaluativeLenses() []EvaluativeLens {
	return AllEvaluativeLenses
}

// LensForID returns the evaluative lens for the given ID.
// Returns false if unknown.
func (r *ValueArchitectureTraceRegistry) LensForID(id EvaluativeLensID) (EvaluativeLens, bool) {
	for _, l := range AllEvaluativeLenses {
		if l.ID == id {
			return l, true
		}
	}
	return EvaluativeLens{}, false
}

// ArchitecturalAbsences returns a description of what the architecture
// deliberately does NOT do (from the §2.3 "These mappings also reveal…" paragraph).
// These absences are as informative as the presences.
func (r *ValueArchitectureTraceRegistry) ArchitecturalAbsences() []string {
	return []string{
		"Does not impose explicit planning graphs on the model's reasoning",
		"Does not provide a single unified extension mechanism",
		"Does not restore all session-scoped trust-related state across resume",
	}
}
