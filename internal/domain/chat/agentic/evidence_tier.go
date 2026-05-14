package agentic

// EvidenceTierRegistry maps the three epistemological evidence tiers used in the
// Claude Code architecture paper (2604.14228v1) to classify the strength and
// provenance of every claim made about the system.
//
// PDF reference: Appendix B §B.1 "Evidence Base and Evidence Tiers" (page 45).
//
// The source corpus analysed by the paper comprises approximately 1,884 files
// totalling roughly 512K lines of TypeScript (v2.1.88 npm extraction). Claims are
// grounded at exactly one of the three tiers; higher-tier evidence supersedes
// lower-tier evidence when they conflict.

// EvidenceTierLabel is the canonical single-letter label used in the paper
// ("Tier A", "Tier B", "Tier C").
type EvidenceTierLabel string

const (
	// EvidenceTierLabelA is the product-documented tier.
	EvidenceTierLabelA EvidenceTierLabel = "A"
	// EvidenceTierLabelB is the code-verified tier (strongest).
	EvidenceTierLabelB EvidenceTierLabel = "B"
	// EvidenceTierLabelC is the reconstructed / inferred tier (hedged).
	EvidenceTierLabelC EvidenceTierLabel = "C"
)

// EvidenceStrength ranks the epistemic confidence of a claim.
type EvidenceStrength string

const (
	// EvidenceStrengthStrong — code-verified; direct inspection of specific files and
	// functions from the extracted TypeScript codebase.
	EvidenceStrengthStrong EvidenceStrength = "strong"
	// EvidenceStrengthModerate — product-documented; draws on official Anthropic
	// documentation and engineering publications. Establishes product intent but may
	// not reflect internal implementation.
	EvidenceStrengthModerate EvidenceStrength = "moderate"
	// EvidenceStrengthSpeculative — reconstructed; derived from community analysis,
	// OpenClaw structural comparison, or inference from code patterns. Stated with
	// hedging language.
	EvidenceStrengthSpeculative EvidenceStrength = "speculative"
)

// EvidenceTierProfile captures the full description of one evidence tier as
// stated in Appendix B §B.1 of the paper.
type EvidenceTierProfile struct {
	// Slug is a stable lowercase identifier for the tier (e.g. "tier_a").
	Slug string

	// Label is the single-letter tier label used in the paper ("A", "B", or "C").
	Label EvidenceTierLabel

	// Name is the parenthetical short name used alongside the tier label.
	// Example: "product-documented", "code-verified", "reconstructed".
	Name string

	// PDFSection is the appendix location in the paper.
	PDFSection string

	// Description is the full prose definition from §B.1.
	Description string

	// Strength is the ranked epistemic confidence of claims at this tier.
	Strength EvidenceStrength

	// TypicalSource describes where evidence at this tier originates.
	TypicalSource string

	// IsStrongest is true only for the tier designated "the strongest evidence
	// tier" in the paper (Tier B).
	IsStrongest bool

	// RequiresHedgingLanguage indicates whether the paper mandates hedged
	// phrasing for claims at this tier (true for Tier C).
	RequiresHedgingLanguage bool

	// CanSupportDirectClaims is true when the tier can anchor unqualified
	// assertions about the system's behaviour (Tiers A and B).
	CanSupportDirectClaims bool
}

// EvidenceTierRegistry is an in-memory registry of the three evidence tiers
// described in Appendix B §B.1 of the Claude Code architecture paper.
type EvidenceTierRegistry struct {
	profiles []EvidenceTierProfile
}

// NewEvidenceTierRegistry constructs and returns the registry pre-seeded with
// the three tiers exactly as defined in §B.1.
func NewEvidenceTierRegistry() *EvidenceTierRegistry {
	return &EvidenceTierRegistry{
		profiles: seedEvidenceTierProfiles(),
	}
}

// FindEvidenceTierBySlug returns the profile whose Slug matches the given value,
// together with a boolean indicating whether the profile was found.
func (r *EvidenceTierRegistry) FindEvidenceTierBySlug(slug string) (*EvidenceTierProfile, bool) {
	for i := range r.profiles {
		if r.profiles[i].Slug == slug {
			return &r.profiles[i], true
		}
	}
	return nil, false
}

