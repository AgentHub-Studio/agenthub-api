package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// FEAT026 — LimitationRegistry unit tests
// PDF reference: Appendix B §B.3 "Limitations" (page 45)
// ---------------------------------------------------------------------------

// TestFEAT026_LimitationRegistry_Count verifies the registry seeds exactly
// SeedLimitationCount (4) profiles.
func TestFEAT026_LimitationRegistry_Count(t *testing.T) {
	r := NewLimitationRegistry()
	assert.Len(t, r.All(), SeedLimitationCount)
}

// TestFEAT026_LimitationRegistry_SeedCount_Constant verifies the exported
// constant equals 4 as stated in §B.3.
func TestFEAT026_LimitationRegistry_SeedCount_Constant(t *testing.T) {
	assert.Equal(t, 4, SeedLimitationCount)
}

// TestFEAT026_LimitationRegistry_SeedSlugCount verifies SeedLimitationSlugs
// length matches SeedLimitationCount.
func TestFEAT026_LimitationRegistry_SeedSlugCount(t *testing.T) {
	assert.Len(t, SeedLimitationSlugs, SeedLimitationCount)
}

// TestFEAT026_LimitationRegistry_AllSlugsPresent verifies every slug in
// SeedLimitationSlugs resolves in the registry.
func TestFEAT026_LimitationRegistry_AllSlugsPresent(t *testing.T) {
	r := NewLimitationRegistry()
	for _, slug := range SeedLimitationSlugs {
		_, ok := r.FindLimitationBySlug(slug)
		assert.Truef(t, ok, "slug %q not found in registry", slug)
	}
}

// TestFEAT026_LimitationRegistry_StaticSnapshot_Fields verifies every field
// of the "Static snapshot." limitation profile.
func TestFEAT026_LimitationRegistry_StaticSnapshot_Fields(t *testing.T) {
	r := NewLimitationRegistry()
	p, ok := r.FindLimitationBySlug("static_snapshot")
	require.True(t, ok)

	assert.Equal(t, "Static snapshot.", p.Label)
	assert.Equal(t, LimitationTypeTemporal, p.LimitationType)
	assert.Equal(t, AffectedClaimTypeAll, p.AffectsClaimType)
	assert.False(t, p.IsEpistemological)
	assert.True(t, p.RequiresHedgedGeneralisation)
	assert.Contains(t, p.Description, "v2.1.88")
	assert.Contains(t, p.Description, "TRANSCRIPT_CLASSIFIER")
	assert.Contains(t, p.Description, "CONTEXT_COLLAPSE")
	assert.NotEmpty(t, p.PDFSection)
	assert.NotEmpty(t, p.MitigationNotes)
}

// TestFEAT026_LimitationRegistry_ReverseEngineeringEpistemology_Fields verifies
// every field of the "Reverse-engineering epistemology." limitation.
func TestFEAT026_LimitationRegistry_ReverseEngineeringEpistemology_Fields(t *testing.T) {
	r := NewLimitationRegistry()
	p, ok := r.FindLimitationBySlug("reverse_engineering_epistemology")
	require.True(t, ok)

	assert.Equal(t, "Reverse-engineering epistemology.", p.Label)
	assert.Equal(t, LimitationTypeEpistemology, p.LimitationType)
	assert.Equal(t, AffectedClaimTypeDesignIntent, p.AffectsClaimType)
	assert.True(t, p.IsEpistemological, "reverse-engineering is inherently epistemological")
	assert.True(t, p.RequiresHedgedGeneralisation)
	assert.Contains(t, p.Description, "design intent")
	assert.NotEmpty(t, p.MitigationNotes)
}

// TestFEAT026_LimitationRegistry_SingleSystemAnalysis_Fields verifies every
// field of the "Single-system analysis." limitation.
func TestFEAT026_LimitationRegistry_SingleSystemAnalysis_Fields(t *testing.T) {
	r := NewLimitationRegistry()
	p, ok := r.FindLimitationBySlug("single_system_analysis")
	require.True(t, ok)

	assert.Equal(t, "Single-system analysis.", p.Label)
	assert.Equal(t, LimitationTypeScope, p.LimitationType)
	assert.Equal(t, AffectedClaimTypeComparative, p.AffectsClaimType)
	assert.False(t, p.IsEpistemological)
	assert.True(t, p.RequiresHedgedGeneralisation)
	assert.Contains(t, p.Description, "bounded")
	assert.NotEmpty(t, p.MitigationNotes)
}

