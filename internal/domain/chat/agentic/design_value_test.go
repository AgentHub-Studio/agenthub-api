package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- DesignValue constants ---

func TestDesignValue_FiveValues(t *testing.T) {
	assert.Equal(t, 5, len(agentic.AllDesignValues))
}

func TestDesignValue_AllValuesUnique(t *testing.T) {
	seen := map[agentic.DesignValue]bool{}
	for _, v := range agentic.AllDesignValues {
		assert.False(t, seen[v], "duplicate value %q", v)
		seen[v] = true
	}
}

func TestDesignValue_NamesMatchSpec(t *testing.T) {
	expected := map[agentic.DesignValue]bool{
		agentic.DesignValueHumanAuthority: true,
		agentic.DesignValueSafety:         true,
		agentic.DesignValueReliability:    true,
		agentic.DesignValueCapability:     true,
		agentic.DesignValueAdaptability:   true,
	}
	for _, v := range agentic.AllDesignValues {
		assert.True(t, expected[v], "unexpected value %q", v)
	}
}

// --- KnownValueTensions ---

func TestKnownValueTensions_FourTensions(t *testing.T) {
	assert.Equal(t, 4, len(agentic.KnownValueTensions))
}

func TestKnownValueTensions_AllValuesAreKnown(t *testing.T) {
	known := map[agentic.DesignValue]bool{}
	for _, v := range agentic.AllDesignValues {
		known[v] = true
	}
	for _, t2 := range agentic.KnownValueTensions {
		assert.True(t, known[t2.Value1], "unknown Value1 %q", t2.Value1)
		assert.True(t, known[t2.Value2], "unknown Value2 %q", t2.Value2)
	}
}

func TestKnownValueTensions_DescriptionsNotEmpty(t *testing.T) {
	for _, t2 := range agentic.KnownValueTensions {
		assert.NotEmpty(t, t2.Description)
	}
}

// --- NewDesignValueWeightVector ---

func TestNewDesignValueWeightVector_AllZero(t *testing.T) {
	dv := agentic.NewDesignValueWeightVector()
	for _, v := range agentic.AllDesignValues {
		assert.Equal(t, float64(0), dv.Get(v))
	}
}

func TestDefaultBalancedWeightVector_EqualWeights(t *testing.T) {
	dv := agentic.DefaultBalancedWeightVector()
	for _, v := range agentic.AllDesignValues {
		assert.InDelta(t, 0.2, dv.Get(v), 1e-9)
	}
	assert.InDelta(t, 1.0, dv.Sum(), 1e-9)
}

func TestSafetyFirstWeightVector_SafetyHighest(t *testing.T) {
	dv := agentic.SafetyFirstWeightVector()
	assert.Equal(t, agentic.DesignValueSafety, dv.Dominant())
	assert.InDelta(t, 1.0, dv.Sum(), 1e-9)
}

func TestCapabilityFirstWeightVector_CapabilityHighest(t *testing.T) {
	dv := agentic.CapabilityFirstWeightVector()
	assert.Equal(t, agentic.DesignValueCapability, dv.Dominant())
	assert.InDelta(t, 1.0, dv.Sum(), 1e-9)
}

// --- Set ---

func TestDesignValueWeightVector_Set_Valid(t *testing.T) {
	dv := agentic.NewDesignValueWeightVector()
	require.NoError(t, dv.Set(agentic.DesignValueSafety, 0.5))
	assert.InDelta(t, 0.5, dv.Get(agentic.DesignValueSafety), 1e-9)
}

func TestDesignValueWeightVector_Set_NegativeWeight(t *testing.T) {
	dv := agentic.NewDesignValueWeightVector()
	assert.Error(t, dv.Set(agentic.DesignValueSafety, -0.1))
}

func TestDesignValueWeightVector_Set_WeightOver1(t *testing.T) {
	dv := agentic.NewDesignValueWeightVector()
	assert.Error(t, dv.Set(agentic.DesignValueSafety, 1.01))
}

func TestDesignValueWeightVector_Set_UnknownValue(t *testing.T) {
	dv := agentic.NewDesignValueWeightVector()
	assert.Error(t, dv.Set("invalid_value", 0.5))
}

// --- Dominant ---

func TestDesignValueWeightVector_Dominant_ReturnsHighest(t *testing.T) {
	dv := agentic.NewDesignValueWeightVector()
	_ = dv.Set(agentic.DesignValueCapability, 0.7)
	_ = dv.Set(agentic.DesignValueSafety, 0.3)
	assert.Equal(t, agentic.DesignValueCapability, dv.Dominant())
}

func TestDesignValueWeightVector_Dominant_TieBreaksByCanonicalOrder(t *testing.T) {
	dv := agentic.NewDesignValueWeightVector() // all zero
	// all weights equal → first in AllDesignValues wins
	assert.Equal(t, agentic.AllDesignValues[0], dv.Dominant())
}

// --- Normalize ---

func TestDesignValueWeightVector_Normalize_SumsToOne(t *testing.T) {
	dv := agentic.NewDesignValueWeightVector()
	_ = dv.Set(agentic.DesignValueSafety, 0.3)
	_ = dv.Set(agentic.DesignValueCapability, 0.7)
	dv.Normalize()
	assert.InDelta(t, 1.0, dv.Sum(), 1e-9)
}

func TestDesignValueWeightVector_Normalize_AllZeroSetsEqual(t *testing.T) {
	dv := agentic.NewDesignValueWeightVector() // all zero
	dv.Normalize()
	assert.InDelta(t, 1.0, dv.Sum(), 1e-9)
	for _, v := range agentic.AllDesignValues {
		assert.InDelta(t, 0.2, dv.Get(v), 1e-9)
	}
}

// --- RankTensions ---

func TestDesignValueWeightVector_RankTensions_LengthPreserved(t *testing.T) {
	dv := agentic.DefaultBalancedWeightVector()
	ranked := dv.RankTensions(agentic.KnownValueTensions)
	assert.Equal(t, len(agentic.KnownValueTensions), len(ranked))
}

func TestDesignValueWeightVector_RankTensions_DescendingOrder(t *testing.T) {
	dv := agentic.NewDesignValueWeightVector()
	_ = dv.Set(agentic.DesignValueSafety, 0.9)
	_ = dv.Set(agentic.DesignValueCapability, 0.1)
	ranked := dv.RankTensions(agentic.KnownValueTensions)
	for i := 1; i < len(ranked); i++ {
		assert.GreaterOrEqual(t, ranked[i-1].Score, ranked[i].Score)
	}
}

func TestDesignValueWeightVector_ScoreTension_SumsValues(t *testing.T) {
	dv := agentic.NewDesignValueWeightVector()
	_ = dv.Set(agentic.DesignValueSafety, 0.4)
	_ = dv.Set(agentic.DesignValueCapability, 0.3)
	tension := agentic.DesignValueTension{Value1: agentic.DesignValueSafety, Value2: agentic.DesignValueCapability}
	score := dv.ScoreTension(tension)
	assert.InDelta(t, 0.7, score, 1e-9)
}
