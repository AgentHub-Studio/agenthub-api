package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreBMPDTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantPicksBubbleDepthFromCatalog", func(t *testing.T) {
		// Given fresh tenants must pick a PERM-003b bubble depth before
		// spawning multi-tier orchestration but reasoning about max-depth
		// + parent-required + history-capture trade-offs is error-prone,
		// When admin opens bubble-mode onboarding,
		// Then 5 recommended templates surface on the depth ladder.
		assert.Equal(t, 5, len(SeedRecommendedBMPDTemplateSlugs))
	})

	t.Run("Scenario_LeafAutoDenyForBottomOfTree", func(t *testing.T) {
		// Given a leaf subagent at the bottom of orchestration has no
		// parent to bubble to,
		// When admin uses leaf-auto-deny,
		// Then max_bubble_depth=0 means every Confirm-tier call is
		// auto-denied — defensive default.
		assert.Contains(t, SeedExpectedBMPDTemplateSlugs, "leaf-auto-deny")
	})

	t.Run("Scenario_SingleHopForRoutineTwoTier", func(t *testing.T) {
		// Given the routine 2-tier orchestration (parent + worker),
		// When admin uses single-hop,
		// Then max_bubble_depth=1 and no admin review required.
		assert.Contains(t, SeedExpectedBMPDTemplateSlugs, "single-hop")
		set := map[string]bool{}
		for _, s := range SeedAdminReviewBMPDTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["single-hop"])
	})

	t.Run("Scenario_TwoHopRoutineForThreeTier", func(t *testing.T) {
		// Given a controller→orchestrator→worker chain,
		// When admin uses two-hop-routine,
		// Then max_bubble_depth=2 + history recorded for audit.
		assert.Contains(t, SeedExpectedBMPDTemplateSlugs, "two-hop-routine")
	})

	t.Run("Scenario_ThreeHopOrchestrationForMultiStage", func(t *testing.T) {
		// Given multi-stage agentic pipelines (4 levels),
		// When admin uses three-hop-orchestration,
		// Then max_bubble_depth=3, admin review required (deep chains
		// can mask responsibility).
		assert.Contains(t, SeedExpectedBMPDTemplateSlugs, "three-hop-orchestration")
	})

	t.Run("Scenario_FiveHopResearchForCuratorNetworks", func(t *testing.T) {
		// Given research/exploration agents flow through curator network,
		// When admin uses five-hop-research,
		// Then max_bubble_depth=5 (deepest allowed); admin review +
		// SUB-005 recursion-safety must be combined.
		assert.Contains(t, SeedExpectedBMPDTemplateSlugs, "five-hop-research")
	})

	t.Run("Scenario_DepthMonotonicallyIncreasesAcrossPresets", func(t *testing.T) {
		// Given the depth ladder is 0/1/2/3/5,
		// When admin compares templates,
		// Then max_bubble_depth strictly increases — depth ladder is
		// monotonic. Validated DB-real in integration test.
		assert.Equal(t, 5, SeedExpectedBMPDTemplateRowCount)
	})

	t.Run("Scenario_ZeroDepthSkipsParentResolver", func(t *testing.T) {
		// Given leaf-auto-deny has max_bubble_depth=0,
		// And PERM-003b NewBubbleModeEvaluator(_, nil, 0) is valid,
		// When admin compares requires_parent_resolver,
		// Then leaf-auto-deny → false; everything else → true.
		// Validated DB-real in integration test.
		assert.Contains(t, SeedExpectedBMPDTemplateSlugs, "leaf-auto-deny")
	})

	t.Run("Scenario_DeepBubblingRequiresHistoryCapture", func(t *testing.T) {
		// Given chains of 2+ bubbles obscure who decided what,
		// When admin compares records_chain_history,
		// Then depth >= 2 → history capture required. Validated DB-real.
		assert.Equal(t, 5, SeedExpectedBMPDTemplateRowCount)
	})

	t.Run("Scenario_AdminReviewGatesNonRoutineDepth", func(t *testing.T) {
		// Given single-hop is the routine 2-tier baseline,
		// And other depths materially change escalation surface,
		// When admin compares admin-review subset,
		// Then 4 of 5 templates gate change.
		assert.Equal(t, 4, len(SeedAdminReviewBMPDTemplateSlugs))
	})

	t.Run("Scenario_SafetyPostureLadderRepresented", func(t *testing.T) {
		// Given posture is a stance ladder (strict..balanced..permissive),
		// When seed templates ship,
		// Then all 3 postures are represented.
		assert.Equal(t, 3, len(SeedExpectedBMPDTemplateSafetyPostures))
	})
}
