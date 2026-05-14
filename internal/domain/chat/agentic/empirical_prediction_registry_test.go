package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFEAT043_EmpiricalPredictionCount verifies the registry contains exactly three
// predictions, matching the three architectural predictions stated in §11.4.
func TestFEAT043_EmpiricalPredictionCount(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	assert.Equal(t, SeedEmpiricalPredictionCount, r.Count())
	assert.Equal(t, 3, SeedEmpiricalPredictionCount)
}

// TestFEAT043_AllPredictionsReturnsAll verifies AllPredictions returns all three
// profiles without omission.
func TestFEAT043_AllPredictionsReturnsAll(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	all := r.AllPredictions()
	assert.Len(t, all, 3)
}

// TestFEAT043_SeedIDsMatchCount verifies the seed ID slice length matches the count
// constant.
func TestFEAT043_SeedIDsMatchCount(t *testing.T) {
	assert.Len(t, SeedEmpiricalPredictionIDs, SeedEmpiricalPredictionCount)
}

// TestFEAT043_AllSeedIDsAreValid verifies every SeedEmpiricalPredictionID is
// findable in the registry.
func TestFEAT043_AllSeedIDsAreValid(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	for _, id := range SeedEmpiricalPredictionIDs {
		_, ok := r.FindByID(id)
		assert.True(t, ok, "seed ID %q not found in registry", id)
	}
}

// TestFEAT043_FindByIDSuccess verifies each of the three concrete constants resolves
// to a non-nil profile.
func TestFEAT043_FindByIDSuccess(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	ids := []PredictionID{
		PredictionPatternDuplication,
		PredictionSubagentRedundancy,
		PredictionLocalGoodGlobalBad,
	}
	for _, id := range ids {
		p, ok := r.FindByID(id)
		require.True(t, ok, "expected to find prediction %q", id)
		assert.NotNil(t, p)
		assert.Equal(t, id, p.ID)
	}
}

// TestFEAT043_FindByIDMissReturnsNil verifies FindByID returns (nil, false) for
// unknown identifiers.
func TestFEAT043_FindByIDMissReturnsNil(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	p, ok := r.FindByID("nonexistent_prediction")
	assert.False(t, ok)
	assert.Nil(t, p)
}

// TestFEAT043_IsValidPredictionID verifies the boolean helper matches FindByID.
func TestFEAT043_IsValidPredictionID(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	assert.True(t, r.IsValidPredictionID(PredictionPatternDuplication))
	assert.True(t, r.IsValidPredictionID(PredictionSubagentRedundancy))
	assert.True(t, r.IsValidPredictionID(PredictionLocalGoodGlobalBad))
	assert.False(t, r.IsValidPredictionID("made_up_id"))
}

// TestFEAT043_AllPredictionsHavePDFSection verifies every profile has a non-empty
// PDFSection value.
func TestFEAT043_AllPredictionsHavePDFSection(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	for _, p := range r.AllPredictions() {
		assert.NotEmpty(t, p.PDFSection, "prediction %q missing PDFSection", p.ID)
		assert.Equal(t, "11.4", p.PDFSection)
	}
}

// TestFEAT043_AllPredictionsHaveTitle verifies every profile has a non-empty Title.
func TestFEAT043_AllPredictionsHaveTitle(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	for _, p := range r.AllPredictions() {
		assert.NotEmpty(t, p.Title, "prediction %q has empty Title", p.ID)
	}
}

// TestFEAT043_AllPredictionsHavePrediction verifies every profile has a non-empty
// Prediction statement.
func TestFEAT043_AllPredictionsHavePrediction(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	for _, p := range r.AllPredictions() {
		assert.NotEmpty(t, p.Prediction, "prediction %q has empty Prediction field", p.ID)
	}
}

// TestFEAT043_AllPredictionsHaveCausalChain verifies every profile has a non-empty
// CausalChain.
func TestFEAT043_AllPredictionsHaveCausalChain(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	for _, p := range r.AllPredictions() {
		assert.NotEmpty(t, p.CausalChain, "prediction %q has empty CausalChain", p.ID)
	}
}

// TestFEAT043_AllPredictionsHaveArchitecturalRoot verifies every profile names its
// architectural root.
func TestFEAT043_AllPredictionsHaveArchitecturalRoot(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	for _, p := range r.AllPredictions() {
		assert.NotEmpty(t, p.ArchitecturalRoot, "prediction %q missing ArchitecturalRoot", p.ID)
	}
}

// TestFEAT043_CategoryCoverage verifies the three predictions span all three
// PredictionCategory values without overlap.
func TestFEAT043_CategoryCoverage(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	categories := map[PredictionCategory]int{}
	for _, p := range r.AllPredictions() {
		categories[p.Category]++
	}
	assert.Equal(t, 1, categories[PredictionCategoryCodeQuality], "expected exactly one code_quality prediction")
	assert.Equal(t, 1, categories[PredictionCategoryCoordination], "expected exactly one coordination prediction")
	assert.Equal(t, 1, categories[PredictionCategorySystemCoherence], "expected exactly one system_coherence prediction")
}