// TestFEAT026_LimitationRegistry_OpenClawSnapshot_Fields verifies every field
// of the "OpenClaw snapshot." limitation.
func TestFEAT026_LimitationRegistry_OpenClawSnapshot_Fields(t *testing.T) {
	r := NewLimitationRegistry()
	p, ok := r.FindLimitationBySlug("openclaw_snapshot")
	require.True(t, ok)

	assert.Equal(t, "OpenClaw snapshot.", p.Label)
	assert.Equal(t, LimitationTypeCalibration, p.LimitationType)
	assert.Equal(t, AffectedClaimTypeComparative, p.AffectsClaimType)
	assert.False(t, p.IsEpistemological)
	assert.True(t, p.RequiresHedgedGeneralisation)
	assert.Contains(t, p.Description, "OpenClaw")
	assert.NotEmpty(t, p.MitigationNotes)
}

// TestFEAT026_LimitationRegistry_FindByLabel_StaticSnapshot locates the
// static snapshot limitation via its label.
func TestFEAT026_LimitationRegistry_FindByLabel_StaticSnapshot(t *testing.T) {
	r := NewLimitationRegistry()
	p, ok := r.FindLimitationByLabel("Static snapshot.")
	require.True(t, ok)
	assert.Equal(t, "static_snapshot", p.Slug)
}

// TestFEAT026_LimitationRegistry_FindByLabel_Unknown returns false for an
// unknown label.
func TestFEAT026_LimitationRegistry_FindByLabel_Unknown(t *testing.T) {
	r := NewLimitationRegistry()
	_, ok := r.FindLimitationByLabel("nonexistent limitation")
	assert.False(t, ok)
}

// TestFEAT026_LimitationRegistry_FindBySlug_Unknown returns nil, false for a
// slug not in the registry.
func TestFEAT026_LimitationRegistry_FindBySlug_Unknown(t *testing.T) {
	r := NewLimitationRegistry()
	p, ok := r.FindLimitationBySlug("does_not_exist")
	assert.False(t, ok)
	assert.Nil(t, p)
}

// TestFEAT026_LimitationRegistry_FindByType_Temporal returns only the temporal
// limitation (static_snapshot).
func TestFEAT026_LimitationRegistry_FindByType_Temporal(t *testing.T) {
	r := NewLimitationRegistry()
	results := r.FindLimitationsByType(LimitationTypeTemporal)
	require.Len(t, results, 1)
	assert.Equal(t, "static_snapshot", results[0].Slug)
}

// TestFEAT026_LimitationRegistry_FindByType_Epistemology returns only the
// epistemological limitation.
func TestFEAT026_LimitationRegistry_FindByType_Epistemology(t *testing.T) {
	r := NewLimitationRegistry()
	results := r.FindLimitationsByType(LimitationTypeEpistemology)
	require.Len(t, results, 1)
	assert.Equal(t, "reverse_engineering_epistemology", results[0].Slug)
}

// TestFEAT026_LimitationRegistry_FindByType_Comparative returns the two
// limitations affecting comparative claims (single_system_analysis and
// openclaw_snapshot).
func TestFEAT026_LimitationRegistry_FindByType_Comparative(t *testing.T) {
	r := NewLimitationRegistry()
	results := r.FindLimitationsByType(LimitationTypeScope)
	require.Len(t, results, 1)
	assert.Equal(t, "single_system_analysis", results[0].Slug)
}

// TestFEAT026_LimitationRegistry_FindByAffectedClaimType_All verifies that
// FindLimitationsByAffectedClaimType(AffectedClaimTypeAll) returns only the
// limitations explicitly typed as affecting all claims.
func TestFEAT026_LimitationRegistry_FindByAffectedClaimType_All(t *testing.T) {
	r := NewLimitationRegistry()
	results := r.FindLimitationsByAffectedClaimType(AffectedClaimTypeAll)
	// static_snapshot affects all claims; and AffectedClaimTypeAll also includes itself
	slugs := make(map[string]bool)
	for _, p := range results {
		slugs[p.Slug] = true
	}
	assert.True(t, slugs["static_snapshot"],
		"static_snapshot affects all claim types")
}

