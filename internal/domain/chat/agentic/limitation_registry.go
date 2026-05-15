package agentic

// LimitationRegistry catalogues the four named epistemological and methodological
// limitations of the Claude Code architecture study, as stated in Appendix B §B.3
// "Limitations" (page 45) of arXiv:2604.14228v1.
//
// Each limitation is modelled as a LimitationProfile that captures its slug,
// label, type, scope, description, mitigation notes, and which categories of
// claim it affects.

// LimitationType classifies the nature of a study limitation.
type LimitationType string

const (
	// LimitationTypeTemporal — the analysis reflects a fixed point in time;
	// findings may not hold for other versions or builds.
	LimitationTypeTemporal LimitationType = "temporal"

	// LimitationTypeEpistemology — the reverse-engineering approach bounds what
	// can be inferred about design intent, runtime state, or deployment prevalence.
	LimitationTypeEpistemology LimitationType = "epistemology"

	// LimitationTypeScope — the study examines a single system; results are not
	// directly generalisable to the broader class of coding agents.
	LimitationTypeScope LimitationType = "scope"

	// LimitationTypeCalibration — the comparative baseline (OpenClaw) reflects
	// a specific development snapshot and may not represent its current state.
	LimitationTypeCalibration LimitationType = "calibration"
)

// AffectedClaimType identifies which categories of claim a limitation affects.
type AffectedClaimType string

const (
	// AffectedClaimTypeAll — the limitation applies to every claim in the study.
	AffectedClaimTypeAll AffectedClaimType = "all"

	// AffectedClaimTypeDesignIntent — claims about why a design decision was made.
	AffectedClaimTypeDesignIntent AffectedClaimType = "design_intent"

	// AffectedClaimTypeComparative — claims that compare Claude Code against
	// other coding agents.
	AffectedClaimTypeComparative AffectedClaimType = "comparative"

	// AffectedClaimTypeImplementation — claims about runtime behaviour, enabled
	// feature flags, or production prevalence.
	AffectedClaimTypeImplementation AffectedClaimType = "implementation"
)

// LimitationProfile captures the full description of one named limitation as
// stated in Appendix B §B.3 of the paper.
type LimitationProfile struct {
	// Slug is a stable lowercase identifier for the limitation (e.g. "static_snapshot").
	Slug string

	// Label is the bold heading used in the §B.3 bullet list.
	Label string

	// PDFSection is the appendix location in the paper.
	PDFSection string

	// LimitationType classifies the nature of the limitation.
	LimitationType LimitationType

	// Description is the prose definition drawn directly from §B.3.
	Description string

	// MitigationNotes describes how the paper handles or signals this limitation.
	MitigationNotes string

	// AffectsClaimType indicates which category of claim this limitation bears on.
	AffectsClaimType AffectedClaimType

	// IsEpistemological is true when the limitation constrains what knowledge
	// can be derived from the evidence (reverse-engineering bounds).
	IsEpistemological bool

	// RequiresHedgedGeneralisation is true when findings derived despite this
	// limitation must be stated as bounded rather than universal.
	RequiresHedgedGeneralisation bool
}

// LimitationRegistry is an in-memory registry of the four named study
// limitations described in Appendix B §B.3 of the Claude Code architecture paper.
type LimitationRegistry struct {
	profiles []LimitationProfile
}

// NewLimitationRegistry constructs and returns the registry pre-seeded with
// the four limitations exactly as defined in §B.3.
func NewLimitationRegistry() *LimitationRegistry {
	return &LimitationRegistry{
		profiles: seedLimitationProfiles(),
	}
}

// FindLimitationBySlug returns the profile whose Slug matches the given value,
// together with a boolean indicating whether the profile was found.
func (r *LimitationRegistry) FindLimitationBySlug(slug string) (*LimitationProfile, bool) {
	for i := range r.profiles {
		if r.profiles[i].Slug == slug {
			return &r.profiles[i], true
		}
	}
	return nil, false
}

// FindLimitationByLabel returns the profile whose Label matches (case-sensitive).
func (r *LimitationRegistry) FindLimitationByLabel(label string) (*LimitationProfile, bool) {
	for i := range r.profiles {
		if r.profiles[i].Label == label {
			return &r.profiles[i], true
		}
	}
	return nil, false
}

// FindLimitationsByType returns all profiles whose LimitationType matches the
// given value.
func (r *LimitationRegistry) FindLimitationsByType(lt LimitationType) []LimitationProfile {
	var out []LimitationProfile
	for _, p := range r.profiles {
		if p.LimitationType == lt {
			out = append(out, p)
		}
	}
	return out
}

