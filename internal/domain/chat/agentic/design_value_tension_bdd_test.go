package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for Table 4 design value tensions (arXiv:2604.14228v1, §11.2).

func TestBDD_DesignValueTensionRegistry(t *testing.T) {
	t.Run("Scenario_FiveTensionsMatchTable4RowCount", func(t *testing.T) {
		// Given Table 4 defines exactly five design value tensions
		// When all tensions are listed from the registry
		// Then exactly five profiles are returned in Table 4 order
		r := NewDesignValueTensionRegistry()
		all := r.AllTensions()
		assert.Equal(t, 5, len(all))
		assert.Equal(t, TensionAuthoritySafety, all[0].ID)
		assert.Equal(t, TensionCapabilityReliability, all[4].ID)
	})

	t.Run("Scenario_CapabilityInvolvedInThreeTensionsFromTable4", func(t *testing.T) {
		// Given Table 4 shows capability appears in safety_capability,
		//   capability_adaptability, and capability_reliability rows
		// When tensions involving capability are queried
		// Then exactly three tensions are returned
		r := NewDesignValueTensionRegistry()
		tensions := r.TensionsInvolvingValue(DesignValueCapability)
		assert.Equal(t, 3, len(tensions))
		ids := map[DesignValueTensionID]bool{}
		for _, p := range tensions {
			ids[p.ID] = true
		}
		assert.True(t, ids[TensionSafetyCapability])
		assert.True(t, ids[TensionCapabilityAdaptability])
		assert.True(t, ids[TensionCapabilityReliability])
	})

	t.Run("Scenario_SafetyInvolvedInThreeTensionsFromTable4", func(t *testing.T) {
		// Given Table 4 shows safety appears in authority_safety,
		//   safety_capability, and adaptability_safety rows
		// When tensions involving safety are queried
		// Then exactly three tensions are returned
		r := NewDesignValueTensionRegistry()
		tensions := r.TensionsInvolvingValue(DesignValueSafety)
		assert.Equal(t, 3, len(tensions))
	})

	t.Run("Scenario_TensionLabelsAndEvidenceArePopulated", func(t *testing.T) {
		// Given each row in Table 4 has a human-readable label and evidence note
		// When all tensions are inspected
		// Then no label or evidence summary is empty
		r := NewDesignValueTensionRegistry()
		for _, p := range r.AllTensions() {
			assert.NotEmpty(t, p.TensionLabel)
			assert.NotEmpty(t, p.EvidenceSummary)
		}
	})

	t.Run("Scenario_InvalidTensionIDRejectedByRegistry", func(t *testing.T) {
		// Given only the five Table 4 slugs are valid
		// When an unrecognised ID is passed to IsValidTension
		// Then false is returned and Profile returns false
		r := NewDesignValueTensionRegistry()
		assert.False(t, r.IsValidTension("not_real"))
		_, ok := r.Profile("not_real")
		assert.False(t, ok)
	})
}
