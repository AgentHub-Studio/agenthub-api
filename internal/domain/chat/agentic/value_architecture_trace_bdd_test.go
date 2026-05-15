package agentic

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD scenarios for §2.3 ValueArchitectureTrace + §2.4 EvaluativeLens
// Source: arXiv:2604.14228v1 §2.3 "From Values to Architecture" and
//         §2.4 "An Evaluative Lens: Long-term Capability Preservation".

// Scenario 1: Each design value traces to concrete architectural decisions
// Given: a ValueArchitectureTraceRegistry populated from §2.3
// When:  a caller asks for the trace for any of the five design values
// Then:  each trace returns a non-empty list of architectural decisions
//        with labels, component references, and PDF section citations
func TestBDD_ValueArchitectureTrace_EachValueHasConcreteDecisions(t *testing.T) {
	// Given
	r := NewValueArchitectureTraceRegistry()

	for _, v := range AllDesignValues {
		t.Run(fmt.Sprintf("value=%s", v), func(t *testing.T) {
			// When
			trace, ok := r.TraceForValue(v)

			// Then
			require.True(t, ok, "value %q must have a trace", v)
			assert.NotEmpty(t, trace.Decisions,
				"value %q must have at least one architectural decision", v)
			for _, d := range trace.Decisions {
				assert.NotEmpty(t, d.Label)
				assert.NotEmpty(t, d.ComponentRef)
				assert.NotEmpty(t, d.PDFSections)
			}
		})
	}
}

// Scenario 2: The deny-first principle grounds decisions in multiple value traces
// Given: a ValueArchitectureTraceRegistry
// When:  a caller queries decisions grounded in PrincipleDenyFirstHumanEscalation
// Then:  decisions appear in at least the HumanAuthority and Safety traces
//        reflecting the cross-cutting nature of deny-first in §2.3
func TestBDD_ValueArchitectureTrace_DenyFirstPrincipleIsSharedAcrossValues(t *testing.T) {
	// Given
	r := NewValueArchitectureTraceRegistry()

	// When
	decisions := r.DecisionsForPrinciple(PrincipleDenyFirstHumanEscalation)

	// Then
	require.GreaterOrEqual(t, len(decisions), 2,
		"deny-first must ground decisions in at least two value traces")

	// Verify the decisions come from distinct value traces
	seen := make(map[string]bool)
	for _, d := range decisions {
		seen[d.Label] = true
	}
	assert.GreaterOrEqual(t, len(seen), 2,
		"deny-first decisions must span at least two distinct architectural choices")
}

// Scenario 3: The §2.3 structural absences are explicitly modelled
// Given: a ValueArchitectureTraceRegistry
// When:  a caller asks for the architectural absences (§2.3 "mappings also reveal…")
// Then:  three absences are returned, including the absence of explicit planning graphs,
//        the absence of a single unified extension mechanism, and the absence of
//        session-scoped trust restoration across resume
func TestBDD_ValueArchitectureTrace_ArchitecturalAbsencesAreFormallyRepresented(t *testing.T) {
	// Given
	r := NewValueArchitectureTraceRegistry()

	// When
	absences := r.ArchitecturalAbsences()

	// Then
	require.Len(t, absences, 3, "§2.3 documents exactly three architectural absences")

	absenceText := strings.Join(absences, " | ")
	assert.Contains(t, strings.ToLower(absenceText), "planning",
		"absences must mention explicit planning graphs")
	assert.Contains(t, strings.ToLower(absenceText), "unified",
		"absences must mention the absence of a single unified extension mechanism")
	assert.Contains(t, strings.ToLower(absenceText), "trust",
		"absences must mention session-scoped trust restoration")
}

// Scenario 4: The §2.4 evaluative lens covers all five primary values
// Given: a ValueArchitectureTraceRegistry
// When:  a caller retrieves the LongTermCapabilityPreservation lens
// Then:  the lens evaluates all five design values, cites empirical evidence,
//        and is classified as cross-cutting (not a primary architectural driver)
func TestBDD_ValueArchitectureTrace_LongTermLensEvaluatesAllFiveValues(t *testing.T) {
	// Given
	r := NewValueArchitectureTraceRegistry()

	// When
	lens, ok := r.LensForID(LensLongTermCapabilityPreservation)

	// Then
	require.True(t, ok, "LensLongTermCapabilityPreservation must exist")
	assert.Equal(t, "2.4", lens.PDFSection,
		"lens must be attributed to §2.4")
	assert.Len(t, lens.EvaluatedValues, 5,
		"the evaluative lens applies across all five primary design values")

	for _, v := range AllDesignValues {
		assert.Contains(t, lens.EvaluatedValues, v,
			"lens must evaluate value %q", v)
	}

	assert.NotEmpty(t, lens.EmpiricalBasis,
		"lens must cite empirical evidence (Huang et al., Shen and Tamkin)")
	assert.Contains(t, lens.EmpiricalBasis, "paradox of supervision",
		"empirical basis must reference the paradox of supervision finding")
}

