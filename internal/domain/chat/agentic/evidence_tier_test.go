package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// FEAT025 — EvidenceTierRegistry unit tests
// PDF reference: Appendix B §B.1 "Evidence Base and Evidence Tiers" (page 45)
// ---------------------------------------------------------------------------

// TestFEAT025_EvidenceTierRegistry_Count verifies the registry seeds exactly
// SeedEvidenceTierCount (3) tiers.
func TestFEAT025_EvidenceTierRegistry_Count(t *testing.T) {
	r := NewEvidenceTierRegistry()
	assert.Len(t, r.All(), SeedEvidenceTierCount)
}

// TestFEAT025_EvidenceTierRegistry_SeedCount_Constant verifies the exported
// constant equals 3 as stated in §B.1.
func TestFEAT025_EvidenceTierRegistry_SeedCount_Constant(t *testing.T) {
	assert.Equal(t, 3, SeedEvidenceTierCount)
}

// TestFEAT025_EvidenceTierRegistry_AllSlugsPresent verifies every slug in
// SeedEvidenceTierSlugs resolves in the registry.
func TestFEAT025_EvidenceTierRegistry_AllSlugsPresent(t *testing.T) {
	r := NewEvidenceTierRegistry()
	for _, slug := range SeedEvidenceTierSlugs {
		_, ok := r.FindEvidenceTierBySlug(slug)
		assert.Truef(t, ok, "slug %q not found in registry", slug)
	}
}

// TestFEAT025_EvidenceTierRegistry_SeedSlugCount verifies the SeedEvidenceTierSlugs
// slice length matches SeedEvidenceTierCount.
func TestFEAT025_EvidenceTierRegistry_SeedSlugCount(t *testing.T) {
	assert.Len(t, SeedEvidenceTierSlugs, SeedEvidenceTierCount)
}

// TestFEAT025_EvidenceTierRegistry_TierA_Fields verifies every field of the
// Tier A (product-documented) profile.
func TestFEAT025_EvidenceTierRegistry_TierA_Fields(t *testing.T) {
	r := NewEvidenceTierRegistry()
	p, ok := r.FindEvidenceTierBySlug("tier_a")
	require.True(t, ok)

	assert.Equal(t, EvidenceTierLabelA, p.Label)
	assert.Equal(t, "product-documented", p.Name)
	assert.Equal(t, EvidenceStrengthModerate, p.Strength)
	assert.False(t, p.IsStrongest)
	assert.False(t, p.RequiresHedgingLanguage)
	assert.True(t, p.CanSupportDirectClaims)
	assert.NotEmpty(t, p.PDFSection)
	assert.NotEmpty(t, p.Description)
	assert.NotEmpty(t, p.TypicalSource)
}

// TestFEAT025_EvidenceTierRegistry_TierB_Fields verifies every field of the
// Tier B (code-verified) profile, including IsStrongest=true.
func TestFEAT025_EvidenceTierRegistry_TierB_Fields(t *testing.T) {
	r := NewEvidenceTierRegistry()
	p, ok := r.FindEvidenceTierBySlug("tier_b")
	require.True(t, ok)

	assert.Equal(t, EvidenceTierLabelB, p.Label)
	assert.Equal(t, "code-verified", p.Name)
	assert.Equal(t, EvidenceStrengthStrong, p.Strength)
	assert.True(t, p.IsStrongest, "Tier B must be the strongest evidence tier per §B.1")
	assert.False(t, p.RequiresHedgingLanguage)
	assert.True(t, p.CanSupportDirectClaims)
}

// TestFEAT025_EvidenceTierRegistry_TierC_Fields verifies every field of the
// Tier C (reconstructed) profile, including RequiresHedgingLanguage=true.
func TestFEAT025_EvidenceTierRegistry_TierC_Fields(t *testing.T) {
	r := NewEvidenceTierRegistry()
	p, ok := r.FindEvidenceTierBySlug("tier_c")
	require.True(t, ok)

	assert.Equal(t, EvidenceTierLabelC, p.Label)
	assert.Equal(t, "reconstructed", p.Name)
	assert.Equal(t, EvidenceStrengthSpeculative, p.Strength)
	assert.False(t, p.IsStrongest)
	assert.True(t, p.RequiresHedgingLanguage, "Tier C claims must use hedging language per §B.1")
	assert.False(t, p.CanSupportDirectClaims)
}

// TestFEAT025_EvidenceTierRegistry_FindByLabel_A locates Tier A via label.
func TestFEAT025_EvidenceTierRegistry_FindByLabel_A(t *testing.T) {
	r := NewEvidenceTierRegistry()
	p, ok := r.FindEvidenceTierByLabel(EvidenceTierLabelA)
	require.True(t, ok)
	assert.Equal(t, "tier_a", p.Slug)
}

// TestFEAT025_EvidenceTierRegistry_FindByLabel_B locates Tier B via label.
func TestFEAT025_EvidenceTierRegistry_FindByLabel_B(t *testing.T) {
	r := NewEvidenceTierRegistry()
	p, ok := r.FindEvidenceTierByLabel(EvidenceTierLabelB)
	require.True(t, ok)
	assert.Equal(t, "tier_b", p.Slug)
}

