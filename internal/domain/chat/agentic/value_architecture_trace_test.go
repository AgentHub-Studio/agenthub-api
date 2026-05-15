package agentic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValueArchitectureTraceRegistry_AllTraces_FiveValues(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	traces := r.AllTraces()
	assert.Len(t, traces, 5, "§2.3 must have exactly five value traces")
}

func TestValueArchitectureTraceRegistry_AllTraces_CanonicalOrder(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	traces := r.AllTraces()
	require.Len(t, traces, 5)
	assert.Equal(t, DesignValueHumanAuthority, traces[0].Value)
	assert.Equal(t, DesignValueSafety, traces[1].Value)
	assert.Equal(t, DesignValueReliability, traces[2].Value)
	assert.Equal(t, DesignValueCapability, traces[3].Value)
	assert.Equal(t, DesignValueAdaptability, traces[4].Value)
}

func TestValueArchitectureTraceRegistry_TraceForValue_HumanAuthority(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	trace, ok := r.TraceForValue(DesignValueHumanAuthority)
	require.True(t, ok)
	assert.Equal(t, DesignValueHumanAuthority, trace.Value)
	assert.GreaterOrEqual(t, len(trace.MotivatedPrinciples), 3,
		"HumanAuthority must motivate at least 3 principles")
	assert.NotEmpty(t, trace.Decisions)
}

func TestValueArchitectureTraceRegistry_TraceForValue_Safety(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	trace, ok := r.TraceForValue(DesignValueSafety)
	require.True(t, ok)
	assert.Equal(t, DesignValueSafety, trace.Value)
	assert.Contains(t, trace.MotivatedPrinciples, PrincipleDefenseInDepthLayered,
		"Safety must motivate defense-in-depth")
	assert.Contains(t, trace.MotivatedPrinciples, PrincipleIsolatedSubagentBoundaries,
		"Safety must motivate isolated subagent boundaries")
}

func TestValueArchitectureTraceRegistry_TraceForValue_Reliability(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	trace, ok := r.TraceForValue(DesignValueReliability)
	require.True(t, ok)
	assert.Contains(t, trace.MotivatedPrinciples, PrincipleContextAsScarceResource)
	assert.Contains(t, trace.MotivatedPrinciples, PrincipleAppendOnlyDurableState)
}

func TestValueArchitectureTraceRegistry_TraceForValue_Capability(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	trace, ok := r.TraceForValue(DesignValueCapability)
	require.True(t, ok)
	assert.Contains(t, trace.MotivatedPrinciples, PrincipleMinimalScaffoldingMaximalHarness)
	assert.Contains(t, trace.MotivatedPrinciples, PrincipleComposableMultiMechanism)
}

func TestValueArchitectureTraceRegistry_TraceForValue_Adaptability(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	trace, ok := r.TraceForValue(DesignValueAdaptability)
	require.True(t, ok)
	assert.Contains(t, trace.MotivatedPrinciples, PrincipleTransparentFileBased)
	assert.Contains(t, trace.MotivatedPrinciples, PrincipleExternalizedProgrammablePolicy)
}

func TestValueArchitectureTraceRegistry_TraceForValue_Unknown(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	_, ok := r.TraceForValue(DesignValue("nonexistent_value"))
	assert.False(t, ok, "unknown value must return false")
}

func TestValueArchitectureTraceRegistry_AllDecisionsHaveComponentRef(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	for _, trace := range r.AllTraces() {
		for _, d := range trace.Decisions {
			assert.NotEmpty(t, d.ComponentRef,
				"decision %q for value %q must have a component reference",
				d.Label, trace.Value)
		}
	}
}

func TestValueArchitectureTraceRegistry_AllDecisionsHavePDFSections(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	for _, trace := range r.AllTraces() {
		for _, d := range trace.Decisions {
			assert.NotEmpty(t, d.PDFSections,
				"decision %q must reference at least one PDF section", d.Label)
		}
	}
}

func TestValueArchitectureTraceRegistry_AllDecisionsReferenceValidPrinciple(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	pr := NewDesignPrincipleRegistry()
	for _, trace := range r.AllTraces() {
		for _, d := range trace.Decisions {
			assert.True(t, pr.IsValidPrinciple(d.PrincipleID),
				"decision %q references unknown principle %q", d.Label, d.PrincipleID)
		}
	}
}

