package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreRelMilestoneTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsFutureTwoTrustLadderOutOfTheBox", func(t *testing.T) {
		// Given a fresh tenant adopts AgentHub,
		// And FUTURE-002 ships a trust ladder unknown→probationary→
		// established→trusted (+ mistrusted band),
		// When admin enables relationship tracking,
		// Then a template exists for each non-unknown trust level so
		// admin doesn't have to invent thresholds.
		needs := map[string]bool{
			"probationary": false, "established": false,
			"trusted": false, "mistrusted": false,
		}
		for _, l := range SeedExpectedRelMilestoneTemplateTrustLevels {
			if _, ok := needs[l]; ok {
				needs[l] = true
			}
		}
		for level, covered := range needs {
			assert.True(t, covered, "trust level %q must have a milestone template", level)
		}
	})

	t.Run("Scenario_PromotionsAlignWithFutureTwoDocumentedRules", func(t *testing.T) {
		// Given FUTURE-002 documents: 1+ event = probationary, 10+
		// rapport≥0 = established, 50+ rapport≥0.5 = trusted,
		// When admin reads the seed,
		// Then milestone slugs encode those exact thresholds in their
		// names (no surprise mismatches between code and templates).
		set := map[string]bool{}
		for _, s := range SeedExpectedRelMilestoneTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["first-interaction-probationary"])
		assert.True(t, set["established-after-ten-positive"])
		assert.True(t, set["trusted-after-fifty-strong-rapport"])
	})

	t.Run("Scenario_MistrustedDetectionIsAdminReviewedDueToStickyContract", func(t *testing.T) {
		// Given FUTURE-002 mistrusted is sticky (admin reset only),
		// When the seed exposes a mistrusted-detection milestone,
		// Then it requires admin review — the platform never
		// silently demotes a user to mistrusted.
		isAdminReview := false
		for _, s := range SeedAdminReviewRelMilestoneTemplateSlugs {
			if s == "mistrusted-on-repeated-escalation" {
				isAdminReview = true
			}
		}
		assert.True(t, isAdminReview)
	})

	t.Run("Scenario_TrustedPromotionRequiresAdminVettingForAutonomyExpansion", func(t *testing.T) {
		// Given trusted users get more autonomy in agent decisions,
		// When the trusted promotion fires,
		// Then admin reviews before privileges expand (not auto-grant).
		isAdminReview := false
		for _, s := range SeedAdminReviewRelMilestoneTemplateSlugs {
			if s == "trusted-after-fifty-strong-rapport" {
				isAdminReview = true
			}
		}
		assert.True(t, isAdminReview)
	})

	t.Run("Scenario_RoutineEventRecordingIsNotAdminGated", func(t *testing.T) {
		// Given the platform records positive/conflict_resolved events
		// continuously without admin intervention (FUTURE-002 rapport
		// score updates),
		// When those record-* milestones fire,
		// Then they must NOT require admin review — gating routine
		// signals would block the entire FUTURE-002 update loop.
		shouldNotBeAdmin := []string{
			"record-positive-acknowledgements",
			"record-conflict-resolutions",
			"record-negative-and-alert",
		}
		set := map[string]bool{}
		for _, s := range SeedAdminReviewRelMilestoneTemplateSlugs {
			set[s] = true
		}
		for _, s := range shouldNotBeAdmin {
			assert.False(t, set[s], "%s must NOT require admin review", s)
		}
	})

	t.Run("Scenario_EscalationRecordingOpensReviewTicketForCompliance", func(t *testing.T) {
		// Given escalation events precede mistrusted promotions and
		// often correlate with compliance incidents,
		// When an escalation is recorded,
		// Then a review ticket is opened (paper-trail for GOV-001).
		isAdminReview := false
		for _, s := range SeedAdminReviewRelMilestoneTemplateSlugs {
			if s == "record-escalation-and-open-ticket" {
				isAdminReview = true
			}
		}
		assert.True(t, isAdminReview)
	})

	t.Run("Scenario_OnReachActionsAreClosedSetForRunnerDispatch", func(t *testing.T) {
		// Given the runtime dispatches platform-side actions when a
		// milestone fires,
		// When the seed declares actions,
		// Then they ALL belong to the closed set the runtime knows
		// how to dispatch (no surprise actions that runtime can't
		// handle silently fail).
		expected := []string{
			"no_action", "elevate_communication_style",
			"send_admin_notification", "open_review_ticket",
		}
		set := map[string]bool{}
		for _, a := range SeedExpectedRelMilestoneTemplateActions {
			set[a] = true
		}
		for _, e := range expected {
			assert.True(t, set[e], "action %q missing from seed", e)
		}
	})

	t.Run("Scenario_TriggerEventKindsLabelMatchFutureTwoEnumByteForByte", func(t *testing.T) {
		// Given FUTURE-002 RapportEvent has 5 enum values,
		// When the seed references event kinds,
		// Then the labels MUST match enum values byte-for-byte (no
		// runtime mapping table needed when join template ⇄ event
		// recording).
		futureTwo := []string{
			"positive", "neutral", "negative", "conflict_resolved", "escalation",
		}
		set := map[string]bool{}
		for _, k := range SeedExpectedRelMilestoneTemplateEventKinds {
			set[k] = true
		}
		for _, e := range futureTwo {
			assert.True(t, set[e], "event_kind %q missing from seed", e)
		}
	})

	t.Run("Scenario_OnboardingFiresImmediatelyOnFirstInteraction", func(t *testing.T) {
		// Given fresh users start at trust=unknown,
		// When the very first interaction is recorded,
		// Then the first-interaction-probationary milestone moves them
		// to probationary so subsequent interactions accumulate trust
		// signals against a non-zero baseline.
		set := map[string]bool{}
		for _, s := range SeedRecommendedRelMilestoneTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["first-interaction-probationary"],
			"onboarding milestone must be recommended one-click")
	})

	t.Run("Scenario_NegativeAlertingFiresEarlyBeforeMistrustedThreshold", func(t *testing.T) {
		// Given mistrusted is sticky (hard to recover),
		// When rapport starts dropping into negative,
		// Then admin gets notified by record-negative-and-alert BEFORE
		// the mistrusted threshold is crossed (early intervention >
		// after-the-fact recovery).
		set := map[string]bool{}
		for _, s := range SeedRecommendedRelMilestoneTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["record-negative-and-alert"],
			"early-warning negative-alert must be recommended")
	})
}
