package agentic

// research_methodology_registry.go — FEAT-048 — §B.2 Research Methodology
//
// arXiv:2604.14228v1, Appendix B §B.2 "Research Methodology" (pages 43–45).
//
// §B.2 describes the stepwise methodology the paper itself employed to produce
// its findings about Claude Code's architecture. The methodology comprises six
// phases: artifact acquisition, codebase exploration, architecture reconstruction,
// comparative analysis, evidence classification, and claim qualification.
//
// Every claim in the paper is grounded at exactly one evidence tier (A, B, or C),
// and every tier carries a ClaimStrength that determines whether qualifying
// language is required. §B.2 also explains which artifact types back which phases
// and classifies each step as a direct observation or an inferred reconstruction.
//
// This registry models those methodology steps so that the research provenance
// of any architectural claim made in the paper can be introspected programmatically.

// ClaimStrength expresses the epistemic confidence of an architectural claim.
// The four levels map to §B.2's evidence-tier framework: strong and moderate
// map to direct evidence (Tiers B and A respectively), weak to partial or
// indirect evidence, and hedged to reconstructed inference (Tier C).
type ClaimStrength string

const (
	// ClaimStrengthStrong — claim is directly verified from source code (Tier B).
	// No qualifying language is required.
	ClaimStrengthStrong ClaimStrength = "strong"

	// ClaimStrengthModerate — claim is backed by official product documentation
	// or Anthropic engineering publications (Tier A). No qualifying language
	// required, but implementation details may diverge.
	ClaimStrengthModerate ClaimStrength = "moderate"

	// ClaimStrengthWeak — claim rests on partial evidence: a code pattern that
	// is consistent with the claim but does not uniquely confirm it. Some
	// qualifying language is expected.
	ClaimStrengthWeak ClaimStrength = "weak"

	// ClaimStrengthHedged — claim is a reconstruction from community analysis,
	// structural comparison with OpenClaw, or inference from code patterns
	// (Tier C). Qualifying language is mandatory (e.g. "appears to", "likely").
	ClaimStrengthHedged ClaimStrength = "hedged"
)

// ArtifactType identifies the category of research artifact that provides
// evidence for a given methodology step.
type ArtifactType string

const (
	// ArtifactTypeSourceCode — TypeScript source files extracted from the
	// published npm package (@anthropic-ai/claude-code v2.1.88). The primary
	// artifact of the study: ~1,884 files, ~512K lines.
	ArtifactTypeSourceCode ArtifactType = "source_code"

	// ArtifactTypeDocumentation — official Anthropic documentation, engineering
	// blog posts, and public product announcements. Provides product-intent
	// context that source code alone cannot confirm.
	ArtifactTypeDocumentation ArtifactType = "documentation"

	// ArtifactTypeBenchmark — published quantitative results (SWE-bench,
	// SWE-bench Verified) used to calibrate comparative capability claims.
	ArtifactTypeBenchmark ArtifactType = "benchmark"

	// ArtifactTypeEmpiricalStudy — academic papers describing other coding agents
	// (SWE-Agent, OpenHands, Aider, Codex CLI, Devin) used for comparative
	// architectural analysis in §13.
	ArtifactTypeEmpiricalStudy ArtifactType = "empirical_study"

	// ArtifactTypeDesignDoc — internal or reconstructed design documents, including
	// inferred design intent from structural patterns. This tier is the weakest
	// and all associated claims carry ClaimStrengthHedged.
	ArtifactTypeDesignDoc ArtifactType = "design_doc"
)

