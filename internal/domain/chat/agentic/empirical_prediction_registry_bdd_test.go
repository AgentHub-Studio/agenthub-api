package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD scenarios for FEAT-043 — EmpiricalPredictionRegistry (§11.4 Empirical Predictions).
//
// arXiv:2604.14228v1 §11.4 states three architectural predictions whose resolution is
// explicitly left open as "directly measurable empirical questions" that source-level
// analysis cannot answer.  These scenarios exercise the registry semantics defined by
// that section.

// Scenario 1: A system architect queries the registry to enumerate all predictions
// and confirms the paper's claim that three testable predictions arise from §11.4.
func TestFEAT043_BDD_ThreeArchitecturalPredictions(t *testing.T) {
	// Given: the EmpiricalPredictionRegistry is initialized.
	r := NewEmpiricalPredictionRegistry()

	// When: all predictions are enumerated.
	all := r.AllPredictions()

	// Then: exactly three predictions are returned (§11.4 enumerates three).
	assert.Len(t, all, 3)

	// And: each prediction covers a distinct output dimension.
	seen := map[PredictionCategory]bool{}
	for _, p := range all {
		assert.False(t, seen[p.Category], "category %q appeared twice", p.Category)
		seen[p.Category] = true
	}
	assert.True(t, seen[PredictionCategoryCodeQuality])
	assert.True(t, seen[PredictionCategoryCoordination])
	assert.True(t, seen[PredictionCategorySystemCoherence])
}

// Scenario 2: A researcher looks up the PatternDuplication prediction and verifies
// that He et al. (2025) is cited as an early empirical signal.
func TestFEAT043_BDD_PatternDuplicationHasHeEtAlSignal(t *testing.T) {
	// Given: the registry is initialized.
	r := NewEmpiricalPredictionRegistry()

	// When: the PatternDuplication profile is retrieved.
	p, ok := r.FindByID(PredictionPatternDuplication)

	// Then: the profile is found.
	require.True(t, ok)

	// And: He et al. (2025) appears among the early signals.
	foundHe := false
	for _, s := range p.EarlySignals {
		if s.Citation == "He et al. (2025)" {
			foundHe = true
			assert.Contains(t, s.Finding, "40.7%",
				"He et al. finding must include the 40.7%% complexity increase figure")
		}
	}
	assert.True(t, foundHe, "He et al. (2025) must be listed as an early signal for PatternDuplication")
}

// Scenario 3: An evaluator checks whether any prediction claims its architectural
// mitigation is sufficient — and confirms none does, because §11.4 explicitly
// marks all three as "directly measurable empirical questions" whose answers
// source-level analysis cannot resolve.
func TestFEAT043_BDD_NoPredictionClaimsSufficientMitigation(t *testing.T) {
	// Given: the registry is initialized.
	r := NewEmpiricalPredictionRegistry()

	// When: open questions are enumerated.
	openQ := r.OpenQuestions()

	// Then: all three predictions are open (no mitigation is claimed sufficient).
	assert.Len(t, openQ, 3)

	// And: every directly-measurable prediction is also an open question.
	directly := r.DirectlyMeasurable()
	assert.Len(t, directly, 3)
}

// Scenario 4: A safety reviewer verifies that no early signal in the registry is
// mislabelled as direct Claude Code evidence — §11.4 explicitly states the signals
// come from "adjacent tools" (Cursor, broad AI commit audits).
func TestFEAT043_BDD_AllSignalsAreAdjacentToolEvidence(t *testing.T) {
	// Given: the registry is initialized.
	r := NewEmpiricalPredictionRegistry()

	// When: all early signals are inspected.
	for _, p := range r.AllPredictions() {
		for _, s := range p.EarlySignals {
			// Then: no signal is marked as direct Claude Code evidence.
			assert.False(t, s.IsDirectClaudeCodeEvidence,
				"signal %q in prediction %q should be adjacent-tool evidence only", s.Citation, p.ID)

			// And: every signal has a non-empty DataScope describing its corpus.
			assert.NotEmpty(t, s.DataScope,
				"signal %q in prediction %q must document its data scope", s.Citation, p.ID)
		}
	}
}

// Scenario 5: A developer queries the SubagentRedundancy prediction and confirms
// the architectural root is §8 subagent isolation and the sole mitigation is
// subagent summary isolation.
func TestFEAT043_BDD_SubagentRedundancyRootAndMitigation(t *testing.T) {
	// Given: the registry is initialized.
	r := NewEmpiricalPredictionRegistry()

	// When: the SubagentRedundancy profile is retrieved.
	p, ok := r.FindByID(PredictionSubagentRedundancy)

	// Then: the profile is found.
	require.True(t, ok)

	// And: the architectural root references §8 subagent isolation.
	assert.Contains(t, p.ArchitecturalRoot, "§8",
		"SubagentRedundancy root must reference §8")
	assert.Contains(t, p.ArchitecturalRoot, "isolation",
		"SubagentRedundancy root must mention isolation")

	// And: exactly one mitigation is listed (subagent_summary_isolation).
	require.Len(t, p.Mitigations, 1)
	assert.Equal(t, "subagent_summary_isolation", p.Mitigations[0].Slug)
	assert.Equal(t, "8", p.Mitigations[0].PDFSection)
}

// Scenario 6: An architect verifies that the LocalGoodGlobalBad prediction
// correctly encodes the §11.1 design-philosophy root and names Liu et al. (2026)
// as one of its early signals.
func TestFEAT043_BDD_LocalGoodGlobalBadRootAndLiuSignal(t *testing.T) {
	// Given: the registry is initialized.
	r := NewEmpiricalPredictionRegistry()

	// When: the LocalGoodGlobalBad profile is retrieved.
	p, ok := r.FindByID(PredictionLocalGoodGlobalBad)

	// Then: the profile is found.
	require.True(t, ok)

	// And: the architectural root references §11.1 design philosophy.
	assert.Contains(t, p.ArchitecturalRoot, "§11.1",
		"LocalGoodGlobalBad root must reference §11.1")

	// And: Liu et al. (2026) appears among the early signals with the 304,000-commit figure.
	foundLiu := false
	for _, s := range p.EarlySignals {
		if s.Citation == "Liu et al. (2026)" {
			foundLiu = true
			assert.Contains(t, s.Finding, "304,000",
				"Liu et al. finding must include the 304,000-commit audit figure")
		}
	}
	assert.True(t, foundLiu, "Liu et al. (2026) must be listed as an early signal for LocalGoodGlobalBad")
}
