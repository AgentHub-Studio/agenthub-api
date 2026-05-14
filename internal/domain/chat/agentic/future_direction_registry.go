package agentic

// FutureDirectionRegistry catalogues the six open design questions identified in
// §12 "Future Directions" (pages 32–35) of arXiv:2604.14228v1.
//
// Each direction is posed as a concrete architectural question that a growing
// external literature has sharpened enough to state precisely.  The six questions
// span the paper's five-value framework (§2.1) and its evaluative lens (§2.4):
//
//   - §12.1 Observability–Evaluation Gap    (Safety)
//   - §12.2 Persistence & Longitudinal Colleague Relationships  (Reliability)
//   - §12.3 Harness Boundary Evolution      (Capability)
//   - §12.4 Horizon Scaling                 (Reliability)
//   - §12.5 Governance and Oversight at Scale (Authority)
//   - §12.6 Evaluative Lens Revisited       (cross-cutting)
//
// PDF reference: §12, pages 32–35.

// FutureDirectionValueDimension names the design-value axis that a future
// direction primarily challenges.
type FutureDirectionValueDimension string

const (
	// FutureDirectionDimSafety — the question targets the Safety & Security value (§2.1).
	FutureDirectionDimSafety FutureDirectionValueDimension = "safety"

	// FutureDirectionDimReliability — the question targets the Reliable Execution value (§2.1).
	FutureDirectionDimReliability FutureDirectionValueDimension = "reliability"

	// FutureDirectionDimCapability — the question targets the Capability Amplification value (§2.1).
	FutureDirectionDimCapability FutureDirectionValueDimension = "capability"

	// FutureDirectionDimAuthority — the question targets the Human Decision Authority value (§2.1).
	FutureDirectionDimAuthority FutureDirectionValueDimension = "authority"

	// FutureDirectionDimCrosscutting — the question spans multiple values simultaneously
	// (used for §12.6 which is the evaluative lens applied across all five values).
	FutureDirectionDimCrosscut FutureDirectionValueDimension = "crosscutting"
)

// FutureDirectionHorizon classifies the temporal scope of an open question.
type FutureDirectionHorizon string

const (
	// FutureDirectionHorizonNearTerm — addressable within current architecture extensions.
	FutureDirectionHorizonNearTerm FutureDirectionHorizon = "near_term"

	// FutureDirectionHorizonMediumTerm — requires new subsystems or cross-session primitives.
	FutureDirectionHorizonMediumTerm FutureDirectionHorizon = "medium_term"

	// FutureDirectionHorizonLongTerm — requires ecosystem-level changes (regulation, multi-system).
	FutureDirectionHorizonLongTerm FutureDirectionHorizon = "long_term"
)

// FutureDirectionProfile is the immutable descriptor of one §12 open question.
type FutureDirectionProfile struct {
	// Slug is a stable lower-kebab-case identifier.
	Slug string

	// Label is the section title as it appears in the PDF.
	Label string

	// PDFSection is the sub-section reference (e.g., "12.1").
	PDFSection string

	// ValueDimension is the primary design-value axis the question challenges.
	ValueDimension FutureDirectionValueDimension

	// Horizon classifies how near or far the resolution of the question is.
	Horizon FutureDirectionHorizon

	// CoreQuestion is the "whether/how/which" formulation from §12 introduction.
	CoreQuestion string

	// ArchitecturalGap describes what the current architecture leaves open.
	ArchitecturalGap string

	// KeyMechanism names a concrete mechanism or scaffolding approach cited in the PDF.
	// Empty string when the paper deliberately leaves the mechanism open.
	KeyMechanism string

	// IsEvaluativeLens is true only for §12.6, which reframes the §2.4 lens as a
	// first-class design problem rather than a diagnostic one.
	IsEvaluativeLens bool

	// RequiresExternalConstraint is true when the question is partly driven by
	// regulatory or governance forces outside the system itself (§12.5).
	RequiresExternalConstraint bool
}

// FutureDirectionRegistry is the registry of the six §12 open design questions.
type FutureDirectionRegistry struct {
	profiles []FutureDirectionProfile
}