// Scenario 5: Every principle referenced by a trace is a valid Table 1 principle
// Given: a ValueArchitectureTraceRegistry and a DesignPrincipleRegistry
// When:  all five traces are inspected for their MotivatedPrinciples
// Then:  every referenced principle ID resolves in the DesignPrincipleRegistry
//        (no dangling references — the two registries stay consistent)
func TestBDD_ValueArchitectureTrace_NoDanglingPrincipleReferences(t *testing.T) {
	// Given
	r := NewValueArchitectureTraceRegistry()
	pr := NewDesignPrincipleRegistry()

	// When + Then
	dangles := 0
	for _, trace := range r.AllTraces() {
		for _, pid := range trace.MotivatedPrinciples {
			if !pr.IsValidPrinciple(pid) {
				t.Errorf("value %q references non-existent principle %q", trace.Value, pid)
				dangles++
			}
		}
		for _, d := range trace.Decisions {
			if !pr.IsValidPrinciple(d.PrincipleID) {
				t.Errorf("decision %q (value %q) references non-existent principle %q",
					d.Label, trace.Value, d.PrincipleID)
				dangles++
			}
		}
	}
	assert.Equal(t, 0, dangles, "no dangling principle references allowed")
}

// Scenario 6: Capability Amplification motivates the thin-harness architectural decision
// Given: a ValueArchitectureTraceRegistry
// When:  the Capability trace is retrieved and inspected
// Then:  a decision referencing the "1.6%" thin reasoning layer exists,
//        grounded in PrincipleMinimalScaffoldingMaximalHarness,
//        with a component reference to query.ts
func TestBDD_ValueArchitectureTrace_CapabilityMotivatesThinHarnessDecision(t *testing.T) {
	// Given
	r := NewValueArchitectureTraceRegistry()

	// When
	trace, ok := r.TraceForValue(DesignValueCapability)
	require.True(t, ok)

	// Then
	var thinHarnessDecision *ArchitecturalDecision
	for i := range trace.Decisions {
		if trace.Decisions[i].PrincipleID == PrincipleMinimalScaffoldingMaximalHarness {
			thinHarnessDecision = &trace.Decisions[i]
			break
		}
	}
	require.NotNil(t, thinHarnessDecision,
		"Capability trace must contain a decision for MinimalScaffoldingMaximalHarness")
	assert.Contains(t, thinHarnessDecision.Description, "1.6%",
		"decision must cite the ~1.6% AI logic ratio from §3.1")
	assert.Contains(t, thinHarnessDecision.ComponentRef, "query.ts")
}

// Scenario 7: Contextual Adaptability motivates transparent file-based config
// Given: a ValueArchitectureTraceRegistry
// When:  the Adaptability trace is retrieved
// Then:  a decision grounded in PrincipleTransparentFileBased exists,
//        referencing CLAUDE.md as the canonical source artefact
func TestBDD_ValueArchitectureTrace_AdaptabilityMotivatesFileBasedConfig(t *testing.T) {
	// Given
	r := NewValueArchitectureTraceRegistry()

	// When
	trace, ok := r.TraceForValue(DesignValueAdaptability)
	require.True(t, ok)

	// Then
	var fileBasedDecision *ArchitecturalDecision
	for i := range trace.Decisions {
		if trace.Decisions[i].PrincipleID == PrincipleTransparentFileBased {
			fileBasedDecision = &trace.Decisions[i]
			break
		}
	}
	require.NotNil(t, fileBasedDecision,
		"Adaptability trace must include a decision for TransparentFileBased")
	assert.Contains(t, fileBasedDecision.ComponentRef, "CLAUDE.md",
		"file-based config decision must reference CLAUDE.md")
}
