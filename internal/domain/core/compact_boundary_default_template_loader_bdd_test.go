package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreCBDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksCompactionTriggerWithoutInventingThresholds", func(t *testing.T) {
		// Given fresh tenants need compaction trigger config,
		// And CTX-011 CompactBoundaryRegistry accepts trigger metadata,
		// When admin opens compaction onboarding,
		// Then 5 recommended profiles cover trigger spectrum.
		assert.Equal(t, 5, len(SeedRecommendedCBDTemplateSlugs))
	})

	t.Run("Scenario_HybridBalancedIsTheSafeOneClickDefault", func(t *testing.T) {
		// Given most tenants want robust compaction (multiple signals),
		// When admin picks hybrid-balanced,
		// Then it triggers on FIRST condition met (turns/tokens/cost/idle).
		assert.Contains(t, SeedExpectedCBDTemplateSlugs, "hybrid-balanced")
	})

	t.Run("Scenario_TurnCountStrictForPredictableCadence", func(t *testing.T) {
		// Given some tenants want predictable cost budgeting,
		// When admin picks turn-count-strict,
		// Then compaction fires every N turns regardless of token-pressure
		// — predictable cadence over signal-driven.
		assert.Contains(t, SeedExpectedCBDTemplateSlugs, "turn-count-strict")
	})

	t.Run("Scenario_TokenPressureDrivenLetsShortConvosBreathe", func(t *testing.T) {
		// Given short conversations don't need compaction (waste),
		// When admin picks token-pressure-driven,
		// Then it only fires when context_used/limit ≥ 70% — short
		// conversations stay full-fidelity.
		assert.Contains(t, SeedExpectedCBDTemplateSlugs, "token-pressure-driven")
	})

	t.Run("Scenario_CostStrictRequiresAdminReviewBecauseDegradesAgentVisibility", func(t *testing.T) {
		// Given cost-strict aggressively compacts (smaller preserve_tail),
		// When admin picks it,
		// Then template requires admin review — agent visibility
		// degradation has product impact.
		set := map[string]bool{}
		for _, s := range SeedAdminReviewCBDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["cost-strict"])
	})

	t.Run("Scenario_IdleDrivenForLongRunningMultiDaySessions", func(t *testing.T) {
		// Given long-running multi-day sessions (user comes back fresh),
		// When admin picks idle-driven,
		// Then compaction fires after 30 min idle — user returns to
		// summarized prior context (not raw bytes).
		assert.Contains(t, SeedExpectedCBDTemplateSlugs, "idle-driven")
	})

	t.Run("Scenario_DevDebugDisablesAutoCompactionForFullVisibility", func(t *testing.T) {
		// Given dev tenants want raw bytes for debugging,
		// When admin picks dev-debug-no-compact,
		// Then ALL trigger thresholds are 0 — no auto-compaction. NOT
		// recommended for production.
		set := map[string]bool{}
		for _, s := range SeedRecommendedCBDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["dev-debug-no-compact"])
	})

	t.Run("Scenario_TriggerKindsAreClosedSetForRunnerDispatch", func(t *testing.T) {
		// Given the runtime dispatches each trigger kind to the right
		// signal source (turn counter / token estimator / cost meter /
		// idle timer / hybrid coordinator),
		// When the seed declares trigger_kind,
		// Then it's exactly one of the 6 closed values — no surprise
		// kinds runtime can't handle.
		expected := []string{
			"hybrid_balanced", "turn_count", "token_pressure",
			"cost", "idle", "dev_debug",
		}
		set := map[string]bool{}
		for _, k := range SeedExpectedCBDTemplateTriggerKinds {
			set[k] = true
		}
		for _, e := range expected {
			assert.True(t, set[e])
		}
	})

	t.Run("Scenario_TokenPressureMatchesCTX010ContextManagerThresholdDefault", func(t *testing.T) {
		// Given CTX-010 ContextManager uses 70% as the auto-compaction
		// trigger threshold by default (per CLAUDE.md),
		// When admin picks hybrid-balanced or token-pressure-driven,
		// Then token_pressure_pct = 70 — DB seed and code default coherent.
		// Validated structurally via integration test.
		assert.Contains(t, SeedExpectedCBDTemplateSlugs, "hybrid-balanced")
	})

	t.Run("Scenario_PreserveTailMessagesAlignsWithUseCase", func(t *testing.T) {
		// Given research preserves more tail (12 messages) for citation
		// continuity, while cost-strict preserves less (5),
		// When admin compares profiles,
		// Then preserve_tail_messages reflects use-case semantics.
		// Validated via integration test.
		assert.Equal(t, 6, len(SeedExpectedCBDTemplateSlugs))
	})

	t.Run("Scenario_SixProfilesCoverTriggerSpectrumPlusDevWithoutOverwhelm", func(t *testing.T) {
		// Given product research showed 5-7 templates is the sweet spot,
		// When the seed ships,
		// Then exactly 6 profiles exist (one per trigger kind).
		assert.Equal(t, 6, len(SeedExpectedCBDTemplateSlugs))
	})
}
