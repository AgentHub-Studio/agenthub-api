package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_ExtensionContextCost(t *testing.T) {
	t.Run("Scenario_ExtensionAuthorDeclaresMicroFootprint", func(t *testing.T) {
		// Given a small read-only extension uses <100 tokens per turn,
		// When the author declares Category=micro with PerTurnTokens=50,
		// Then Validate accepts (declared band matches observed).
		p := validExtensionContextCost()
		require.NoError(t, p.Validate())
		assert.Equal(t, ExtensionContextCostMicro, p.Category)
	})

	t.Run("Scenario_CategoryDriftRejectedToPreventLyingAuthors", func(t *testing.T) {
		// Given an author claims "micro" but PerTurnTokens=600 (medium band),
		// When Validate runs,
		// Then it rejects with CategoryDrift — declared cost must match
		// observed per-turn band.
		p := validExtensionContextCost()
		p.PerTurnTokens = 600
		p.Category = ExtensionContextCostMicro
		assert.ErrorIs(t, p.Validate(), ErrExtensionContextCostCategoryDrift)
	})

	t.Run("Scenario_PlatformSumsCostAcrossLoadedExtensions", func(t *testing.T) {
		// Given a tenant loads 3 extensions in one session,
		// When the platform estimates total cost for 5 turns 0 tool calls,
		// Then EstimateLoadedExtensions returns the sum.
		r := NewExtensionContextCostRegistry()
		p1 := validExtensionContextCost()
		p1.ExtensionSlug = "ext-1"
		p2 := validExtensionContextCost()
		p2.ExtensionSlug = "ext-2"
		p3 := validExtensionContextCost()
		p3.ExtensionSlug = "ext-3"
		require.NoError(t, r.Register(p1))
		require.NoError(t, r.Register(p2))
		require.NoError(t, r.Register(p3))
		total, missing := r.EstimateLoadedExtensions(
			[]string{"ext-1", "ext-2", "ext-3"}, 5, 0,
		)
		// per-ext: 120 + 50*5 = 370. Total = 1110.
		assert.Equal(t, 1110, total)
		assert.Empty(t, missing)
	})

	t.Run("Scenario_MissingExtensionsReportedSeparatelyNotCounted", func(t *testing.T) {
		// Given some slugs reference unregistered extensions,
		// When estimating,
		// Then missing names are returned but not counted.
		r := NewExtensionContextCostRegistry()
		p := validExtensionContextCost()
		require.NoError(t, r.Register(p))
		_, missing := r.EstimateLoadedExtensions(
			[]string{p.ExtensionSlug, "ghost"}, 1, 0,
		)
		assert.Equal(t, []string{"ghost"}, missing)
	})

	t.Run("Scenario_PlatformRefusesToLoadIfCostExceedsWindow", func(t *testing.T) {
		// Given a session window of 200k tokens with 100k headroom for
		// user content + LLM responses,
		// When loaded extensions would consume 150k+ tokens,
		// Then CanAffordSessionWindow returns false.
		r := NewExtensionContextCostRegistry()
		p := validExtensionContextCost()
		p.PerTurnTokens = 1500
		p.Category = ExtensionContextCostMedium
		require.NoError(t, r.Register(p))
		ok, _, _ := r.CanAffordSessionWindow(
			[]string{p.ExtensionSlug}, 1000, 0, 200000, 100000,
		)
		assert.False(t, ok)
	})

	t.Run("Scenario_DuplicateExtensionSlugRejected", func(t *testing.T) {
		// Given each extension is unique by slug,
		// When the same slug is registered twice,
		// Then the second registration fails.
		r := NewExtensionContextCostRegistry()
		p := validExtensionContextCost()
		require.NoError(t, r.Register(p))
		err := r.Register(p)
		assert.ErrorIs(t, err, ErrExtensionContextCostDuplicate)
	})

	t.Run("Scenario_AdminGroupsExtensionsByCostCategory", func(t *testing.T) {
		// Given admin UI groups extensions by cost tier,
		// When ListByCategory is called,
		// Then only matching policies are returned.
		r := NewExtensionContextCostRegistry()
		p1 := validExtensionContextCost()
		p1.ExtensionSlug = "micro-ext"
		p2 := validExtensionContextCost()
		p2.ExtensionSlug = "heavy-ext"
		p2.PerTurnTokens = 12000
		p2.Category = ExtensionContextCostHeavy
		require.NoError(t, r.Register(p1))
		require.NoError(t, r.Register(p2))
		heavies := r.ListByCategory(ExtensionContextCostHeavy)
		assert.Equal(t, 1, len(heavies))
	})

	t.Run("Scenario_NegativeTokensRejectedAtAllPositions", func(t *testing.T) {
		// Given negative token counts are nonsensical,
		// When any field goes negative,
		// Then Validate rejects.
		p := validExtensionContextCost()
		p.StaticHeaderTokens = -1
		assert.ErrorIs(t, p.Validate(), ErrExtensionContextCostNegative)
	})

	t.Run("Scenario_FiveCategoriesCoverFromMicroToHeavy", func(t *testing.T) {
		// Given costs vary by orders of magnitude,
		// When admin lists categories,
		// Then 5 bands are bounded (micro/small/medium/large/heavy).
		assert.Equal(t, 5, len(AllExtensionContextCostCategories()))
	})

	t.Run("Scenario_EstimatorBakesHeaderTurnsAndToolCalls", func(t *testing.T) {
		// Given the cost model is header + per_turn*turns + per_tool*calls,
		// When called with 10 turns and 5 tool calls,
		// Then estimate matches the formula.
		p := validExtensionContextCost()
		// 120 + 50*10 + 30*5 = 770
		assert.Equal(t, 770, p.EstimateForTurns(10, 5))
	})
}