// TestFEAT025_EvidenceTierRegistry_FindByLabel_C locates Tier C via label.
func TestFEAT025_EvidenceTierRegistry_FindByLabel_C(t *testing.T) {
	r := NewEvidenceTierRegistry()
	p, ok := r.FindEvidenceTierByLabel(EvidenceTierLabelC)
	require.True(t, ok)
	assert.Equal(t, "tier_c", p.Slug)
}

// TestFEAT025_EvidenceTierRegistry_FindByLabel_Unknown returns false for an
// unknown label.
func TestFEAT025_EvidenceTierRegistry_FindByLabel_Unknown(t *testing.T) {
	r := NewEvidenceTierRegistry()
	_, ok := r.FindEvidenceTierByLabel("Z")
	assert.False(t, ok)
}

// TestFEAT025_EvidenceTierRegistry_FindBySlug_Unknown returns nil, false for a
// slug not in the registry.
func TestFEAT025_EvidenceTierRegistry_FindBySlug_Unknown(t *testing.T) {
	r := NewEvidenceTierRegistry()
	p, ok := r.FindEvidenceTierBySlug("tier_x")
	assert.False(t, ok)
	assert.Nil(t, p)
}

// TestFEAT025_EvidenceTierRegistry_StrongestTier verifies StrongestTier returns
// Tier B and marks exactly one tier as strongest.
func TestFEAT025_EvidenceTierRegistry_StrongestTier(t *testing.T) {
	r := NewEvidenceTierRegistry()
	p, ok := r.StrongestTier()
	require.True(t, ok)
	assert.Equal(t, EvidenceTierLabelB, p.Label)
	assert.Equal(t, "tier_b", p.Slug)

	// Exactly one tier must carry IsStrongest=true.
	count := 0
	for _, tier := range r.All() {
		if tier.IsStrongest {
			count++
		}
	}
	assert.Equal(t, 1, count, "exactly one tier must be the strongest")
}

// TestFEAT025_EvidenceTierRegistry_DirectClaimTiers verifies Tiers A and B
// can support unqualified claims and Tier C cannot.
func TestFEAT025_EvidenceTierRegistry_DirectClaimTiers(t *testing.T) {
	r := NewEvidenceTierRegistry()
	direct := r.DirectClaimTiers()
	assert.Len(t, direct, 2)

	labels := make(map[EvidenceTierLabel]bool)
	for _, p := range direct {
		labels[p.Label] = true
	}
	assert.True(t, labels[EvidenceTierLabelA])
	assert.True(t, labels[EvidenceTierLabelB])
	assert.False(t, labels[EvidenceTierLabelC])
}

// TestFEAT025_EvidenceTierRegistry_HedgedTiers verifies only Tier C requires
// hedging language.
func TestFEAT025_EvidenceTierRegistry_HedgedTiers(t *testing.T) {
	r := NewEvidenceTierRegistry()
	hedged := r.HedgedTiers()
	require.Len(t, hedged, 1)
	assert.Equal(t, EvidenceTierLabelC, hedged[0].Label)
}

// TestFEAT025_EvidenceTierRegistry_AllReturnsImmutableCopy verifies that
// mutating the returned slice does not affect the registry.
func TestFEAT025_EvidenceTierRegistry_AllReturnsImmutableCopy(t *testing.T) {
	r := NewEvidenceTierRegistry()
	first := r.All()
	first[0].Name = "tampered"
	second := r.All()
	assert.NotEqual(t, "tampered", second[0].Name)
}

// TestFEAT025_EvidenceTierRegistry_EvidenceStrengthOrdering verifies that the
// three EvidenceStrength constants are distinct.
func TestFEAT025_EvidenceTierRegistry_EvidenceStrengthOrdering(t *testing.T) {
	strengths := map[EvidenceStrength]bool{
		EvidenceStrengthStrong:      true,
		EvidenceStrengthModerate:    true,
		EvidenceStrengthSpeculative: true,
	}
	assert.Len(t, strengths, 3, "all three strength constants must be distinct")
}

// TestFEAT025_EvidenceTierRegistry_TierB_DescriptionContainsVersion ensures
// the Tier B description references the npm package version v2.1.88 as stated
// in §B.1.
func TestFEAT025_EvidenceTierRegistry_TierB_DescriptionContainsVersion(t *testing.T) {
	r := NewEvidenceTierRegistry()
	p, ok := r.FindEvidenceTierBySlug("tier_b")
	require.True(t, ok)
	assert.Contains(t, p.Description, "v2.1.88")
}

// TestFEAT025_EvidenceTierRegistry_TierC_DescriptionMentionsOpenClaw ensures
// the Tier C description references OpenClaw as a calibration source, per §B.1.
func TestFEAT025_EvidenceTierRegistry_TierC_DescriptionMentionsOpenClaw(t *testing.T) {
	r := NewEvidenceTierRegistry()
	p, ok := r.FindEvidenceTierBySlug("tier_c")
	require.True(t, ok)
	assert.Contains(t, p.Description, "OpenClaw")
}

// TestFEAT025_EvidenceTierRegistry_LabelConstants verifies the three label
// constants hold the expected single-letter values.
func TestFEAT025_EvidenceTierRegistry_LabelConstants(t *testing.T) {
	assert.Equal(t, EvidenceTierLabel("A"), EvidenceTierLabelA)
	assert.Equal(t, EvidenceTierLabel("B"), EvidenceTierLabelB)
	assert.Equal(t, EvidenceTierLabel("C"), EvidenceTierLabelC)
}