// TestFEAT043_ByCategoryCodeQuality verifies the code_quality category returns
// exactly the PatternDuplication prediction.
func TestFEAT043_ByCategoryCodeQuality(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	results := r.ByCategory(PredictionCategoryCodeQuality)
	require.Len(t, results, 1)
	assert.Equal(t, PredictionPatternDuplication, results[0].ID)
}

// TestFEAT043_ByCategoryCoordination verifies the coordination category returns
// exactly the SubagentRedundancy prediction.
func TestFEAT043_ByCategoryCoordination(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	results := r.ByCategory(PredictionCategoryCoordination)
	require.Len(t, results, 1)
	assert.Equal(t, PredictionSubagentRedundancy, results[0].ID)
}

// TestFEAT043_ByCategorySystemCoherence verifies the system_coherence category
// returns exactly the LocalGoodGlobalBad prediction.
func TestFEAT043_ByCategorySystemCoherence(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	results := r.ByCategory(PredictionCategorySystemCoherence)
	require.Len(t, results, 1)
	assert.Equal(t, PredictionLocalGoodGlobalBad, results[0].ID)
}

// TestFEAT043_ByCategoryUnknownReturnsEmpty verifies ByCategory returns nil/empty
// for an unregistered category value.
func TestFEAT043_ByCategoryUnknownReturnsEmpty(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	results := r.ByCategory("unknown_category")
	assert.Empty(t, results)
}

// TestFEAT043_AllDirectlyMeasurable verifies that all three §11.4 predictions are
// marked as directly measurable empirical questions.
func TestFEAT043_AllDirectlyMeasurable(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	directlyMeasurable := r.DirectlyMeasurable()
	assert.Len(t, directlyMeasurable, 3, "all three predictions must be IsDirectlyMeasurable=true")
}

// TestFEAT043_AllMitigationsNotClaimedSufficient verifies §11.4's explicit stance
// that no mitigation is claimed to be sufficient.
func TestFEAT043_AllMitigationsNotClaimedSufficient(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	for _, p := range r.AllPredictions() {
		assert.False(t, p.MitigationClaimedSufficient,
			"prediction %q must not claim mitigation is sufficient (paper leaves it open)", p.ID)
	}
	openQ := r.OpenQuestions()
	assert.Len(t, openQ, 3, "all three predictions must be open questions")
}

// TestFEAT043_AllHaveAtLeastOneEarlySignal verifies every prediction has at least
// one published empirical signal cited in §11.4.
func TestFEAT043_AllHaveAtLeastOneEarlySignal(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	for _, p := range r.AllPredictions() {
		assert.NotEmpty(t, p.EarlySignals,
			"prediction %q must have at least one early signal", p.ID)
	}
}

// TestFEAT043_AllHaveAtLeastOneMitigation verifies every prediction names at least
// one architectural mitigation.
func TestFEAT043_AllHaveAtLeastOneMitigation(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	for _, p := range r.AllPredictions() {
		assert.NotEmpty(t, p.Mitigations,
			"prediction %q must have at least one mitigation", p.ID)
	}
}

// TestFEAT043_NoEarlySignalIsDirectClaudeCodeEvidence verifies that §11.4's
// disclaimer ("adjacent tools, not Claude Code specifically") is encoded in the data:
// no signal must have IsDirectClaudeCodeEvidence=true.
func TestFEAT043_NoEarlySignalIsDirectClaudeCodeEvidence(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	for _, p := range r.AllPredictions() {
		for _, s := range p.EarlySignals {
			assert.False(t, s.IsDirectClaudeCodeEvidence,
				"signal %q in prediction %q must not be direct Claude Code evidence (§11.4 disclaimer)",
				s.Citation, p.ID)
		}
	}
}

// TestFEAT043_WithEarlySignalsReturnsAllThree verifies WithEarlySignals returns all
// three profiles (each has at least one signal).
func TestFEAT043_WithEarlySignalsReturnsAllThree(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	results := r.WithEarlySignals()
	assert.Len(t, results, 3)
}

// TestFEAT043_TotalEarlySignalCount verifies the aggregate signal count is at least
// five (two signals for PatternDuplication, one for SubagentRedundancy, two for
// LocalGoodGlobalBad).
func TestFEAT043_TotalEarlySignalCount(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	total := r.TotalEarlySignalCount()
	assert.GreaterOrEqual(t, total, 5, "expected at least 5 total early signals")
}

// TestFEAT043_TotalMitigationCount verifies the aggregate mitigation count is at
// least four (three for PatternDuplication, one for SubagentRedundancy, two for
// LocalGoodGlobalBad).
func TestFEAT043_TotalMitigationCount(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	total := r.TotalMitigationCount()
	assert.GreaterOrEqual(t, total, 4, "expected at least 4 total mitigations")
}

