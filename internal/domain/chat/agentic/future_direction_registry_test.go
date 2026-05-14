package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Unit tests — FEAT027
// ---------------------------------------------------------------------------

func TestFEAT027_SeedCount(t *testing.T) {
	// §12 defines exactly 6 future directions (12.1–12.6).
	assert.Equal(t, 6, SeedFutureDirectionCount,
		"SeedFutureDirectionCount must equal 6 per §12")
}

func TestFEAT027_SeedSlugsLength(t *testing.T) {
	assert.Len(t, SeedFutureDirectionSlugs, SeedFutureDirectionCount,
		"SeedFutureDirectionSlugs must have the same length as SeedFutureDirectionCount")
}

func TestFEAT027_AllReturnsAllProfiles(t *testing.T) {
	r := NewFutureDirectionRegistry()
	all := r.AllFutureDirections()
	assert.Len(t, all, SeedFutureDirectionCount,
		"AllFutureDirections must return exactly SeedFutureDirectionCount profiles")
}

func TestFEAT027_AllReturnsCopy(t *testing.T) {
	r := NewFutureDirectionRegistry()
	a := r.AllFutureDirections()
	b := r.AllFutureDirections()
	a[0].Slug = "mutated"
	assert.NotEqual(t, "mutated", b[0].Slug,
		"AllFutureDirections must return an independent copy")
}

func TestFEAT027_SectionOrderPreserved(t *testing.T) {
	r := NewFutureDirectionRegistry()
	all := r.AllFutureDirections()
	sections := []string{"12.1", "12.2", "12.3", "12.4", "12.5", "12.6"}
	require.Len(t, all, len(sections))
	for i, want := range sections {
		assert.Equal(t, want, all[i].PDFSection,
			"profile[%d] must have PDFSection %q in §12 order", i, want)
	}
}

func TestFEAT027_FindBySlugHit(t *testing.T) {
	r := NewFutureDirectionRegistry()
	for _, slug := range SeedFutureDirectionSlugs {
		p, ok := r.FindFutureDirectionBySlug(slug)
		require.True(t, ok, "FindFutureDirectionBySlug must find seeded slug %q", slug)
		assert.Equal(t, slug, p.Slug)
	}
}

func TestFEAT027_FindBySlugMiss(t *testing.T) {
	r := NewFutureDirectionRegistry()
	_, ok := r.FindFutureDirectionBySlug("nonexistent_direction")
	assert.False(t, ok, "FindFutureDirectionBySlug must return false for unknown slug")
}

func TestFEAT027_FindBySlugReturnsMutablePointer(t *testing.T) {
	r := NewFutureDirectionRegistry()
	p, ok := r.FindFutureDirectionBySlug("observability_evaluation_gap")
	require.True(t, ok)
	assert.NotNil(t, p)
}

func TestFEAT027_AllProfilesHaveNonEmptyLabels(t *testing.T) {
	r := NewFutureDirectionRegistry()
	for _, p := range r.AllFutureDirections() {
		assert.NotEmpty(t, p.Label, "profile %q must have a non-empty Label", p.Slug)
	}
}

func TestFEAT027_AllProfilesHaveNonEmptyPDFSection(t *testing.T) {
	r := NewFutureDirectionRegistry()
	for _, p := range r.AllFutureDirections() {
		assert.NotEmpty(t, p.PDFSection, "profile %q must have a non-empty PDFSection", p.Slug)
	}
}

func TestFEAT027_AllProfilesHaveNonEmptyCoreQuestion(t *testing.T) {
	r := NewFutureDirectionRegistry()
	for _, p := range r.AllFutureDirections() {
		assert.NotEmpty(t, p.CoreQuestion, "profile %q must have a non-empty CoreQuestion", p.Slug)
	}
}

func TestFEAT027_AllProfilesHaveNonEmptyArchitecturalGap(t *testing.T) {
	r := NewFutureDirectionRegistry()
	for _, p := range r.AllFutureDirections() {
		assert.NotEmpty(t, p.ArchitecturalGap,
			"profile %q must have a non-empty ArchitecturalGap", p.Slug)
	}
}