// MethodologyStep models one named phase of the §B.2 research methodology.
// Each step corresponds to a discrete activity in the paper's investigative
// process, from artifact acquisition through claim qualification.
type MethodologyStep struct {
	// StepID is a stable, lowercase slug uniquely identifying this step
	// (e.g. "artifact_acquisition", "codebase_exploration").
	StepID string

	// Label is the short display name used in §B.2 prose.
	Label string

	// Description is the full explanation of the methodology step, drawn from
	// §B.2 language and the surrounding appendix context.
	Description string

	// PDFSection is the specific appendix reference in arXiv:2604.14228v1
	// where this step is described.
	PDFSection string

	// ArtifactType is the primary category of artifact consulted during this step.
	ArtifactType ArtifactType

	// EvidenceCategory is the Tier label (A, B, or C) that claims derived from
	// this step are assigned in the paper's evidence framework.
	EvidenceCategory EvidenceTierLabel

	// ClaimStrength is the epistemic strength of claims produced by this step.
	ClaimStrength ClaimStrength

	// HasQualifyingLanguage indicates whether claims from this step require
	// hedging phrases (e.g. "appears to", "likely", "suggests that").
	// Must be true for every step whose ClaimStrength is ClaimStrengthHedged.
	HasQualifyingLanguage bool

	// IsDirectObservation reports whether the step involves reading artifacts
	// directly (true) rather than inferring from structural patterns or analogy
	// to other systems (false).
	// Strong-claim steps must be direct observations (§B.2 constraint).
	IsDirectObservation bool

	// CitedSource is the primary source description cited in §B.2 for this step.
	// Example: "npm package @anthropic-ai/claude-code v2.1.88".
	CitedSource string
}

// ResearchMethodologyRegistry is an in-memory registry of the six methodology
// steps described in Appendix B §B.2 of arXiv:2604.14228v1.
//
// The registry provides query methods for filtering steps by claim strength,
// artifact type, evidence category, and observation character, as well as
// structural invariant checks that mirror §B.2's epistemic constraints.
type ResearchMethodologyRegistry struct {
	steps []MethodologyStep
}

// NewResearchMethodologyRegistry constructs and returns the registry pre-seeded
// with the six §B.2 methodology steps in their narrative order.
func NewResearchMethodologyRegistry() *ResearchMethodologyRegistry {
	return &ResearchMethodologyRegistry{
		steps: seedMethodologySteps(),
	}
}

// FindByID returns the step whose StepID matches the given value, together with
// a boolean indicating whether the step was found.
func (r *ResearchMethodologyRegistry) FindByID(id string) (*MethodologyStep, bool) {
	for i := range r.steps {
		if r.steps[i].StepID == id {
			return &r.steps[i], true
		}
	}
	return nil, false
}

// AllSteps returns all registered methodology steps in their §B.2 narrative order.
func (r *ResearchMethodologyRegistry) AllSteps() []MethodologyStep {
	out := make([]MethodologyStep, len(r.steps))
	copy(out, r.steps)
	return out
}

// Count returns the number of methodology steps registered.
func (r *ResearchMethodologyRegistry) Count() int {
	return len(r.steps)
}

// IsValidStepID returns true if the given ID corresponds to a registered step.
func (r *ResearchMethodologyRegistry) IsValidStepID(id string) bool {
	_, ok := r.FindByID(id)
	return ok
}

// StepsByClaimStrength returns all steps whose ClaimStrength matches the given value.
func (r *ResearchMethodologyRegistry) StepsByClaimStrength(cs ClaimStrength) []MethodologyStep {
	var out []MethodologyStep
	for _, s := range r.steps {
		if s.ClaimStrength == cs {
			out = append(out, s)
		}
	}
	return out
}

// HedgedSteps returns all steps that require qualifying language in the paper
// (HasQualifyingLanguage == true).
func (r *ResearchMethodologyRegistry) HedgedSteps() []MethodologyStep {
	var out []MethodologyStep
	for _, s := range r.steps {
		if s.HasQualifyingLanguage {
			out = append(out, s)
		}
	}
	return out
}

// StrongClaimSteps returns all steps that produce strong-confidence claims
// (ClaimStrength == ClaimStrengthStrong).
func (r *ResearchMethodologyRegistry) StrongClaimSteps() []MethodologyStep {
	return r.StepsByClaimStrength(ClaimStrengthStrong)
}

// DirectObservationSteps returns all steps classified as direct observations
// (IsDirectObservation == true).
func (r *ResearchMethodologyRegistry) DirectObservationSteps() []MethodologyStep {
	var out []MethodologyStep
	for _, s := range r.steps {
		if s.IsDirectObservation {
			out = append(out, s)
		}
	}
	return out
}

// StepsByArtifactType returns all steps that consult the given artifact type.
func (r *ResearchMethodologyRegistry) StepsByArtifactType(at ArtifactType) []MethodologyStep {
	var out []MethodologyStep
	for _, s := range r.steps {
		if s.ArtifactType == at {
			out = append(out, s)
		}
	}
	return out
}

