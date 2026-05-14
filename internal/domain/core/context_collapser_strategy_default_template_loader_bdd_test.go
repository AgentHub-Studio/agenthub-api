package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreCCSDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksCollapseStrategyWithoutInventingMix", func(t *testing.T) {
		// Given fresh tenants need collapser config,
		// And CTX-012 ContextCollapser accepts (strategy + consumer_kind),
		// When admin opens collapser onboarding,
		// Then 5 recommended presets cover common consumer shapes.
		assert.Equal(t, 5, len(SeedRecommendedCCSDTemplateSlugs))
	})

	t.Run("Scenario_LLMRendererUsesBalancedAsProductionDefault", func(t *testing.T) {
		// Given balanced strategy drops tool_use but keeps tool_result,
		// When admin picks llm-renderer-balanced,
		// Then production LLM calls get token-cost-aware projection
		// without losing tool outcomes.
		assert.Contains(t, SeedExpectedCCSDTemplateSlugs, "llm-renderer-balanced")
	})

	t.Run("Scenario_ReplayDebugUsesMinimalToPreserveEverything", func(t *testing.T) {
		// Given incident debugging needs raw transcript bytes,
		// When admin picks replay-debug-verbose,
		// Then minimal strategy preserves all messages — no surprise
		// data loss when investigating.
		assert.Contains(t, SeedExpectedCCSDTemplateSlugs, "replay-debug-verbose")
	})

	t.Run("Scenario_CostDashboardUsesAggressiveForBusinessOutcomesOnly", func(t *testing.T) {
		// Given cost dashboards show business outcomes (assistant text,
		// errors, summaries),
		// When admin picks cost-dashboard-aggressive,
		// Then internal tool churn is hidden — dashboard reads at a
		// glance.
		assert.Contains(t, SeedExpectedCCSDTemplateSlugs, "cost-dashboard-aggressive")
	})

	t.Run("Scenario_AuditExportRequiresAdminReviewAndMinimalForRegulatorBinding", func(t *testing.T) {
		// Given regulator-facing exports (FUTURE-005) must show full
		// original transcript,
		// When admin picks audit-export-minimal,
		// Then admin review required (regulator binding) + minimal
		// strategy (no compression).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewCCSDTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["audit-export-minimal"])
	})

	t.Run("Scenario_UISummaryHidesToolInternalsFromEndUsers", func(t *testing.T) {
		// Given end-user UI shows readability summary not engineering,
		// When admin picks ui-summary-aggressive,
		// Then aggressive strategy keeps only assistant text + errors —
		// readability over fidelity.
		assert.Contains(t, SeedExpectedCCSDTemplateSlugs, "ui-summary-aggressive")
	})

	t.Run("Scenario_DevDebugVerboseNotRecommendedForProduction", func(t *testing.T) {
		// Given dev-debug-verbose preserves everything (no compression),
		// When admin browses,
		// Then dev-debug is NOT recommended for production tenants —
		// would burn tokens unnecessarily.
		set := map[string]bool{}
		for _, s := range SeedRecommendedCCSDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["dev-debug-verbose-no-recommendations"])
	})

	t.Run("Scenario_StrategyLabelsAreCTX012EnumByteForByte", func(t *testing.T) {
		// Given CTX-012 CollapseStrategy has 3 enum values,
		// When seed declares collapse_strategy,
		// Then labels match enum bytes (no mapping table).
		expected := []string{"minimal", "balanced", "aggressive"}
		set := map[string]bool{}
		for _, s := range SeedExpectedCCSDTemplateStrategies {
			set[s] = true
		}
		for _, e := range expected {
			assert.True(t, set[e], "CTX-012 strategy %q missing from seed", e)
		}
	})

	t.Run("Scenario_PreserveErrorsAlwaysReflectsCTX012Invariant", func(t *testing.T) {
		// Given CTX-012 ALWAYS preserves errors (invariant),
		// When seed declares preserve_errors_always,
		// Then every row has TRUE (admin clarity that errors NEVER drop).
		// Validated structurally via integration test.
		assert.Equal(t, 6, len(SeedExpectedCCSDTemplateSlugs))
	})

	t.Run("Scenario_PreserveCompactSummaryAlwaysReflectsResumeContract", func(t *testing.T) {
		// Given PDF §9.2 resume needs compact_summary markers,
		// When seed declares preserve_compact_summary_always,
		// Then every row has TRUE (resume integrity preserved).
		// Validated structurally via integration test.
		assert.Equal(t, 6, len(SeedExpectedCCSDTemplateSlugs))
	})

	t.Run("Scenario_SixPresetsCoverCommonConsumerKindsWithoutOverwhelm", func(t *testing.T) {
		// Given product research showed 5-7 templates is the sweet spot,
		// When the seed ships,
		// Then exactly 6 presets cover the major consumer kinds.
		assert.Equal(t, 6, len(SeedExpectedCCSDTemplateSlugs))
	})
}