// FindEvidenceTierByLabel returns the profile whose Label matches (e.g. "A").
func (r *EvidenceTierRegistry) FindEvidenceTierByLabel(label EvidenceTierLabel) (*EvidenceTierProfile, bool) {
	for i := range r.profiles {
		if r.profiles[i].Label == label {
			return &r.profiles[i], true
		}
	}
	return nil, false
}

// All returns all registered evidence tier profiles in ascending tier order
// (Tier A → Tier B → Tier C).
func (r *EvidenceTierRegistry) All() []EvidenceTierProfile {
	out := make([]EvidenceTierProfile, len(r.profiles))
	copy(out, r.profiles)
	return out
}

// DirectClaimTiers returns the tiers that can anchor unqualified (non-hedged)
// assertions about the system (Tiers A and B).
func (r *EvidenceTierRegistry) DirectClaimTiers() []EvidenceTierProfile {
	var out []EvidenceTierProfile
	for _, p := range r.profiles {
		if p.CanSupportDirectClaims {
			out = append(out, p)
		}
	}
	return out
}

// HedgedTiers returns the tiers that require hedging language when cited (Tier C).
func (r *EvidenceTierRegistry) HedgedTiers() []EvidenceTierProfile {
	var out []EvidenceTierProfile
	for _, p := range r.profiles {
		if p.RequiresHedgingLanguage {
			out = append(out, p)
		}
	}
	return out
}

// StrongestTier returns the tier designated as "the strongest evidence tier" in
// the paper. Per §B.1 this is always Tier B (code-verified).
func (r *EvidenceTierRegistry) StrongestTier() (*EvidenceTierProfile, bool) {
	for i := range r.profiles {
		if r.profiles[i].IsStrongest {
			return &r.profiles[i], true
		}
	}
	return nil, false
}

// SeedEvidenceTierCount is the number of evidence tiers defined in §B.1.
const SeedEvidenceTierCount = 3

// SeedEvidenceTierSlugs lists all tier slugs in the order they appear in §B.1.
var SeedEvidenceTierSlugs = []string{
	"tier_a",
	"tier_b",
	"tier_c",
}

// seedEvidenceTierProfiles returns the canonical slice of EvidenceTierProfile
// values as defined in Appendix B §B.1 of the paper.
func seedEvidenceTierProfiles() []EvidenceTierProfile {
	return []EvidenceTierProfile{
		{
			Slug:       "tier_a",
			Label:      EvidenceTierLabelA,
			Name:       "product-documented",
			PDFSection: "Appendix B §B.1",
			Description: "Claims drawn from official Anthropic documentation and engineering " +
				"publications. These establish product intent but may not reflect internal " +
				"implementation.",
			Strength:                EvidenceStrengthModerate,
			TypicalSource:           "Official Anthropic documentation and engineering publications",
			IsStrongest:             false,
			RequiresHedgingLanguage: false,
			CanSupportDirectClaims:  true,
		},
		{
			Slug:       "tier_b",
			Label:      EvidenceTierLabelB,
			Name:       "code-verified",
			PDFSection: "Appendix B §B.1",
			Description: "Claims citing specific files and functions in the extracted TypeScript " +
				"codebase (v2.1.88, obtained from a publicly available npm package extraction). " +
				"This is the strongest evidence tier.",
			Strength:                EvidenceStrengthStrong,
			TypicalSource:           "Specific files and functions in the extracted TypeScript codebase v2.1.88 (npm extraction)",
			IsStrongest:             true,
			RequiresHedgingLanguage: false,
			CanSupportDirectClaims:  true,
		},
		{
			Slug:       "tier_c",
			Label:      EvidenceTierLabelC,
			Name:       "reconstructed",
			PDFSection: "Appendix B §B.1",
			Description: "Claims derived from community analysis, OpenClaw structural comparison, " +
				"or inference from code patterns. These are stated with hedging language.",
			Strength:                EvidenceStrengthSpeculative,
			TypicalSource:           "Community analysis, OpenClaw structural comparison, or inference from code patterns",
			IsStrongest:             false,
			RequiresHedgingLanguage: true,
			CanSupportDirectClaims:  false,
		},
	}
}
