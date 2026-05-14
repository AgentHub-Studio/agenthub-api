package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Unit Tests (FEAT028) ─────────────────────────────────────────────────────

// TestFEAT028_SeedCount verifies the canonical number of §11.7 commitments.
func TestFEAT028_SeedCount(t *testing.T) {
	assert.Equal(t, 3, SeedRecurringDesignCommitmentCount,
		"§11.7 documents exactly 3 recurring design commitments")
}

// TestFEAT028_SeedSlugs verifies all three slug constants are in the seed list.
func TestFEAT028_SeedSlugs(t *testing.T) {
	require.Len(t, SeedRecurringDesignCommitmentSlugs, 3)
	assert.Equal(t, CommitGraduatedLayering, SeedRecurringDesignCommitmentSlugs[0])
	assert.Equal(t, CommitAppendOnlyAuditability, SeedRecurringDesignCommitmentSlugs[1])
	assert.Equal(t, CommitModelJudgmentInHarness, SeedRecurringDesignCommitmentSlugs[2])
}

// TestFEAT028_RegistrySize verifies the registry holds exactly three profiles.
func TestFEAT028_RegistrySize(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	all := r.AllCommitments()
	assert.Len(t, all, SeedRecurringDesignCommitmentCount,
		"AllCommitments must return exactly SeedRecurringDesignCommitmentCount profiles")
}

// TestFEAT028_CanonicalOrder verifies profiles are returned in §11.7 prose order.
func TestFEAT028_CanonicalOrder(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	all := r.AllCommitments()
	require.Len(t, all, 3)
	assert.Equal(t, CommitGraduatedLayering, all[0].Slug)
	assert.Equal(t, CommitAppendOnlyAuditability, all[1].Slug)
	assert.Equal(t, CommitModelJudgmentInHarness, all[2].Slug)
}

// TestFEAT028_FindBySlug_HappyPath verifies lookup returns the correct profile.
func TestFEAT028_FindBySlug_HappyPath(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	for _, slug := range SeedRecurringDesignCommitmentSlugs {
		p, ok := r.FindRecurringDesignCommitmentBySlug(slug)
		require.Truef(t, ok, "slug %q must be found", slug)
		require.NotNil(t, p)
		assert.Equal(t, slug, p.Slug)
	}
}

// TestFEAT028_FindBySlug_NotFound verifies unknown slugs return (nil, false).
func TestFEAT028_FindBySlug_NotFound(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	p, ok := r.FindRecurringDesignCommitmentBySlug("nonexistent_slug")
	assert.False(t, ok)
	assert.Nil(t, p)
}

// TestFEAT028_AllProfilesPDFSection verifies every profile cites §11.7.
func TestFEAT028_AllProfilesPDFSection(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	for _, p := range r.AllCommitments() {
		assert.Equalf(t, "11.7", p.PDFSection,
			"profile %q must cite PDFSection 11.7", p.Slug)
	}
}

// TestFEAT028_CommitmentTypesDistinct verifies each profile has a unique CommitmentType.
func TestFEAT028_CommitmentTypesDistinct(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	seen := map[CommitmentType]bool{}
	for _, p := range r.AllCommitments() {
		assert.Falsef(t, seen[p.CommitmentType],
			"CommitmentType %q must appear exactly once", p.CommitmentType)
		seen[p.CommitmentType] = true
	}
}

// TestFEAT028_CommitmentTypeValues verifies the three expected CommitmentType values.
func TestFEAT028_CommitmentTypeValues(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	p1, _ := r.FindRecurringDesignCommitmentBySlug(CommitGraduatedLayering)
	p2, _ := r.FindRecurringDesignCommitmentBySlug(CommitAppendOnlyAuditability)
	p3, _ := r.FindRecurringDesignCommitmentBySlug(CommitModelJudgmentInHarness)

	assert.Equal(t, CommitmentTypeLayering, p1.CommitmentType)
	assert.Equal(t, CommitmentTypeAuditability, p2.CommitmentType)
	assert.Equal(t, CommitmentTypeJudgmentDelegation, p3.CommitmentType)
}