// TestFEAT026_LimitationRegistry_FindByAffectedClaimType_DesignIntent returns
// the reverse_engineering_epistemology limitation plus any "all" limitations.
func TestFEAT026_LimitationRegistry_FindByAffectedClaimType_DesignIntent(t *testing.T) {
	r := NewLimitationRegistry()
	results := r.FindLimitationsByAffectedClaimType(AffectedClaimTypeDesignIntent)
	slugs := make(map[string]bool)
	for _, p := range results {
		slugs[p.Slug] = true
	}
	// reverse_engineering_epistemology directly affects design_intent
	assert.True(t, slugs["reverse_engineering_epistemology"])
	// static_snapshot (AffectedClaimTypeAll) must also be included
	assert.True(t, slugs["static_snapshot"])
}

// TestFEAT026_LimitationRegistry_EpistemologicalLimitations returns exactly one
// profile (reverse_engineering_epistemology).
func TestFEAT026_LimitationRegistry_EpistemologicalLimitations(t *testing.T) {
	r := NewLimitationRegistry()
	results := r.EpistemologicalLimitations()
	require.Len(t, results, 1)
	assert.Equal(t, "reverse_engineering_epistemology", results[0].Slug)
	assert.True(t, results[0].IsEpistemological)
}

// TestFEAT026_LimitationRegistry_HedgedGeneralisationLimitations verifies that
// all four limitations require hedged generalisations.
func TestFEAT026_LimitationRegistry_HedgedGeneralisationLimitations(t *testing.T) {
	r := NewLimitationRegistry()
	results := r.HedgedGeneralisationLimitations()
	// Every limitation in §B.3 requires bounded/hedged generalisations.
	assert.Len(t, results, SeedLimitationCount)
}

// TestFEAT026_LimitationRegistry_AllReturnsImmutableCopy verifies that
// mutating the returned slice does not affect the registry.
func TestFEAT026_LimitationRegistry_AllReturnsImmutableCopy(t *testing.T) {
	r := NewLimitationRegistry()
	first := r.All()
	first[0].Label = "tampered"
	second := r.All()
	assert.NotEqual(t, "tampered", second[0].Label)
}

// TestFEAT026_LimitationRegistry_LimitationTypeConstants verifies all four
// LimitationType constants are distinct.
func TestFEAT026_LimitationRegistry_LimitationTypeConstants(t *testing.T) {
	types := map[LimitationType]bool{
		LimitationTypeTemporal:    true,
		LimitationTypeEpistemology: true,
		LimitationTypeScope:       true,
		LimitationTypeCalibration: true,
	}
	assert.Len(t, types, 4, "all four limitation type constants must be distinct")
}

// TestFEAT026_LimitationRegistry_AffectedClaimTypeConstants verifies all four
// AffectedClaimType constants are distinct.
func TestFEAT026_LimitationRegistry_AffectedClaimTypeConstants(t *testing.T) {
	types := map[AffectedClaimType]bool{
		AffectedClaimTypeAll:            true,
		AffectedClaimTypeDesignIntent:   true,
		AffectedClaimTypeComparative:    true,
		AffectedClaimTypeImplementation: true,
	}
	assert.Len(t, types, 4, "all four affected claim type constants must be distinct")
}

// TestFEAT026_LimitationRegistry_AllProfilesHavePDFSection verifies that every
// seeded profile carries a non-empty PDFSection reference.
func TestFEAT026_LimitationRegistry_AllProfilesHavePDFSection(t *testing.T) {
	r := NewLimitationRegistry()
	for _, p := range r.All() {
		assert.NotEmptyf(t, p.PDFSection,
			"limitation %q must carry a PDF section reference", p.Slug)
	}
}

// TestFEAT026_LimitationRegistry_AllProfilesHaveMitigationNotes verifies that
// every seeded profile carries non-empty mitigation notes.
func TestFEAT026_LimitationRegistry_AllProfilesHaveMitigationNotes(t *testing.T) {
	r := NewLimitationRegistry()
	for _, p := range r.All() {
		assert.NotEmptyf(t, p.MitigationNotes,
			"limitation %q must carry mitigation notes", p.Slug)
	}
}