// FindLimitationsByAffectedClaimType returns all profiles that affect the given
// claim type, including those marked AffectedClaimTypeAll.
func (r *LimitationRegistry) FindLimitationsByAffectedClaimType(act AffectedClaimType) []LimitationProfile {
	var out []LimitationProfile
	for _, p := range r.profiles {
		if p.AffectsClaimType == act || p.AffectsClaimType == AffectedClaimTypeAll {
			out = append(out, p)
		}
	}
	return out
}

// EpistemologicalLimitations returns only the limitations that are
// epistemological in nature (IsEpistemological == true).
func (r *LimitationRegistry) EpistemologicalLimitations() []LimitationProfile {
	var out []LimitationProfile
	for _, p := range r.profiles {
		if p.IsEpistemological {
			out = append(out, p)
		}
	}
	return out
}

// HedgedGeneralisationLimitations returns only the limitations that require
// findings to be stated with bounded generalisations.
func (r *LimitationRegistry) HedgedGeneralisationLimitations() []LimitationProfile {
	var out []LimitationProfile
	for _, p := range r.profiles {
		if p.RequiresHedgedGeneralisation {
			out = append(out, p)
		}
	}
	return out
}

// All returns all registered limitation profiles in the order they appear in §B.3.
func (r *LimitationRegistry) All() []LimitationProfile {
	out := make([]LimitationProfile, len(r.profiles))
	copy(out, r.profiles)
	return out
}

// SeedLimitationCount is the number of named limitations defined in §B.3.
const SeedLimitationCount = 4

// SeedLimitationSlugs lists all limitation slugs in the order they appear in §B.3.
var SeedLimitationSlugs = []string{
	"static_snapshot",
	"reverse_engineering_epistemology",
	"single_system_analysis",
	"openclaw_snapshot",
}

// seedLimitationProfiles returns the canonical slice of LimitationProfile values
// as defined in Appendix B §B.3 of the paper.
func seedLimitationProfiles() []LimitationProfile {
	return []LimitationProfile{
		{
			Slug:       "static_snapshot",
			Label:      "Static snapshot.",
			PDFSection: "Appendix B §B.3",
			LimitationType: LimitationTypeTemporal,
			Description: "Analysis reflects one version (v2.1.88). Feature flags " +
				"(e.g., TRANSCRIPT_CLASSIFIER, CONTEXT_COLLAPSE) create build-time " +
				"variability; different build targets may produce functionally different " +
				"applications.",
			MitigationNotes: "Claims are scoped to v2.1.88 explicitly; feature-flag-gated " +
				"behaviour is identified and labelled accordingly throughout the paper.",
			AffectsClaimType:             AffectedClaimTypeAll,
			IsEpistemological:            false,
			RequiresHedgedGeneralisation: true,
		},
		{
			Slug:       "reverse_engineering_epistemology",
			Label:      "Reverse-engineering epistemology.",
			PDFSection: "Appendix B §B.3",
			LimitationType: LimitationTypeEpistemology,
			Description: "Source code reveals implemented structure, control flow, dependencies, " +
				"and feature gates. It cannot confirm design intent, enabled production flags, " +
				"runtime prevalence, or unshipped behaviour.",
			MitigationNotes: "Claims about design intent are qualified with Tier A evidence " +
				"(product documentation) where available; Tier C (reconstructed) language is " +
				"used for inferred intent.",
			AffectsClaimType:             AffectedClaimTypeDesignIntent,
			IsEpistemological:            true,
			RequiresHedgedGeneralisation: true,
		},
		{
			Slug:       "single_system_analysis",
			Label:      "Single-system analysis.",
			PDFSection: "Appendix B §B.3",
			LimitationType: LimitationTypeScope,
			Description: "Findings describe Claude Code's design space, not the entire design " +
				"space of coding agents. Generalisations are bounded.",
			MitigationNotes: "Comparative observations are marked explicitly; the paper " +
				"frames its contribution as a single-system case study rather than a " +
				"cross-system survey.",
			AffectsClaimType:             AffectedClaimTypeComparative,
			IsEpistemological:            false,
			RequiresHedgedGeneralisation: true,
		},
		{
			Slug:       "openclaw_snapshot",
			Label:      "OpenClaw snapshot.",
			PDFSection: "Appendix B §B.3",
			LimitationType: LimitationTypeCalibration,
			Description: "The OpenClaw analysis reflects a specific development state and may " +
				"not represent its current capabilities.",
			MitigationNotes: "OpenClaw is used for structural calibration only (Tier C); " +
				"it is not cited as ground truth and its snapshot date is acknowledged as " +
				"a bounding condition.",
			AffectsClaimType:             AffectedClaimTypeComparative,
			IsEpistemological:            false,
			RequiresHedgedGeneralisation: true,
		},
	}
}