// TestFEAT028_ByCommitmentType_Layering returns exactly one profile.
func TestFEAT028_ByCommitmentType_Layering(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	results := r.ByCommitmentType(CommitmentTypeLayering)
	require.Len(t, results, 1)
	assert.Equal(t, CommitGraduatedLayering, results[0].Slug)
}

// TestFEAT028_ByCommitmentType_Unknown returns empty slice for unknown type.
func TestFEAT028_ByCommitmentType_Unknown(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	results := r.ByCommitmentType("unknown_type")
	assert.Empty(t, results)
}

// TestFEAT028_AllProfilesAreStructural verifies all three commitments are structural.
func TestFEAT028_AllProfilesAreStructural(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	for _, p := range r.AllCommitments() {
		assert.Truef(t, p.IsStructural,
			"profile %q must be structural per §11.7 — baked into architecture", p.Slug)
	}
}

// TestFEAT028_StructuralCommitments returns all three profiles.
func TestFEAT028_StructuralCommitments(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	structural := r.StructuralCommitments()
	assert.Len(t, structural, 3,
		"all three §11.7 commitments are structural (IsStructural=true)")
}

// TestFEAT028_ManifestationCount verifies total manifestation entries.
func TestFEAT028_ManifestationCount(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	// Each of the three profiles has 4 manifestation entries.
	assert.Equal(t, 12, r.ManifestationCount(),
		"3 commitments × 4 manifestations each = 12 total")
}

// TestFEAT028_EachProfileHasManifestations verifies no profile has an empty
// Manifestations slice.
func TestFEAT028_EachProfileHasManifestations(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	for _, p := range r.AllCommitments() {
		assert.NotEmptyf(t, p.Manifestations,
			"profile %q must have at least one manifestation", p.Slug)
	}
}

// TestFEAT028_EachProfileHasTradeoffAccepted verifies no profile has an empty
// TradeoffAccepted field.
func TestFEAT028_EachProfileHasTradeoffAccepted(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	for _, p := range r.AllCommitments() {
		assert.NotEmptyf(t, p.TradeoffAccepted,
			"profile %q must state what is sacrificed (TradeoffAccepted)", p.Slug)
	}
}

// TestFEAT028_EachProfileHasDesignValues verifies each profile has at least
// two design values.
func TestFEAT028_EachProfileHasDesignValues(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	for _, p := range r.AllCommitments() {
		assert.GreaterOrEqualf(t, len(p.DesignValues), 2,
			"profile %q must serve at least two design values", p.Slug)
	}
}

// TestFEAT028_ServingDesignValue_Safety verifies graduated_layering serves safety.
func TestFEAT028_ServingDesignValue_Safety(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	results := r.ServingDesignValue(DesignValueSafety)
	var slugs []RecurringDesignCommitmentSlug
	for _, p := range results {
		slugs = append(slugs, p.Slug)
	}
	assert.Contains(t, slugs, CommitGraduatedLayering,
		"graduated_layering serves safety via defense-in-depth layering")
}

// TestFEAT028_ServingDesignValue_HumanAuthority verifies append_only_auditability
// serves human_authority.
func TestFEAT028_ServingDesignValue_HumanAuthority(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	results := r.ServingDesignValue(DesignValueHumanAuthority)
	var slugs []RecurringDesignCommitmentSlug
	for _, p := range results {
		slugs = append(slugs, p.Slug)
	}
	assert.Contains(t, slugs, CommitAppendOnlyAuditability,
		"append_only_auditability serves human_authority via inspectable audit trail")
}

// TestFEAT028_ServingDesignValue_Unknown returns empty for unregistered value.
func TestFEAT028_ServingDesignValue_Unknown(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	results := r.ServingDesignValue("nonexistent_value")
	assert.Empty(t, results)
}

