package agentic

// emerging_direction_registry.go — FEAT-044 — §11.6 Emerging Directions
//
// arXiv:2604.14228v1, §11.6 "Emerging Directions" (pages 31–32).
//
// §11.6 identifies five named directions in which the tightly-coupled Claude Code
// architecture is expected to evolve: architectural decoupling of agent components,
// memory as a first-class subsystem, observability and silent-failure closure,
// governance constraints from external regulation, and proactive (KAIROS-style)
// architectures.  Each direction is grounded in specific empirical literature cited
// in the section and maps to one or more of the five human values from §2.1.

// EmergingDirectionID is a stable identifier for one §11.6 emerging direction.
type EmergingDirectionID string

const (
	// DirectionArchitecturalDecoupling — §11.6 "Architectural decoupling".
	// Managed Agents work (Martin et al., 2026) virtualises session, harness, and
	// sandbox into independently-replaceable interfaces, drawing an analogy to how
	// operating systems virtualised hardware.  Rajasekaran (2026) notes the space of
	// interesting harness combinations "doesn't shrink as models improve; it moves".
	DirectionArchitecturalDecoupling EmergingDirectionID = "architectural_decoupling"

	// DirectionMemoryFirstClass — §11.6 "Memory as a first-class subsystem".
	// Hu et al. (2025) argues agent memory is becoming a distinct cognitive substrate.
	// Three open frontiers: automated memory management, RL-driven memory, and
	// trustworthy memory (privacy, explainability, hallucination robustness).
	// Claude Code today exposes factual tier (CLAUDE.md, auto memory) and working
	// tier (conversation window); the experiential tier is the natural next step.
	DirectionMemoryFirstClass EmergingDirectionID = "memory_first_class"

	// DirectionObservabilityAndSilentFailure — §11.6 "Observability and silent failure".
	// Wade et al. (2026) estimates 78% of AI failures are invisible; LangChain (2026)
	// finds observability as the top barrier to production use (89% adoption gap) with
	// 52.4% offline evaluation adoption.  Closing the gap requires generator-evaluator
	// separation, sprint contracts, and post-hoc checks (Rajasekaran, 2026).
	DirectionObservabilityAndSilentFailure EmergingDirectionID = "observability_and_silent_failure"

	// DirectionGovernance — §11.6 "Governance".
	// External regulatory constraints: EU AI Act (fully applicable August 2026),
	// International AI Safety Report (Bengio et al., 2026), MIT AI Agent Index
	// (Staufer et al., 2026) — only 13.3% of indexed agentic systems publish
	// agent-specific safety cards.  Evolving copyright jurisprudence around
	// AI-generated code imposes external constraints on logging, transparency, and
	// human oversight.
	DirectionGovernance EmergingDirectionID = "governance"

	// DirectionProactiveArchitectures — §11.6 "Proactive architectures".
	// The feature-gated KAIROS system implements persistent background agents with
	// tick-based heartbeats: when no user messages are pending, the harness injects
	// periodic <tick> prompts and the model decides whether to act or sleep.
	// Terminal-focus awareness and economic throttling via SleepTool (each wake-up
	// costs an API call; prompt cache expires after five minutes) bind proactivity to
	// both user presence and token economics.  Addresses the 12–18% task-completion
	// uplift vs. preference-drop tension (Chen et al., 2025).
	DirectionProactiveArchitectures EmergingDirectionID = "proactive_architectures"
)

// EmergingDirectionCategory groups §11.6 directions by the primary design axis
// they affect.
type EmergingDirectionCategory string

const (
	// CategoryArchitecture — structural organisation of agent components.
	CategoryArchitecture EmergingDirectionCategory = "architecture"
	// CategoryMemory — persistence, accumulation, and retrieval of agent state.
	CategoryMemory EmergingDirectionCategory = "memory"
	// CategoryObservability — evaluation, monitoring, and failure surfacing.
	CategoryObservability EmergingDirectionCategory = "observability"
	// CategoryGovernance — external regulatory and policy constraints.
	CategoryGovernance EmergingDirectionCategory = "governance"
	// CategoryProactivity — agent-initiated actions in the absence of user messages.
	CategoryProactivity EmergingDirectionCategory = "proactivity"
)

