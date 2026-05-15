package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// BDD scenarios for DesignValueWeightVector — §13 five recurring design
// values and value-tension scoring (arXiv:2604.14228v1, Table 4 / §13).

func TestBDD_DesignValueWeightVector(t *testing.T) {
	t.Run("Scenario_BalancedVectorGivesEqualWeightToAllValues", func(t *testing.T) {
		// Given an agent deployment with no explicit policy preference
		// When the balanced default weight vector is applied
		// Then all five design values have equal weight (0.2 each, sum = 1.0)
		dv := agentic.DefaultBalancedWeightVector()
		assert.InDelta(t, 1.0, dv.Sum(), 1e-9)
		for _, v := range agentic.AllDesignValues {
			assert.InDelta(t, 0.2, dv.Get(v), 1e-9,
				"value %q must have weight 0.2 in balanced vector", v)
		}
	})

	t.Run("Scenario_SafetyFirstVectorDominatesWithSafety", func(t *testing.T) {
		// Given a high-stakes regulated deployment (e.g., financial, healthcare)
		// When the safety-first weight vector is configured
		// Then safety is the dominant design value
		dv := agentic.SafetyFirstWeightVector()
		assert.Equal(t, agentic.DesignValueSafety, dv.Dominant())
	})

	t.Run("Scenario_TensionRankingPrioritisesHighWeightPairs", func(t *testing.T) {
		// Given a deployment that heavily weights safety and human authority
		// When tensions are ranked
		// Then the Safety×HumanAuthority tension scores highest
		dv := agentic.NewDesignValueWeightVector()
		require.NoError(t, dv.Set(agentic.DesignValueSafety, 0.6))
		require.NoError(t, dv.Set(agentic.DesignValueHumanAuthority, 0.4))

		ranked := dv.RankTensions(agentic.KnownValueTensions)
		assert.Equal(t, agentic.DesignValueHumanAuthority, ranked[0].Tension.Value1,
			"Safety×HumanAuthority tension must rank first")
		assert.Equal(t, agentic.DesignValueSafety, ranked[0].Tension.Value2)
		assert.InDelta(t, 1.0, ranked[0].Score, 1e-9)
	})

	t.Run("Scenario_NormalizePreservesRatiosBetweenValues", func(t *testing.T) {
		// Given a vector where safety=0.8, capability=0.2 (4:1 ratio)
		// When normalized
		// Then the 4:1 ratio is preserved and sum = 1.0
		dv := agentic.NewDesignValueWeightVector()
		require.NoError(t, dv.Set(agentic.DesignValueSafety, 0.8))
		require.NoError(t, dv.Set(agentic.DesignValueCapability, 0.2))
		dv.Normalize()

		assert.InDelta(t, 1.0, dv.Sum(), 1e-9)
		safetyW := dv.Get(agentic.DesignValueSafety)
		capW := dv.Get(agentic.DesignValueCapability)
		assert.InDelta(t, 4.0, safetyW/capW, 0.01,
			"safety:capability ratio must be preserved as 4:1")
	})

	t.Run("Scenario_AllZeroNormalizeProducesEqualWeights", func(t *testing.T) {
		// Given a freshly created vector with all weights at 0 (no preference)
		// When normalized
		// Then equal weights are assigned (fallback to balanced)
		dv := agentic.NewDesignValueWeightVector()
		dv.Normalize()
		for _, v := range agentic.AllDesignValues {
			assert.InDelta(t, 0.2, dv.Get(v), 1e-9)
		}
	})

	t.Run("Scenario_FourKnownTensionsCoverTableFour", func(t *testing.T) {
		// Given Table 4 from §13 (arXiv:2604.14228v1)
		// When the known tensions catalog is inspected
		// Then all four tensions from the paper are present
		assert.Len(t, agentic.KnownValueTensions, 4,
			"Table 4 has exactly 4 tensions: Authority×Safety, Safety×Capability, Adaptability×Safety, Capability×Reliability")
		pairs := map[string]bool{}
		for _, t2 := range agentic.KnownValueTensions {
			key := string(t2.Value1) + "+" + string(t2.Value2)
			pairs[key] = true
		}
		assert.True(t, pairs["human_authority+safety"])
		assert.True(t, pairs["safety+capability"])
		assert.True(t, pairs["adaptability+safety"])
		assert.True(t, pairs["capability+reliability"])
	})
}