// StepsByEvidenceCategory returns all steps assigned to the given evidence tier.
func (r *ResearchMethodologyRegistry) StepsByEvidenceCategory(tier EvidenceTierLabel) []MethodologyStep {
	var out []MethodologyStep
	for _, s := range r.steps {
		if s.EvidenceCategory == tier {
			out = append(out, s)
		}
	}
	return out
}

// HedgedStepsHaveQualifyingLanguage is an invariant that asserts every step
// with ClaimStrengthHedged has HasQualifyingLanguage == true.
// Returns (true, nil) when the invariant holds, (false, violations) otherwise.
func (r *ResearchMethodologyRegistry) HedgedStepsHaveQualifyingLanguage() (bool, []string) {
	var violations []string
	for _, s := range r.steps {
		if s.ClaimStrength == ClaimStrengthHedged && !s.HasQualifyingLanguage {
			violations = append(violations, s.StepID)
		}
	}
	return len(violations) == 0, violations
}

// StrongClaimsHaveDirectObservation is an invariant that asserts every step
// with ClaimStrengthStrong has IsDirectObservation == true.
// Returns (true, nil) when the invariant holds, (false, violations) otherwise.
func (r *ResearchMethodologyRegistry) StrongClaimsHaveDirectObservation() (bool, []string) {
	var violations []string
	for _, s := range r.steps {
		if s.ClaimStrength == ClaimStrengthStrong && !s.IsDirectObservation {
			violations = append(violations, s.StepID)
		}
	}
	return len(violations) == 0, violations
}

// AllHaveEvidenceCategory is an invariant that asserts every step has a
// non-empty EvidenceCategory.
// Returns (true, nil) when the invariant holds, (false, violations) otherwise.
func (r *ResearchMethodologyRegistry) AllHaveEvidenceCategory() (bool, []string) {
	var violations []string
	for _, s := range r.steps {
		if s.EvidenceCategory == "" {
			violations = append(violations, s.StepID)
		}
	}
	return len(violations) == 0, violations
}

// SeedMethodologyStepCount is the canonical number of §B.2 methodology steps.
const SeedMethodologyStepCount = 6

// SeedMethodologyStepIDs lists the step IDs in §B.2 narrative order.
var SeedMethodologyStepIDs = []string{
	"artifact_acquisition",
	"codebase_exploration",
	"architecture_reconstruction",
	"comparative_analysis",
	"evidence_classification",
	"claim_qualification",
}

