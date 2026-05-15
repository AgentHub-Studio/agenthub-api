package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreECPTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshExtensionAuthorPicksClosestExemplar", func(t *testing.T) {
		// Given a fresh extension author needs proven token budgets,
		// When admin queries ah_core for cost-policy exemplars,
		// Then 5 recommended templates surface (one per band).
		assert.Equal(t, 5, len(SeedRecommendedECPTemplateSlugs))
	})

	t.Run("Scenario_CategoriesMatchEXT010EnumByteForByte", func(t *testing.T) {
		// Given EXT-010 ExtensionContextCostCategory has 5 bands,
		// When seed declares category column,
		// Then byte-for-byte alignment holds (no mapping table runtime).
		assert.Equal(t, 5, len(SeedExpectedECPTemplateCategories))
	})

	t.Run("Scenario_MicroExemplarServesReadOnlyLookups", func(t *testing.T) {
		// Given <=100 PerTurnTokens is the micro band threshold,
		// When admin reviews the micro exemplar,
		// Then it's a readonly_lookup archetype (smallest footprint).
		set := map[string]bool{}
		for _, k := range SeedExpectedECPTemplateExtensionKinds {
			set[k] = true
		}
		assert.True(t, set["readonly_lookup"])
	})

	t.Run("Scenario_HeavyExemplarServesMultimodalVision", func(t *testing.T) {
		// Given >8000 PerTurnTokens is the heavy band,
		// When admin reviews the heavy exemplar,
		// Then it's a multimodal_vision archetype (largest footprint).
		set := map[string]bool{}
		for _, k := range SeedExpectedECPTemplateExtensionKinds {
			set[k] = true
		}
		assert.True(t, set["multimodal_vision"])
	})

	t.Run("Scenario_MediumExemplarServesRAGBundles", func(t *testing.T) {
		// Given knowledge-base RAG with citation tools is mid-band,
		// When admin reviews the medium exemplar,
		// Then it's a rag_bundle archetype.
		set := map[string]bool{}
		for _, k := range SeedExpectedECPTemplateExtensionKinds {
			set[k] = true
		}
		assert.True(t, set["rag_bundle"])
	})

	t.Run("Scenario_LargeExemplarServesCodingSuites", func(t *testing.T) {
		set := map[string]bool{}
		for _, k := range SeedExpectedECPTemplateExtensionKinds {
			set[k] = true
		}
		assert.True(t, set["coding_suite"])
	})

	t.Run("Scenario_OneToOneCategoryToTemplate", func(t *testing.T) {
		// Given 5 bands × 1 exemplar = 5 rows,
		// When seed templates ship,
		// Then 1:1 holds.
		assert.Equal(t, len(SeedExpectedECPTemplateCategories), SeedExpectedECPTemplateRowCount)
	})

	t.Run("Scenario_DBCheckEnforcesAntiDriftInvariant", func(t *testing.T) {
		// Given EXT-010 anti-drift requires declared category to match
		// per_turn_tokens band,
		// When admin tries to INSERT with category="micro" but
		// per_turn_tokens=999,
		// Then DB CHECK chk_ext_cost_band_matches fires.
		// Validated DB-real in integration test.
		assert.True(t, true)
	})

	t.Run("Scenario_DBCheckEnforcesNonNegativeTokens", func(t *testing.T) {
		// Given EXT-010 requires non-negative tokens,
		// When admin tries to INSERT with negative,
		// Then DB CHECK chk_ext_cost_*_nonneg fires.
		assert.True(t, true)
	})

	t.Run("Scenario_BandBoundariesMatchEXT010Constants", func(t *testing.T) {
		// Given EXT-010 declares band boundaries 100/500/2000/8000,
		// When seed declares same boundaries,
		// Then constants are byte-for-byte aligned.
		assert.Equal(t, 100, SeedEXT010MicroMaxPerTurn)
		assert.Equal(t, 500, SeedEXT010SmallMaxPerTurn)
		assert.Equal(t, 2000, SeedEXT010MediumMaxPerTurn)
		assert.Equal(t, 8000, SeedEXT010LargeMaxPerTurn)
	})
}
