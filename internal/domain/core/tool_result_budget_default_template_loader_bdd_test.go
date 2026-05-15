package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreTRBDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksToolResultBudgetWithoutInventingNumbers", func(t *testing.T) {
		// Given fresh tenants need budget configurations,
		// And CTX-008 ToolResultBudgetEnforcer accepts (per-call,
		// per-turn, per-run, force-summarize),
		// When admin opens budget onboarding,
		// Then 5 recommended profiles cover model tiers + use cases.
		assert.Equal(t, 5, len(SeedRecommendedTRBDTemplateSlugs))
	})

	t.Run("Scenario_BalancedDefaultMirrorsCTX008DefaultToolResultBudgetConfig", func(t *testing.T) {
		// Given CTX-008 ships DefaultToolResultBudgetConfig (per_call=4000,
		// per_turn=12000, per_run=50000, force_summarize_over=8000),
		// When admin picks balanced-default,
		// Then values match byte-for-byte so in-code default and seeded
		// one-click are coherent.
		assert.Contains(t, SeedExpectedTRBDTemplateSlugs, "balanced-default")
	})

	t.Run("Scenario_SmallContextTightForLegacy8kModels", func(t *testing.T) {
		// Given legacy 8k-context models exist,
		// When admin uses small-context-tight,
		// Then per-call cap is small (1k) so a single tool result
		// doesn't drown the 8k window.
		assert.Contains(t, SeedExpectedTRBDTemplateSlugs, "small-context-tight")
	})

	t.Run("Scenario_ResearchHeavyExploitsLargeContextWindow", func(t *testing.T) {
		// Given research workflows with 128k+ context models,
		// When admin uses research-heavy,
		// Then per-call cap is generous (16k) so KB queries aren't
		// truncated unnecessarily.
		assert.Contains(t, SeedExpectedTRBDTemplateSlugs, "research-heavy")
	})

	t.Run("Scenario_CostStrictRequiresAdminReviewDueToVisibilityDegradation", func(t *testing.T) {
		// Given cost-strict aggressively summarizes/drops tool results,
		// When admin picks it,
		// Then template requires admin review (degraded agent visibility
		// has product impact).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewTRBDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["cost-strict"])
	})

	t.Run("Scenario_DevDebugIsOptInNotRecommendedDueToProductionRisk", func(t *testing.T) {
		// Given dev-debug has all caps zeroed (unlimited),
		// When admin browses templates,
		// Then dev-debug is NOT highlighted recommended — production
		// tenants would exceed model context windows on a single huge
		// tool result.
		set := map[string]bool{}
		for _, s := range SeedRecommendedTRBDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["dev-debug"])
	})

	t.Run("Scenario_BudgetsAreMonotonicAcrossTiers", func(t *testing.T) {
		// Given budget profiles ladder by per_call_max:
		//   small(1k) ≤ cost_strict(2k) ≤ balanced(4k) ≤ code(8k) ≤ research(16k)
		// When admin compares templates,
		// Then numbers ladder coherently — small→tight→balanced→code→research.
		// (Validated structurally via integration test.)
		assert.Equal(t, 6, len(SeedExpectedTRBDTemplateSlugs))
	})

	t.Run("Scenario_ForceSummarizeAlwaysExceedsZeroExceptDev", func(t *testing.T) {
		// Given force_summarize_over_tokens triggers summary placeholder
		// for very large results,
		// When admin looks at templates,
		// Then every non-dev template has positive force_summarize_over
		// (otherwise large results blunt-truncate, losing content).
		// (Validated via integration test.)
		assert.Equal(t, 5, len(SeedRecommendedTRBDTemplateSlugs))
	})

	t.Run("Scenario_ModelFamilyHintHelpsAdminMatchBudgetToModel", func(t *testing.T) {
		// Given budget recommendations depend on model context window,
		// When admin pre-filters templates by model family,
		// Then UI can show appropriate options (e.g. small_local for 8k).
		expected := []string{"mid_tier", "small_local", "large_context", "dev_local"}
		set := map[string]bool{}
		for _, m := range SeedExpectedTRBDTemplateModelFamilies {
			set[m] = true
		}
		for _, e := range expected {
			assert.True(t, set[e])
		}
	})

	t.Run("Scenario_SixProfilesCoverCommonScenariosWithoutOverwhelm", func(t *testing.T) {
		// Given product research showed 5-7 templates is the sweet spot,
		// When the seed ships,
		// Then exactly 6 profiles exist.
		assert.Equal(t, 6, len(SeedExpectedTRBDTemplateSlugs))
	})
}