// seedMethodologySteps returns the canonical §B.2 methodology steps.
func seedMethodologySteps() []MethodologyStep {
	return []MethodologyStep{
		{
			StepID:     "artifact_acquisition",
			Label:      "Artifact Acquisition",
			PDFSection: "Appendix B §B.2",
			Description: "The primary research artifact is the publicly available npm package " +
				"@anthropic-ai/claude-code v2.1.88. It was extracted and subjected to minification " +
				"reversal, yielding approximately 1,884 TypeScript files totalling roughly 512K lines. " +
				"This extraction is the foundation for all Tier B (code-verified) claims in the paper.",
			ArtifactType:          ArtifactTypeSourceCode,
			EvidenceCategory:      EvidenceTierLabelB,
			ClaimStrength:         ClaimStrengthStrong,
			HasQualifyingLanguage: false,
			IsDirectObservation:   true,
			CitedSource:           "npm package @anthropic-ai/claude-code v2.1.88",
		},
		{
			StepID:     "codebase_exploration",
			Label:      "Codebase Exploration",
			PDFSection: "Appendix B §B.2",
			Description: "Systematic traversal of the extracted TypeScript codebase to identify " +
				"module boundaries, import graphs, and subsystem responsibilities. Key files were " +
				"identified by directory structure (internal/, types/, commands/, tools/) and by " +
				"frequency of cross-module import. Control-flow analysis of the agent loop and " +
				"permission pipeline provided Tier B evidence for §4 (Context Assembly) and §5 " +
				"(Permission Architecture).",
			ArtifactType:          ArtifactTypeSourceCode,
			EvidenceCategory:      EvidenceTierLabelB,
			ClaimStrength:         ClaimStrengthStrong,
			HasQualifyingLanguage: false,
			IsDirectObservation:   true,
			CitedSource:           "Extracted TypeScript source: internal/, types/, commands/, tools/",
		},
		{
			StepID:     "architecture_reconstruction",
			Label:      "Architecture Reconstruction",
			PDFSection: "Appendix B §B.2",
			Description: "Higher-level architectural claims (subsystem decomposition, data-flow " +
				"patterns, design principles) were reconstructed from code patterns and " +
				"cross-referenced with official Anthropic product documentation. Reconstruction " +
				"introduces Tier A evidence (documentation-backed) when docs corroborate the " +
				"structural finding, and Tier C (reconstructed) when no corroborating documentation " +
				"exists. Design-intent claims — why a mechanism was designed as observed — are " +
				"always Tier C unless an Anthropic engineering blog post or announcement confirms " +
				"the rationale explicitly.",
			ArtifactType:          ArtifactTypeDesignDoc,
			EvidenceCategory:      EvidenceTierLabelC,
			ClaimStrength:         ClaimStrengthHedged,
			HasQualifyingLanguage: true,
			IsDirectObservation:   false,
			CitedSource:           "Reconstructed from code patterns + Anthropic engineering publications",
		},
		{
			StepID:     "comparative_analysis",
			Label:      "Comparative Analysis",
			PDFSection: "Appendix B §B.2",
			Description: "Comparative claims in §13 (\"Agent Architecture Patterns\") place Claude Code " +
				"in relation to SWE-Agent, OpenHands, Aider, Codex CLI, and Devin. Evidence for " +
				"comparators comes from empirical studies and benchmark papers describing those " +
				"systems. Because comparator analyses are based on published descriptions rather " +
				"than direct source-code inspection, they are Tier A (documentation-backed) or " +
				"Tier C (reconstructed), never Tier B. The OpenClaw structural comparison is " +
				"specifically Tier C and described in §B.3 as a calibration snapshot that may " +
				"not reflect current state.",
			ArtifactType:          ArtifactTypeEmpiricalStudy,
			EvidenceCategory:      EvidenceTierLabelA,
			ClaimStrength:         ClaimStrengthModerate,
			HasQualifyingLanguage: false,
			IsDirectObservation:   false,
			CitedSource:           "Yang et al. 2024 (SWE-Agent), Wang et al. 2024b (OpenHands), Gauthier 2024 (Aider), SWE-bench Verified leaderboard",
		},
		{
			StepID:     "evidence_classification",
			Label:      "Evidence Classification",
			PDFSection: "Appendix B §B.2",
			Description: "Each claim was assigned to exactly one of three evidence tiers (A, B, C) " +
				"based on the provenance of its supporting evidence. Tier B is reserved for claims " +
				"that cite a specific file and function in the extracted TypeScript codebase. " +
				"Tier A is used when the claim draws on official Anthropic documentation or " +
				"engineering publications. Tier C is used for claims derived from structural " +
				"inference, community analysis, or OpenClaw comparison. When evidence from multiple " +
				"tiers is available, the highest tier governs. This classification is applied " +
				"consistently throughout §3–§13 and surfaced in benchmark result citations.",
			ArtifactType:          ArtifactTypeDocumentation,
			EvidenceCategory:      EvidenceTierLabelA,
			ClaimStrength:         ClaimStrengthModerate,
			HasQualifyingLanguage: false,
			IsDirectObservation:   true,
			CitedSource:           "Evidence tier framework defined in Appendix B §B.1",
		},
		{
			StepID:     "claim_qualification",
			Label:      "Claim Qualification",
			PDFSection: "Appendix B §B.2",
			Description: "Claims grounded at Tier C are stated with mandatory hedging language " +
				"(\"appears to\", \"likely\", \"suggests that\", \"we infer\"). Claims grounded at " +
				"Tiers A and B may be stated directly. Feature-flag-gated behaviour is labelled " +
				"throughout the paper to acknowledge that enabled flags may produce a functionally " +
				"different application at runtime than what static source-code analysis reveals. " +
				"This qualification step ensures that the paper's epistemic commitments match the " +
				"evidence strength at each claim site, consistent with the §B.3 limitation " +
				"\"Reverse-engineering epistemology\".",
			ArtifactType:          ArtifactTypeDocumentation,
			EvidenceCategory:      EvidenceTierLabelC,
			ClaimStrength:         ClaimStrengthHedged,
			HasQualifyingLanguage: true,
			IsDirectObservation:   false,
			CitedSource:           "§B.3 limitations: static_snapshot, reverse_engineering_epistemology",
		},
	}
}