func TestFEAT027_ByValueDimensionSafety(t *testing.T) {
	// §12.1 targets Safety.
	r := NewFutureDirectionRegistry()
	safety := r.ByValueDimension(FutureDirectionDimSafety)
	require.Len(t, safety, 1, "exactly one direction targets the Safety dimension")
	assert.Equal(t, "observability_evaluation_gap", safety[0].Slug)
}

func TestFEAT027_ByValueDimensionReliability(t *testing.T) {
	// §12.2 and §12.4 both target Reliability.
	r := NewFutureDirectionRegistry()
	rel := r.ByValueDimension(FutureDirectionDimReliability)
	assert.Len(t, rel, 2, "two directions target the Reliability dimension (§12.2 and §12.4)")
	slugs := []string{rel[0].Slug, rel[1].Slug}
	assert.Contains(t, slugs, "persistence_longitudinal_colleague")
	assert.Contains(t, slugs, "horizon_scaling")
}

func TestFEAT027_ByValueDimensionCapability(t *testing.T) {
	// §12.3 targets Capability.
	r := NewFutureDirectionRegistry()
	cap := r.ByValueDimension(FutureDirectionDimCapability)
	require.Len(t, cap, 1, "exactly one direction targets the Capability dimension")
	assert.Equal(t, "harness_boundary_evolution", cap[0].Slug)
}

func TestFEAT027_ByValueDimensionAuthority(t *testing.T) {
	// §12.5 targets Authority.
	r := NewFutureDirectionRegistry()
	auth := r.ByValueDimension(FutureDirectionDimAuthority)
	require.Len(t, auth, 1, "exactly one direction targets the Authority dimension")
	assert.Equal(t, "governance_oversight_at_scale", auth[0].Slug)
}

func TestFEAT027_ByValueDimensionCrosscut(t *testing.T) {
	// §12.6 is crosscutting.
	r := NewFutureDirectionRegistry()
	cross := r.ByValueDimension(FutureDirectionDimCrosscut)
	require.Len(t, cross, 1, "exactly one direction is crosscutting (§12.6)")
	assert.Equal(t, "evaluative_lens_revisited", cross[0].Slug)
}

func TestFEAT027_ByHorizonNearTerm(t *testing.T) {
	r := NewFutureDirectionRegistry()
	near := r.ByHorizon(FutureDirectionHorizonNearTerm)
	require.Len(t, near, 1, "exactly one near-term direction (§12.1)")
	assert.Equal(t, "observability_evaluation_gap", near[0].Slug)
}

func TestFEAT027_ByHorizonMediumTerm(t *testing.T) {
	r := NewFutureDirectionRegistry()
	mid := r.ByHorizon(FutureDirectionHorizonMediumTerm)
	assert.Len(t, mid, 2, "two medium-term directions (§12.2 and §12.3)")
}

func TestFEAT027_ByHorizonLongTerm(t *testing.T) {
	r := NewFutureDirectionRegistry()
	long := r.ByHorizon(FutureDirectionHorizonLongTerm)
	assert.Len(t, long, 3, "three long-term directions (§12.4, §12.5, §12.6)")
}

func TestFEAT027_EvaluativeLensDirections(t *testing.T) {
	// Only §12.6 is the evaluative lens.
	r := NewFutureDirectionRegistry()
	lens := r.EvaluativeLensDirections()
	require.Len(t, lens, 1, "exactly one direction is the evaluative lens (§12.6)")
	assert.Equal(t, "evaluative_lens_revisited", lens[0].Slug)
	assert.True(t, lens[0].IsEvaluativeLens)
}

func TestFEAT027_NonEvaluativeLensDirectionsFlagFalse(t *testing.T) {
	r := NewFutureDirectionRegistry()
	for _, p := range r.AllFutureDirections() {
		if p.Slug == "evaluative_lens_revisited" {
			continue
		}
		assert.False(t, p.IsEvaluativeLens,
			"profile %q must NOT be flagged as evaluative lens", p.Slug)
	}
}