// NewFutureDirectionRegistry returns a registry pre-seeded with all six §12
// future-direction profiles in PDF section order.
func NewFutureDirectionRegistry() *FutureDirectionRegistry {
	return &FutureDirectionRegistry{
		profiles: []FutureDirectionProfile{
			{
				Slug:       "observability_evaluation_gap",
				Label:      "Silent Failure and the Observability–Evaluation Gap",
				PDFSection: "12.1",
				ValueDimension: FutureDirectionDimSafety,
				Horizon:    FutureDirectionHorizonNearTerm,
				CoreQuestion: "Whether the observability–evaluation adoption gap is a missing tooling " +
					"layer, a missing evaluation interface inside the harness, or a model-capability ceiling.",
				ArchitecturalGap: "The architecture gives operators visibility into tool calls, hooks, and " +
					"session transcripts, but closing the evaluation gap likely requires additional scaffolding " +
					"(generator-evaluator separation, sprint contracts, post-hoc checks) rather than model " +
					"improvements alone.",
				KeyMechanism:               "generator-evaluator separation and post-hoc hook scaffolding",
				IsEvaluativeLens:           false,
				RequiresExternalConstraint: false,
			},
			{
				Slug:       "persistence_longitudinal_colleague",
				Label:      "Persistence: Memory and Longitudinal Colleague Relationships",
				PDFSection: "12.2",
				ValueDimension: FutureDirectionDimReliability,
				Horizon:    FutureDirectionHorizonMediumTerm,
				CoreQuestion: "Whether agent state and the human–agent working relationship should persist " +
					"across sessions, and in what form, specifically what belongs between the CLAUDE.md " +
					"hierarchy and append-only transcripts.",
				ArchitecturalGap: "Session-scoped permissions do not restore on resume (§9 deliberate choice); " +
					"durable state that is neither a static instruction nor a single session transcript is an " +
					"open design question. A single substrate carrying both personal instruction hierarchy and " +
					"shared organisational context while preserving CLAUDE.md file transparency is unresolved.",
				KeyMechanism:               "accumulating memory layer between CLAUDE.md and session transcripts",
				IsEvaluativeLens:           false,
				RequiresExternalConstraint: false,
			},
			{
				Slug:       "harness_boundary_evolution",
				Label:      "Harness Boundary Evolution: Where, When, What, and with Whom the Agent Acts",
				PDFSection: "12.3",
				ValueDimension: FutureDirectionDimCapability,
				Horizon:    FutureDirectionHorizonMediumTerm,
				CoreQuestion: "Whether the space of interesting harness combinations fragments into specialised " +
					"stacks or whether a single harness architecture can span all four extension axes: where " +
					"the harness runs, when it acts, what it acts on, and with whom it coordinates.",
				ArchitecturalGap: "The where-, what-, and with-whom-extensions raise questions the paper's " +
					"single-subsystem analyses cannot resolve: which governance obligations attach when harness " +
					"components become hosted services, and how reversibility-weighted risk scales to physical " +
					"rather than textual effects.",
				KeyMechanism:               "",
				IsEvaluativeLens:           false,
				RequiresExternalConstraint: false,
			},
			{
				Slug:       "horizon_scaling",
				Label:      "Horizon Scaling: From Session to Scientific Program",
				PDFSection: "12.4",
				ValueDimension: FutureDirectionDimReliability,
				Horizon:    FutureDirectionHorizonLongTerm,
				CoreQuestion: "Whether the context-management pipeline of §7, the last-assistant-text " +
					"return policy of §8, and the append-only persistence of §9 remain sufficient when " +
					"sessions compose into multi-session programs spanning days or weeks.",
				ArchitecturalGap: "The architecture's primary units are the turn, the session, and the " +
					"sub-agent; whether it continues to support long-horizon dependability as autonomous work " +
					"extends beyond a single session is an open question. Horizon scaling restates §11.4's " +
					"directly-measurable empirical question at the scale of weeks.",
				KeyMechanism:               "cross-session memory substrate or coordination primitives beyond session",
				IsEvaluativeLens:           false,
				RequiresExternalConstraint: false,
			},
			{
				Slug:       "governance_oversight_at_scale",
				Label:      "Governance and Oversight at Scale",
				PDFSection: "12.5",
				ValueDimension: FutureDirectionDimAuthority,
				Horizon:    FutureDirectionHorizonLongTerm,
				CoreQuestion: "Which logging, transparency, and human-oversight affordances coding-agent " +
					"architectures should expose under external regulatory constraints, specifically the EU " +
					"AI Act (fully applicable August 2026) and evolving copyright jurisprudence.",
				ArchitecturalGap: "The deny-first evaluation documented in §5 is internally auditable through " +
					"session transcripts (§9) but not yet externally auditable in the forms that emerging " +
					"frameworks contemplate. The values-over-rules principle admits the kind of explicit rule " +
					"articulation that compliance review may call for — both properties lie within the harness " +
					"rather than the model.",
				KeyMechanism:               "regulator-facing interfaces for audit and transparency",
				IsEvaluativeLens:           false,
				RequiresExternalConstraint: true,
			},
			{
				Slug:       "evaluative_lens_revisited",
				Label:      "The Evaluative Lens Revisited: Long-Term Human Capability",
				PDFSection: "12.6",
				ValueDimension: FutureDirectionDimCrosscut,
				Horizon:    FutureDirectionHorizonLongTerm,
				CoreQuestion: "Whether the §2.4 evaluative lens (long-term human capability preservation) " +
					"can be treated as a first-class design problem rather than a downstream evaluation " +
					"metric, and what architectural mechanisms a first-class treatment would require.",
				ArchitecturalGap: "The architecture exposes no per-session signal for comprehension or " +
					"convention drift. The harness documented here is not the right locus for action in " +
					"isolation — the IDE, the organisation, or the human development loop may also be " +
					"required. Whether architecture alone can respond to session-granularity measurements " +
					"is the design-gap question §14 poses.",
				KeyMechanism:               "comprehension-preserving surfaces or session-level cognitive-offloading probes",
				IsEvaluativeLens:           true,
				RequiresExternalConstraint: false,
			},
		},
	}
}