// EmergingHorizon indicates how near the direction is to production deployments
// as assessed by §11.6.
type EmergingHorizon string

const (
	// HorizonNearTerm — direction already partially present in production builds.
	HorizonNearTerm EmergingHorizon = "near_term"
	// HorizonMidTerm — direction active in research literature; production path clear.
	HorizonMidTerm EmergingHorizon = "mid_term"
	// HorizonLongTerm — direction named but mechanism choices remain open.
	HorizonLongTerm EmergingHorizon = "long_term"
)

// CitedEvidence records one piece of empirical or analytical support cited by §11.6
// for a given direction.
type CitedEvidence struct {
	// Citation is the author-year key as it appears in §11.6.
	Citation string
	// Finding summarises the cited claim in one sentence.
	Finding string
	// IsEmpiricalStudy is true when the source presents original measurements.
	IsEmpiricalStudy bool
}

// RelatedDesignPrinciple names one of the thirteen §2.2 design principles whose
// tension or resolution motivates the direction.
type RelatedDesignPrinciple string

// EmergingDirectionProfile is the full structured profile for one §11.6 direction.
type EmergingDirectionProfile struct {
	// ID is the stable identifier.
	ID EmergingDirectionID
	// Name is the human-readable heading as used in §11.6.
	Name string
	// PDFSection is the section reference within arXiv:2604.14228v1.
	PDFSection string
	// Category is the primary design axis.
	Category EmergingDirectionCategory
	// Horizon is the assessed nearness to production.
	Horizon EmergingHorizon
	// AffectedValues lists the §2.1 human values the direction primarily serves.
	AffectedValues []DesignValue
	// SupportingEvidence lists the citations §11.6 provides.
	SupportingEvidence []CitedEvidence
	// RelatedPrinciples names §2.2 design principles this direction extends.
	RelatedPrinciples []RelatedDesignPrinciple
	// CurrentStateInClaudeCode describes what Claude Code already implements
	// as of the paper's analysis, if any.
	CurrentStateInClaudeCode string
	// OpenQuestion is the specific design choice §11.6 leaves unresolved.
	OpenQuestion string
	// RequiresHarnessChange is true when the direction demands harness-layer work
	// beyond model capability improvements alone.
	RequiresHarnessChange bool
}