func TestFEAT027_ExternallyConstrainedDirections(t *testing.T) {
	// Only §12.5 requires external (regulatory) constraint.
	r := NewFutureDirectionRegistry()
	ext := r.ExternallyConstrainedDirections()
	require.Len(t, ext, 1, "exactly one direction requires external constraint (§12.5)")
	assert.Equal(t, "governance_oversight_at_scale", ext[0].Slug)
	assert.True(t, ext[0].RequiresExternalConstraint)
}

func TestFEAT027_DirectionsWithOpenMechanism(t *testing.T) {
	// §12.3 deliberately leaves the mechanism open.
	r := NewFutureDirectionRegistry()
	open := r.DirectionsWithOpenMechanism()
	require.Len(t, open, 1, "exactly one direction has an open (empty) mechanism (§12.3)")
	assert.Equal(t, "harness_boundary_evolution", open[0].Slug)
}

func TestFEAT027_AllSeedsReachableViaSlug(t *testing.T) {
	r := NewFutureDirectionRegistry()
	for _, slug := range SeedFutureDirectionSlugs {
		_, ok := r.FindFutureDirectionBySlug(slug)
		assert.True(t, ok, "seed slug %q must be reachable via FindFutureDirectionBySlug", slug)
	}
}

func TestFEAT027_NoDuplicateSlugs(t *testing.T) {
	r := NewFutureDirectionRegistry()
	seen := make(map[string]bool)
	for _, p := range r.AllFutureDirections() {
		assert.False(t, seen[p.Slug], "duplicate slug detected: %q", p.Slug)
		seen[p.Slug] = true
	}
}

func TestFEAT027_PDFSectionPrefixedWith12(t *testing.T) {
	r := NewFutureDirectionRegistry()
	for _, p := range r.AllFutureDirections() {
		assert.True(t, strings.HasPrefix(p.PDFSection, "12."),
			"profile %q PDFSection must start with '12.' (got %q)", p.Slug, p.PDFSection)
	}
}

// ---------------------------------------------------------------------------
// BDD tests — FEAT027
// ---------------------------------------------------------------------------

// TestFEAT027_BDD_FutureDirectionRegistry exercises the registry via
// Gherkin-style scenarios covering the six §12 open questions.

func TestFEAT027_BDD_SixDirectionsSeeded(t *testing.T) {
	// Scenario: §12 defines exactly six open future directions
	// Given a freshly constructed FutureDirectionRegistry
	r := NewFutureDirectionRegistry()

	// When I ask for all profiles
	all := r.AllFutureDirections()

	// Then exactly six are returned
	assert.Len(t, all, 6,
		"§12 defines exactly six open design questions")

	// And they cover sections 12.1 through 12.6
	for i, wantSection := range []string{"12.1", "12.2", "12.3", "12.4", "12.5", "12.6"} {
		assert.Equal(t, wantSection, all[i].PDFSection,
			"profile[%d] should be §%s", i, wantSection)
	}
}

func TestFEAT027_BDD_ObservabilityGapTargetsSafety(t *testing.T) {
	// Scenario: §12.1 targets the Safety value dimension
	// Given the registry
	r := NewFutureDirectionRegistry()

	// When I look up the observability–evaluation gap direction
	p, ok := r.FindFutureDirectionBySlug("observability_evaluation_gap")
	require.True(t, ok, "observability_evaluation_gap must be seeded")

	// Then its value dimension is Safety
	assert.Equal(t, FutureDirectionDimSafety, p.ValueDimension,
		"§12.1 primarily challenges the Safety value")

	// And its horizon is near-term
	assert.Equal(t, FutureDirectionHorizonNearTerm, p.Horizon,
		"§12.1 is addressable within current architecture extensions")

	// And the key mechanism references generator-evaluator separation
	assert.Contains(t, p.KeyMechanism, "generator-evaluator",
		"§12.1 key mechanism must reference generator-evaluator separation")
}