// TestFEAT028_IsValidSlug_KnownSlugs verifies all seed slugs are valid.
func TestFEAT028_IsValidSlug_KnownSlugs(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	for _, slug := range SeedRecurringDesignCommitmentSlugs {
		assert.Truef(t, r.IsValidSlug(slug), "slug %q must be valid", slug)
	}
}

// TestFEAT028_IsValidSlug_UnknownSlug verifies unknown slugs are invalid.
func TestFEAT028_IsValidSlug_UnknownSlug(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	assert.False(t, r.IsValidSlug("not_a_real_slug"))
}

// TestFEAT028_ModelJudgmentHarnessRatio verifies the 1.6%/98.4% ratio is
// captured in the model_judgment_in_harness manifestations.
func TestFEAT028_ModelJudgmentHarnessRatio(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	p, ok := r.FindRecurringDesignCommitmentBySlug(CommitModelJudgmentInHarness)
	require.True(t, ok)
	found := false
	for _, m := range p.Manifestations {
		if strings.Contains(m, "1.6%") && strings.Contains(m, "98.4%") {
			found = true
			break
		}
	}
	assert.True(t, found,
		"model_judgment_in_harness manifestations must cite the 1.6%/98.4% harness ratio from §11.1")
}

// TestFEAT028_AppendOnlySessionManifest verifies the append-only JSONL
// transcript evidence appears in append_only_auditability.
func TestFEAT028_AppendOnlySessionManifest(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	p, ok := r.FindRecurringDesignCommitmentBySlug(CommitAppendOnlyAuditability)
	require.True(t, ok)
	combined := strings.Join(p.Manifestations, " ")
	assert.Contains(t, combined, "JSONL",
		"append_only_auditability must cite append-only JSONL session transcripts")
}

// TestFEAT028_AllCommitmentsReturnCopy verifies AllCommitments returns a copy
// so mutations do not affect the registry state.
func TestFEAT028_AllCommitmentsReturnCopy(t *testing.T) {
	r := NewRecurringDesignCommitmentRegistry()
	first := r.AllCommitments()
	first[0].Label = "mutated"
	second := r.AllCommitments()
	assert.NotEqual(t, "mutated", second[0].Label,
		"AllCommitments must return a copy; mutations must not affect registry state")
}

// ─── BDD Tests (FEAT028) ─────────────────────────────────────────────────────