// seedEmergingDirectionProfiles is the closed set of §11.6 directions.
var seedEmergingDirectionProfiles = []EmergingDirectionProfile{
	{
		ID:         DirectionArchitecturalDecoupling,
		Name:       "Architectural Decoupling",
		PDFSection: "§11.6",
		Category:   CategoryArchitecture,
		Horizon:    HorizonNearTerm,
		AffectedValues: []DesignValue{
			DesignValueCapability,
			DesignValueReliability,
		},
		SupportingEvidence: []CitedEvidence{
			{
				Citation:         "Martin et al. (2026)",
				Finding:          "Managed Agents work virtualises session, harness, and sandbox into independently-replaceable interfaces analogous to OS virtualisation of hardware.",
				IsEmpiricalStudy: false,
			},
			{
				Citation:         "Rajasekaran (2026)",
				Finding:          "The space of interesting harness combinations doesn't shrink as models improve; it moves — the architecture is a snapshot of a co-evolving system.",
				IsEmpiricalStudy: false,
			},
		},
		RelatedPrinciples: []RelatedDesignPrinciple{
			"minimal_scaffolding_maximal_operational_harness",
			"composable_multi_mechanism_extensibility",
		},
		CurrentStateInClaudeCode: "Tightly-coupled local architecture: session, harness, and sandbox share common performance constraints and are not independently replaceable.",
		OpenQuestion:             "Whether to virtualise components into independently-replaceable interfaces and at which granularity (session vs. harness vs. sandbox).",
		RequiresHarnessChange:    true,
	},
	{
		ID:         DirectionMemoryFirstClass,
		Name:       "Memory as a First-Class Subsystem",
		PDFSection: "§11.6",
		Category:   CategoryMemory,
		Horizon:    HorizonMidTerm,
		AffectedValues: []DesignValue{
			DesignValueAdaptability,
			DesignValueReliability,
		},
		SupportingEvidence: []CitedEvidence{
			{
				Citation:         "Hu et al. (2025)",
				Finding:          "Agent memory is becoming a distinct cognitive substrate; three open frontiers are automated memory management, RL-driven memory, and trustworthy memory.",
				IsEmpiricalStudy: true,
			},
			{
				Citation:         "Zhang et al. (2025a)",
				Finding:          "Context-engineering literature has started to provide mechanisms for experiential tier accumulation.",
				IsEmpiricalStudy: false,
			},
		},
		RelatedPrinciples: []RelatedDesignPrinciple{
			"transparent_file_based_configuration_and_memory",
			"append_only_durable_state",
		},
		CurrentStateInClaudeCode: "Factual tier (CLAUDE.md, auto memory) and working tier (conversation window) are exposed; experiential tier (accumulated playbooks) is not yet present.",
		OpenQuestion:             "Whether a single substrate can carry both a user's personal instruction hierarchy and a shared organisational context while preserving file-based transparency.",
		RequiresHarnessChange:    true,
	},
	{
		ID:         DirectionObservabilityAndSilentFailure,
		Name:       "Observability and Silent Failure",
		PDFSection: "§11.6",
		Category:   CategoryObservability,
		Horizon:    HorizonNearTerm,
		AffectedValues: []DesignValue{
			DesignValueSafety,
			DesignValueReliability,
		},
		SupportingEvidence: []CitedEvidence{
			{
				Citation:         "Wade et al. (2026)",
				Finding:          "Bessemer's 2026 infrastructure report estimates 78% of AI failures are invisible — the dominant failure mode is not crashes but silent mistakes.",
				IsEmpiricalStudy: true,
			},
			{
				Citation:         "LangChain (2026)",
				Finding:          "1,340-respondent state-of-agent-engineering survey finds observability as top barrier to production use (89% adoption gap) with only 52.4% offline evaluation adoption.",
				IsEmpiricalStudy: true,
			},
			{
				Citation:         "Rajasekaran (2026)",
				Finding:          "Closing the evaluation gap likely requires generator-evaluator separation, sprint contracts, and post-hoc checks — additional scaffolding rather than model improvements alone.",
				IsEmpiricalStudy: false,
			},
		},
		RelatedPrinciples: []RelatedDesignPrinciple{
			"graceful_recovery_and_resilience",
			"append_only_durable_state",
		},
		CurrentStateInClaudeCode: "Architecture provides visibility into tool calls, hooks, and session transcripts but lacks generator-evaluator separation and post-hoc evaluation scaffolding.",
		OpenQuestion:             "Whether evaluation scaffolding belongs inside the harness as an additional hook event or outside it as a separate evaluation layer.",
		RequiresHarnessChange:    true,
	},
	{
		ID:         DirectionGovernance,
		Name:       "Governance",
		PDFSection: "§11.6",
		Category:   CategoryGovernance,
		Horizon:    HorizonNearTerm,
		AffectedValues: []DesignValue{
			DesignValueHumanAuthority,
			DesignValueSafety,
		},
		SupportingEvidence: []CitedEvidence{
			{
				Citation:         "Bengio et al. (2026)",
				Finding:          "International AI Safety Report warns AI agents pose heightened risks because they act autonomously, making it harder for humans to intervene before failures cause harm.",
				IsEmpiricalStudy: false,
			},
			{
				Citation:         "Staufer et al. (2026)",
				Finding:          "MIT AI Agent Index finds only 13.3% of indexed agentic systems publish agent-specific safety cards.",
				IsEmpiricalStudy: true,
			},
		},
		RelatedPrinciples: []RelatedDesignPrinciple{
			"deny_first_with_human_escalation",
			"externalized_programmable_policy",
		},
		CurrentStateInClaudeCode: "Permission architecture and deny-first defaults provide some compliance surface, but no agent-specific safety card mechanism exists in the analysed source.",
		OpenQuestion:             "How evolving copyright jurisprudence around AI-generated code and the EU AI Act (fully applicable August 2026) will impose constraints on logging, transparency, and human oversight.",
		RequiresHarnessChange:    true,
	},
	{
		ID:         DirectionProactiveArchitectures,
		Name:       "Proactive Architectures",
		PDFSection: "§11.6",
		Category:   CategoryProactivity,
		Horizon:    HorizonNearTerm,
		AffectedValues: []DesignValue{
			DesignValueCapability,
			DesignValueAdaptability,
		},
		SupportingEvidence: []CitedEvidence{
			{
				Citation:         "Chen et al. (2025)",
				Finding:          "Proactive AI assistants increase task completion by 12–18% but reduce user preference at high frequencies — KAIROS resolves this via terminal-focus awareness.",
				IsEmpiricalStudy: true,
			},
		},
		RelatedPrinciples: []RelatedDesignPrinciple{
			"graduated_trust_spectrum",
			"reversibility_weighted_risk_assessment",
		},
		CurrentStateInClaudeCode: "KAIROS is a feature-gated system: when no user messages are pending, harness injects periodic <tick> prompts; SleepTool enforces economic throttling (each wake-up costs an API call; prompt cache expires after five minutes).",
		OpenQuestion:             "Whether KAIROS is active in production builds; whether terminal-focus awareness and economic throttling together adequately address the proactivity-vs-disruption tension at scale.",
		RequiresHarnessChange:    false, // KAIROS is already a harness-layer mechanism
	},
}