func TestFEAT027_BDD_GovernanceRequiresExternalConstraint(t *testing.T) {
	// Scenario: §12.5 governance direction is driven by external regulatory forces
	// Given the registry
	r := NewFutureDirectionRegistry()

	// When I look up the governance direction
	p, ok := r.FindFutureDirectionBySlug("governance_oversight_at_scale")
	require.True(t, ok, "governance_oversight_at_scale must be seeded")

	// Then RequiresExternalConstraint is true
	assert.True(t, p.RequiresExternalConstraint,
		"§12.5 is partly driven by external regulatory constraints (EU AI Act)")

	// And no other direction requires external constraint
	ext := r.ExternallyConstrainedDirections()
	assert.Len(t, ext, 1,
		"exactly one direction requires external constraint")
}

func TestFEAT027_BDD_EvaluativeLensIsUniqueToSection126(t *testing.T) {
	// Scenario: only §12.6 reframes the §2.4 evaluative lens as a design problem
	// Given the registry
	r := NewFutureDirectionRegistry()

	// When I query for evaluative-lens directions
	lens := r.EvaluativeLensDirections()

	// Then exactly one is returned and it is §12.6
	require.Len(t, lens, 1,
		"only §12.6 is the evaluative lens revisited")
	assert.Equal(t, "evaluative_lens_revisited", lens[0].Slug)
	assert.Equal(t, "12.6", lens[0].PDFSection)

	// And its value dimension is crosscutting (spans all five values)
	assert.Equal(t, FutureDirectionDimCrosscut, lens[0].ValueDimension,
		"§12.6 is crosscutting across all five design values")
}

func TestFEAT027_BDD_HarnessBoundaryHasOpenMechanism(t *testing.T) {
	// Scenario: §12.3 deliberately leaves the resolution mechanism unspecified
	// Given the registry
	r := NewFutureDirectionRegistry()

	// When I look up the harness boundary direction
	p, ok := r.FindFutureDirectionBySlug("harness_boundary_evolution")
	require.True(t, ok, "harness_boundary_evolution must be seeded")

	// Then its KeyMechanism is empty (paper deliberately leaves it open)
	assert.Empty(t, p.KeyMechanism,
		"§12.3 deliberately leaves the specific mechanism unspecified")

	// And it targets Capability
	assert.Equal(t, FutureDirectionDimCapability, p.ValueDimension,
		"§12.3 primarily challenges the Capability Amplification value")
}

func TestFEAT027_BDD_ReliabilityDirectionsAreSection122And124(t *testing.T) {
	// Scenario: two directions target Reliability — §12.2 (persistence) and §12.4 (horizon scaling)
	// Given the registry
	r := NewFutureDirectionRegistry()

	// When I query by Reliability dimension
	rel := r.ByValueDimension(FutureDirectionDimReliability)

	// Then exactly two are returned
	require.Len(t, rel, 2,
		"two §12 directions target Reliable Execution (§12.2 and §12.4)")

	// And they are §12.2 and §12.4
	sections := []string{rel[0].PDFSection, rel[1].PDFSection}
	assert.Contains(t, sections, "12.2",
		"§12.2 persistence must be a Reliability direction")
	assert.Contains(t, sections, "12.4",
		"§12.4 horizon scaling must be a Reliability direction")
}

func TestFEAT027_BDD_LongTermHorizonHasThreeDirections(t *testing.T) {
	// Scenario: three §12 directions require long-term resolution
	// Given the registry
	r := NewFutureDirectionRegistry()

	// When I query by long-term horizon
	long := r.ByHorizon(FutureDirectionHorizonLongTerm)

	// Then exactly three are returned
	assert.Len(t, long, 3,
		"§12.4, §12.5, and §12.6 are long-term directions")

	// And they include governance (§12.5)
	found := false
	for _, p := range long {
		if p.Slug == "governance_oversight_at_scale" {
			found = true
		}
	}
	assert.True(t, found, "governance_oversight_at_scale must be a long-term direction")
}