func TestValueArchitectureTraceRegistry_DecisionsForPrinciple_DenyFirst(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	decisions := r.DecisionsForPrinciple(PrincipleDenyFirstHumanEscalation)
	assert.GreaterOrEqual(t, len(decisions), 2,
		"deny-first principle appears in multiple value traces")
}

func TestValueArchitectureTraceRegistry_DecisionsForPrinciple_Unknown(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	decisions := r.DecisionsForPrinciple(DesignPrincipleID("nonexistent"))
	assert.Empty(t, decisions)
}

func TestValueArchitectureTraceRegistry_DecisionsForValue_ReturnsCopy(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	d1 := r.DecisionsForValue(DesignValueSafety)
	d2 := r.DecisionsForValue(DesignValueSafety)
	assert.Equal(t, len(d1), len(d2))
}

func TestValueArchitectureTraceRegistry_DecisionsForValue_UnknownValue(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	decisions := r.DecisionsForValue(DesignValue("bogus"))
	assert.Nil(t, decisions)
}

func TestValueArchitectureTraceRegistry_EvaluativeLenses_HasLongTermLens(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	lenses := r.EvaluativeLenses()
	require.NotEmpty(t, lenses)
	var found bool
	for _, l := range lenses {
		if l.ID == LensLongTermCapabilityPreservation {
			found = true
			assert.NotEmpty(t, l.Label)
			assert.NotEmpty(t, l.Description)
			assert.NotEmpty(t, l.EmpiricalBasis)
			assert.Equal(t, "2.4", l.PDFSection)
			assert.Len(t, l.EvaluatedValues, 5,
				"the long-term lens evaluates all five values")
		}
	}
	assert.True(t, found, "LensLongTermCapabilityPreservation must be present")
}

func TestValueArchitectureTraceRegistry_LensForID_Found(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	lens, ok := r.LensForID(LensLongTermCapabilityPreservation)
	require.True(t, ok)
	assert.Equal(t, LensLongTermCapabilityPreservation, lens.ID)
	assert.Contains(t, lens.EmpiricalBasis, "paradox of supervision")
}

func TestValueArchitectureTraceRegistry_LensForID_NotFound(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	_, ok := r.LensForID(EvaluativeLensID("nonexistent_lens"))
	assert.False(t, ok)
}

func TestValueArchitectureTraceRegistry_ArchitecturalAbsences_ThreeAbsences(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	absences := r.ArchitecturalAbsences()
	assert.Len(t, absences, 3, "§2.3 documents exactly three architectural absences")
}

func TestValueArchitectureTraceRegistry_ArchitecturalAbsences_ContainsPlanningGraphs(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	absences := r.ArchitecturalAbsences()
	var found bool
	for _, a := range absences {
		if strings.Contains(strings.ToLower(a), "planning") {
			found = true
		}
	}
	assert.True(t, found, "absences must mention explicit planning graphs")
}

func TestValueArchitectureTraceRegistry_EachValueHasAtLeastFourDecisions(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	for _, trace := range r.AllTraces() {
		assert.GreaterOrEqual(t, len(trace.Decisions), 4,
			"value %q must have at least 4 architectural decisions", trace.Value)
	}
}

func TestValueArchitectureTraceRegistry_AllMotivatedPrinciplesAreValid(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	pr := NewDesignPrincipleRegistry()
	for _, trace := range r.AllTraces() {
		for _, pid := range trace.MotivatedPrinciples {
			assert.True(t, pr.IsValidPrinciple(pid),
				"value %q references unknown principle %q", trace.Value, pid)
		}
	}
}

func TestArchitecturalDecision_FieldsNonEmpty(t *testing.T) {
	r := NewValueArchitectureTraceRegistry()
	for _, trace := range r.AllTraces() {
		for _, d := range trace.Decisions {
			assert.NotEmpty(t, d.Label, "decision must have a label")
			assert.NotEmpty(t, d.Description, "decision must have a description")
			assert.NotEmpty(t, d.PrincipleID, "decision must have a principle ID")
		}
	}
}

func TestEvaluativeLensID_LongTermCapabilityPreservationValue(t *testing.T) {
	assert.Equal(t, EvaluativeLensID("long_term_capability_preservation"),
		LensLongTermCapabilityPreservation)
}