// TestFEAT028_BDD_RecurringDesignCommitment groups BDD-style scenario tests
// that exercise the registry as described in §11.7 of the paper.
func TestFEAT028_BDD_RecurringDesignCommitment(t *testing.T) {
	// Scenario A: paper documents exactly three cross-cutting commitments.
	t.Run("exactly_three_commitments_per_11_7", func(t *testing.T) {
		// GIVEN the §11.7 registry is instantiated
		r := NewRecurringDesignCommitmentRegistry()
		// WHEN all commitments are retrieved
		all := r.AllCommitments()
		// THEN there are exactly three, matching the paper's three paragraphs
		assert.Len(t, all, 3)
		assert.Equal(t, SeedRecurringDesignCommitmentCount, len(all))
	})

	// Scenario B: graduated_layering commitment has four distinct subsystem
	// manifestations (permissions, context management, extensibility, subagents).
	t.Run("graduated_layering_manifests_across_four_subsystems", func(t *testing.T) {
		// GIVEN the registry
		r := NewRecurringDesignCommitmentRegistry()
		// WHEN the graduated_layering profile is looked up
		p, ok := r.FindRecurringDesignCommitmentBySlug(CommitGraduatedLayering)
		// THEN it exists and has four named subsystems
		require.True(t, ok)
		require.Len(t, p.Manifestations, 4)
		combined := strings.Join(p.Manifestations, " ")
		assert.Contains(t, combined, "permissions")
		assert.Contains(t, combined, "context_management")
		assert.Contains(t, combined, "extensibility")
		assert.Contains(t, combined, "subagents")
	})

	// Scenario C: append_only_auditability states what is sacrificed
	// (richer structured queries require post-hoc reconstruction).
	t.Run("append_only_auditability_tradeoff_accepted", func(t *testing.T) {
		// GIVEN the registry
		r := NewRecurringDesignCommitmentRegistry()
		// WHEN the append_only_auditability profile is retrieved
		p, ok := r.FindRecurringDesignCommitmentBySlug(CommitAppendOnlyAuditability)
		// THEN the TradeoffAccepted field mentions post-hoc reconstruction
		require.True(t, ok)
		assert.Contains(t, p.TradeoffAccepted, "post-hoc reconstruction",
			"the cost of append-only is that rich queries require reconstruction")
	})

	// Scenario D: all three commitments are structural (baked into architecture).
	t.Run("all_commitments_are_structural", func(t *testing.T) {
		// GIVEN the registry
		r := NewRecurringDesignCommitmentRegistry()
		// WHEN structural commitments are filtered
		structural := r.StructuralCommitments()
		// THEN all three commitments are structural
		assert.Len(t, structural, 3,
			"§11.7 commitments are baked into the architecture, not mere policy choices")
	})

	// Scenario E: filtering by CommitmentType returns exactly one profile per type.
	t.Run("commitment_type_filter_returns_one_per_type", func(t *testing.T) {
		// GIVEN the registry
		r := NewRecurringDesignCommitmentRegistry()
		// WHEN each CommitmentType is queried
		types := []CommitmentType{
			CommitmentTypeLayering,
			CommitmentTypeAuditability,
			CommitmentTypeJudgmentDelegation,
		}
		for _, ct := range types {
			results := r.ByCommitmentType(ct)
			// THEN exactly one profile matches each type
			assert.Lenf(t, results, 1,
				"CommitmentType %q must match exactly one profile", ct)
		}
	})

	// Scenario F: model_judgment_in_harness serves capability and reliability.
	t.Run("model_judgment_serves_capability_and_reliability", func(t *testing.T) {
		// GIVEN the registry
		r := NewRecurringDesignCommitmentRegistry()
		// WHEN commitment is retrieved
		p, ok := r.FindRecurringDesignCommitmentBySlug(CommitModelJudgmentInHarness)
		require.True(t, ok)
		// THEN DesignValues includes both capability and reliability
		assert.Contains(t, p.DesignValues, string(DesignValueCapability))
		assert.Contains(t, p.DesignValues, string(DesignValueReliability))
	})

	// Scenario G: ServingDesignValue cross-references back to correct profiles.
	t.Run("serving_design_value_cross_reference", func(t *testing.T) {
		// GIVEN the registry
		r := NewRecurringDesignCommitmentRegistry()
		// WHEN commitments serving reliability are retrieved
		reliabilityProfiles := r.ServingDesignValue(DesignValueReliability)
		var slugs []RecurringDesignCommitmentSlug
		for _, p := range reliabilityProfiles {
			slugs = append(slugs, p.Slug)
		}
		// THEN both append_only_auditability and model_judgment_in_harness appear
		// (graduated_layering serves safety+reliability)
		assert.Contains(t, slugs, CommitAppendOnlyAuditability)
		assert.Contains(t, slugs, CommitModelJudgmentInHarness)
	})

	// Scenario H: IsValidSlug guards against typos at call sites.
	t.Run("is_valid_slug_guards_call_sites", func(t *testing.T) {
		// GIVEN the registry
		r := NewRecurringDesignCommitmentRegistry()
		// WHEN valid and invalid slugs are tested
		validCases := []RecurringDesignCommitmentSlug{
			CommitGraduatedLayering,
			CommitAppendOnlyAuditability,
			CommitModelJudgmentInHarness,
		}
		for _, slug := range validCases {
			assert.Truef(t, r.IsValidSlug(slug), "slug %q must be valid", slug)
		}
		assert.False(t, r.IsValidSlug("graduated_something_else"),
			"near-miss slugs must not be accepted")
	})
}