// FindFutureDirectionBySlug returns the profile for the given slug, or false if not found.
func (r *FutureDirectionRegistry) FindFutureDirectionBySlug(slug string) (*FutureDirectionProfile, bool) {
	for i := range r.profiles {
		if r.profiles[i].Slug == slug {
			return &r.profiles[i], true
		}
	}
	return nil, false
}

// AllFutureDirections returns all six profiles in §12 section order.
func (r *FutureDirectionRegistry) AllFutureDirections() []FutureDirectionProfile {
	out := make([]FutureDirectionProfile, len(r.profiles))
	copy(out, r.profiles)
	return out
}

// ByValueDimension returns all profiles whose primary value dimension matches dim.
func (r *FutureDirectionRegistry) ByValueDimension(dim FutureDirectionValueDimension) []FutureDirectionProfile {
	var out []FutureDirectionProfile
	for _, p := range r.profiles {
		if p.ValueDimension == dim {
			out = append(out, p)
		}
	}
	return out
}

// ByHorizon returns all profiles with the given temporal horizon.
func (r *FutureDirectionRegistry) ByHorizon(h FutureDirectionHorizon) []FutureDirectionProfile {
	var out []FutureDirectionProfile
	for _, p := range r.profiles {
		if p.Horizon == h {
			out = append(out, p)
		}
	}
	return out
}

// EvaluativeLensDirections returns the subset of profiles that reframe the §2.4
// evaluative lens as a first-class design concern (currently only §12.6).
func (r *FutureDirectionRegistry) EvaluativeLensDirections() []FutureDirectionProfile {
	var out []FutureDirectionProfile
	for _, p := range r.profiles {
		if p.IsEvaluativeLens {
			out = append(out, p)
		}
	}
	return out
}

// ExternallyConstrainedDirections returns the profiles whose resolution is partly
// driven by regulatory or governance forces outside the system itself (§12.5).
func (r *FutureDirectionRegistry) ExternallyConstrainedDirections() []FutureDirectionProfile {
	var out []FutureDirectionProfile
	for _, p := range r.profiles {
		if p.RequiresExternalConstraint {
			out = append(out, p)
		}
	}
	return out
}

// DirectionsWithOpenMechanism returns profiles where the PDF deliberately leaves
// the specific mechanism unspecified (KeyMechanism == "").
func (r *FutureDirectionRegistry) DirectionsWithOpenMechanism() []FutureDirectionProfile {
	var out []FutureDirectionProfile
	for _, p := range r.profiles {
		if p.KeyMechanism == "" {
			out = append(out, p)
		}
	}
	return out
}

// SeedFutureDirectionCount is the number of profiles seeded into the registry (§12).
const SeedFutureDirectionCount = 6

// SeedFutureDirectionSlugs lists all slugs in §12 section order.
var SeedFutureDirectionSlugs = []string{
	"observability_evaluation_gap",
	"persistence_longitudinal_colleague",
	"harness_boundary_evolution",
	"horizon_scaling",
	"governance_oversight_at_scale",
	"evaluative_lens_revisited",
}
