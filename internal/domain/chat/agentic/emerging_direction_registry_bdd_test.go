package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emerging_direction_registry_bdd_test.go — BDD scenarios for §11.6 EmergingDirectionRegistry.
//
// Each test function encodes one Given/When/Then scenario derived from §11.6 of
// arXiv:2604.14228v1.

// Scenario 1: §11.6 names exactly five distinct emerging directions.
//
// Given a fresh registry seeded from §11.6
// When the caller counts all registered directions
// Then the count is five and every seed ID resolves to a distinct profile.
func TestBDD_EmergingDirectionRegistry_FiveDistinctDirections(t *testing.T) {
	// Given
	r := NewEmergingDirectionRegistry()

	// When
	all := r.All()

	// Then
	assert.Equal(t, 5, len(all), "§11.6 names exactly five emerging directions")

	seen := make(map[EmergingDirectionID]bool)
	for _, p := range all {
		assert.False(t, seen[p.ID], "direction ID %q must appear exactly once", p.ID)
		seen[p.ID] = true
	}
}

// Scenario 2: Every §11.6 direction is grounded in an open design question.
//
// Given the five §11.6 direction profiles
// When the caller reads the OpenQuestion field of each
// Then no OpenQuestion is empty — §11.6 frames each direction as an unresolved question.
func TestBDD_EmergingDirectionRegistry_EachDirectionHasOpenQuestion(t *testing.T) {
	// Given
	r := NewEmergingDirectionRegistry()

	// When / Then
	for _, p := range r.All() {
		assert.NotEmpty(t, p.OpenQuestion,
			"§11.6 frames %q as an open design question; OpenQuestion must not be empty", p.ID)
	}
}

// Scenario 3: The Governance direction captures the 13.3% safety-card adoption finding.
//
// Given the Governance direction profile
// When the caller inspects the SupportingEvidence
// Then the MIT AI Agent Index citation with the 13.3% statistic is present and
//
//	marked as an empirical study.
func TestBDD_EmergingDirectionRegistry_Governance_MITAgentIndexEvidencePresent(t *testing.T) {
	// Given
	r := NewEmergingDirectionRegistry()

	// When
	p, ok := r.FindByID(DirectionGovernance)
	require.True(t, ok)

	// Then
	found := false
	for _, e := range p.SupportingEvidence {
		if e.Citation == "Staufer et al. (2026)" {
			assert.True(t, e.IsEmpiricalStudy, "MIT AI Agent Index is an empirical study")
			assert.Contains(t, e.Finding, "13.3%", "finding must reference the 13.3% safety-card statistic")
			found = true
		}
	}
	assert.True(t, found, "Staufer et al. (2026) must appear in Governance supporting evidence")
}

// Scenario 4: Proactive Architectures does not require a harness change —
//
//	KAIROS is already a harness-level mechanism.
//
// Given the ProactiveArchitectures direction profile
// When the caller checks RequiresHarnessChange
// Then it is false, because §11.6 describes KAIROS as a feature-gated harness mechanism.
func TestBDD_EmergingDirectionRegistry_ProactiveArchitectures_NoHarnessChangeRequired(t *testing.T) {
	// Given
	r := NewEmergingDirectionRegistry()

	// When
	p, ok := r.FindByID(DirectionProactiveArchitectures)
	require.True(t, ok)

	// Then
	assert.False(t, p.RequiresHarnessChange,
		"KAIROS is already harness-level; ProactiveArchitectures must not require a harness change")
	assert.Equal(t, CategoryProactivity, p.Category)
	assert.Equal(t, HorizonNearTerm, p.Horizon)
}

// Scenario 5: Four directions require harness engineering beyond model improvements.
//
// Given the full registry
// When the caller calls RequiringHarnessChange
// Then exactly four directions are returned, consistent with §11.6's statement that
//
//	closing the gaps requires "additional scaffolding … rather than model improvements alone".
func TestBDD_EmergingDirectionRegistry_FourDirectionsRequireHarnessEngineering(t *testing.T) {
	// Given
	r := NewEmergingDirectionRegistry()

	// When
	harness := r.RequiringHarnessChange()

	// Then
	assert.Len(t, harness, 4, "§11.6 implies four directions need harness-layer work")
	for _, p := range harness {
		assert.NotEqual(t, DirectionProactiveArchitectures, p.ID,
			"ProactiveArchitectures already has a harness mechanism (KAIROS)")
	}
}

// Scenario 6: Memory as a First-Class Subsystem is the only mid-term direction and
//
//	identifies three open memory frontiers.
//
// Given the MemoryFirstClass direction profile
// When the caller inspects its horizon and evidence
// Then the horizon is mid-term and at least one evidence item references Hu et al. (2025).
func TestBDD_EmergingDirectionRegistry_MemoryFirstClass_MidTermWithHuEtAlEvidence(t *testing.T) {
	// Given
	r := NewEmergingDirectionRegistry()

	// When
	midTerm := r.ByHorizon(HorizonMidTerm)

	// Then — single mid-term direction
	require.Len(t, midTerm, 1, "only MemoryFirstClass is mid-term in §11.6")
	p := midTerm[0]
	assert.Equal(t, DirectionMemoryFirstClass, p.ID)

	// Evidence must include Hu et al. (2025)
	found := false
	for _, e := range p.SupportingEvidence {
		if e.Citation == "Hu et al. (2025)" {
			assert.True(t, e.IsEmpiricalStudy)
			found = true
		}
	}
	assert.True(t, found, "Hu et al. (2025) must be cited in MemoryFirstClass evidence")
}

// Scenario 7: ServingValue(DesignValueReliability) covers the three directions that
//
//	directly address reliability concerns (decoupling, memory, observability).
//
// Given the registry
// When the caller queries directions serving DesignValueReliability
// Then exactly three directions are returned and none is Governance or Proactive.
func TestBDD_EmergingDirectionRegistry_ReliabilityServedByThreeDirections(t *testing.T) {
	// Given
	r := NewEmergingDirectionRegistry()

	// When
	got := r.ServingValue(DesignValueReliability)

	// Then
	assert.Len(t, got, 3, "ArchitecturalDecoupling, MemoryFirstClass, and ObservabilityAndSilentFailure serve Reliability")
	ids := make(map[EmergingDirectionID]bool)
	for _, p := range got {
		ids[p.ID] = true
	}
	assert.False(t, ids[DirectionGovernance], "Governance does not primarily serve Reliability")
	assert.False(t, ids[DirectionProactiveArchitectures], "ProactiveArchitectures does not serve Reliability")
}