// TestFEAT043_PatternDuplicationProfile_Invariants verifies key invariants for the
// PatternDuplication prediction profile.
func TestFEAT043_PatternDuplicationProfile_Invariants(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	p, ok := r.FindByID(PredictionPatternDuplication)
	require.True(t, ok)
	assert.Equal(t, PredictionCategoryCodeQuality, p.Category)
	assert.True(t, p.IsDirectlyMeasurable)
	assert.False(t, p.MitigationClaimedSufficient)
	// §11.4 cites He et al. and Liu et al. for this prediction
	assert.Len(t, p.EarlySignals, 2)
	// Three mitigations from §7: graduated_compression, cache_aware, read_time_projection
	assert.Len(t, p.Mitigations, 3)
}

// TestFEAT043_SubagentRedundancyProfile_Invariants verifies key invariants for the
// SubagentRedundancy prediction profile.
func TestFEAT043_SubagentRedundancyProfile_Invariants(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	p, ok := r.FindByID(PredictionSubagentRedundancy)
	require.True(t, ok)
	assert.Equal(t, PredictionCategoryCoordination, p.Category)
	assert.True(t, p.IsDirectlyMeasurable)
	assert.False(t, p.MitigationClaimedSufficient)
	// One mitigation from §8: subagent summary isolation
	assert.Len(t, p.Mitigations, 1)
	assert.Equal(t, "subagent_summary_isolation", p.Mitigations[0].Slug)
}

// TestFEAT043_LocalGoodGlobalBadProfile_Invariants verifies key invariants for the
// LocalGoodGlobalBad prediction profile.
func TestFEAT043_LocalGoodGlobalBadProfile_Invariants(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	p, ok := r.FindByID(PredictionLocalGoodGlobalBad)
	require.True(t, ok)
	assert.Equal(t, PredictionCategorySystemCoherence, p.Category)
	assert.True(t, p.IsDirectlyMeasurable)
	assert.False(t, p.MitigationClaimedSufficient)
	// §11.4 cites He et al. and Liu et al. for this prediction
	assert.Len(t, p.EarlySignals, 2)
}

// TestFEAT043_MitigationSlugsAreNonEmpty verifies every mitigation has a non-empty
// Slug and Description.
func TestFEAT043_MitigationSlugsAreNonEmpty(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	for _, p := range r.AllPredictions() {
		for _, m := range p.Mitigations {
			assert.NotEmpty(t, m.Slug, "prediction %q has mitigation with empty Slug", p.ID)
			assert.NotEmpty(t, m.Description, "mitigation %q in prediction %q has empty Description", m.Slug, p.ID)
			assert.NotEmpty(t, m.PDFSection, "mitigation %q in prediction %q has empty PDFSection", m.Slug, p.ID)
		}
	}
}

// TestFEAT043_EarlySignalFieldsAreNonEmpty verifies each EarlySignal has Citation,
// Finding, and DataScope set.
func TestFEAT043_EarlySignalFieldsAreNonEmpty(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	for _, p := range r.AllPredictions() {
		for _, s := range p.EarlySignals {
			assert.NotEmpty(t, s.Citation, "prediction %q has signal with empty Citation", p.ID)
			assert.NotEmpty(t, s.Finding, "prediction %q has signal with empty Finding", p.ID)
			assert.NotEmpty(t, s.DataScope, "prediction %q has signal with empty DataScope", p.ID)
		}
	}
}

// TestFEAT043_NewRegistryIsIndependent verifies that two registry instances are
// independent: mutating one does not affect the other.
func TestFEAT043_NewRegistryIsIndependent(t *testing.T) {
	r1 := NewEmpiricalPredictionRegistry()
	r2 := NewEmpiricalPredictionRegistry()
	assert.Equal(t, r1.Count(), r2.Count())
	// AllPredictions returns a copy; modifying it must not affect registry state.
	all := r1.AllPredictions()
	all[0].Title = "mutated"
	p, _ := r1.FindByID(PredictionPatternDuplication)
	assert.NotEqual(t, "mutated", p.Title, "registry internal state must be immutable from returned copies")
}

// TestFEAT043_TableDriven_FindByID exercises FindByID against all known and unknown
// IDs in a table-driven sub-test set.
func TestFEAT043_TableDriven_FindByID(t *testing.T) {
	r := NewEmpiricalPredictionRegistry()
	cases := []struct {
		id      PredictionID
		wantHit bool
	}{
		{PredictionPatternDuplication, true},
		{PredictionSubagentRedundancy, true},
		{PredictionLocalGoodGlobalBad, true},
		{"does_not_exist", false},
		{"", false},
		{"pattern_duplication_extra", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.id), func(t *testing.T) {
			p, ok := r.FindByID(tc.id)
			assert.Equal(t, tc.wantHit, ok)
			if tc.wantHit {
				assert.NotNil(t, p)
			} else {
				assert.Nil(t, p)
			}
		})
	}
}