// SeedEmergingDirectionCount is the total number of §11.6 emerging directions.
const SeedEmergingDirectionCount = 5

// SeedEmergingDirectionIDs lists all direction IDs in the order §11.6 presents them.
var SeedEmergingDirectionIDs = []EmergingDirectionID{
	DirectionArchitecturalDecoupling,
	DirectionMemoryFirstClass,
	DirectionObservabilityAndSilentFailure,
	DirectionGovernance,
	DirectionProactiveArchitectures,
}

// EmergingDirectionRegistry provides structured access to the five §11.6 emerging
// directions identified by arXiv:2604.14228v1.
type EmergingDirectionRegistry struct {
	profiles []EmergingDirectionProfile
	index    map[EmergingDirectionID]*EmergingDirectionProfile
}

// NewEmergingDirectionRegistry constructs a registry pre-loaded with the seed data.
func NewEmergingDirectionRegistry() *EmergingDirectionRegistry {
	r := &EmergingDirectionRegistry{
		profiles: make([]EmergingDirectionProfile, len(seedEmergingDirectionProfiles)),
		index:    make(map[EmergingDirectionID]*EmergingDirectionProfile, len(seedEmergingDirectionProfiles)),
	}
	copy(r.profiles, seedEmergingDirectionProfiles)
	for i := range r.profiles {
		r.index[r.profiles[i].ID] = &r.profiles[i]
	}
	return r
}

// FindByID returns the profile for the given direction ID and whether it was found.
func (r *EmergingDirectionRegistry) FindByID(id EmergingDirectionID) (EmergingDirectionProfile, bool) {
	p, ok := r.index[id]
	if !ok {
		return EmergingDirectionProfile{}, false
	}
	return *p, true
}

// All returns a copy of all direction profiles in seed order.
func (r *EmergingDirectionRegistry) All() []EmergingDirectionProfile {
	out := make([]EmergingDirectionProfile, len(r.profiles))
	copy(out, r.profiles)
	return out
}

// Count returns the number of registered directions.
func (r *EmergingDirectionRegistry) Count() int { return len(r.profiles) }

// IsValid returns true when id names a known direction.
func (r *EmergingDirectionRegistry) IsValid(id EmergingDirectionID) bool {
	_, ok := r.index[id]
	return ok
}

// ByCategory returns all directions whose Category matches cat.
func (r *EmergingDirectionRegistry) ByCategory(cat EmergingDirectionCategory) []EmergingDirectionProfile {
	var out []EmergingDirectionProfile
	for i := range r.profiles {
		if r.profiles[i].Category == cat {
			out = append(out, r.profiles[i])
		}
	}
	return out
}

// ByHorizon returns all directions with the given horizon.
func (r *EmergingDirectionRegistry) ByHorizon(h EmergingHorizon) []EmergingDirectionProfile {
	var out []EmergingDirectionProfile
	for i := range r.profiles {
		if r.profiles[i].Horizon == h {
			out = append(out, r.profiles[i])
		}
	}
	return out
}

// RequiringHarnessChange returns directions that cannot be addressed by model
// capability improvements alone — they require harness-layer engineering work.
func (r *EmergingDirectionRegistry) RequiringHarnessChange() []EmergingDirectionProfile {
	var out []EmergingDirectionProfile
	for i := range r.profiles {
		if r.profiles[i].RequiresHarnessChange {
			out = append(out, r.profiles[i])
		}
	}
	return out
}

// ServingValue returns all directions that list v among their AffectedValues.
func (r *EmergingDirectionRegistry) ServingValue(v DesignValue) []EmergingDirectionProfile {
	var out []EmergingDirectionProfile
	for i := range r.profiles {
		for _, av := range r.profiles[i].AffectedValues {
			if av == v {
				out = append(out, r.profiles[i])
				break
			}
		}
	}
	return out
}

// WithEmpiricalEvidence returns directions that have at least one piece of
// empirical study evidence cited by §11.6.
func (r *EmergingDirectionRegistry) WithEmpiricalEvidence() []EmergingDirectionProfile {
	var out []EmergingDirectionProfile
	for i := range r.profiles {
		for _, e := range r.profiles[i].SupportingEvidence {
			if e.IsEmpiricalStudy {
				out = append(out, r.profiles[i])
				break
			}
		}
	}
	return out
}

// TotalEvidenceCount returns the total number of cited evidence items across all
// directions.
func (r *EmergingDirectionRegistry) TotalEvidenceCount() int {
	n := 0
	for i := range r.profiles {
		n += len(r.profiles[i].SupportingEvidence)
	}
	return n
}

// StructuralInvariants documents the invariants that must hold for the registry to
// faithfully reflect §11.6:
//
//  1. Exactly five directions are registered (one per §11.6 named direction).
//  2. Every direction has a non-empty OpenQuestion — §11.6 frames each direction as
//     an open design question, not a resolved answer.
//  3. At least four of the five directions RequiresHarnessChange — §11.6 explicitly
//     states that closing the gaps "likely requires additional scaffolding … rather
//     than model improvements alone" for all directions except ProactiveArchitectures
//     which already has a harness-level mechanism (KAIROS).
func (r *EmergingDirectionRegistry) StructuralInvariants() (fiveDirections, allHaveOpenQuestion, fourRequireHarness bool) {
	fiveDirections = r.Count() == SeedEmergingDirectionCount

	allHaveOpenQuestion = true
	for i := range r.profiles {
		if r.profiles[i].OpenQuestion == "" {
			allHaveOpenQuestion = false
			break
		}
	}

	harnessCount := 0
	for i := range r.profiles {
		if r.profiles[i].RequiresHarnessChange {
			harnessCount++
		}
	}
	fourRequireHarness = harnessCount >= 4

	return
}
