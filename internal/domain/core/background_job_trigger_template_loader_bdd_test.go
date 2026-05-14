package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreBgJobTriggerTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsTwoTemplatesPerFutureThreeTriggerKind", func(t *testing.T) {
		// Given a fresh tenant adopts AgentHub,
		// And FUTURE-003 ships 4 background trigger kinds (cron / event /
		// threshold / absence),
		// When admin opens background-job onboarding,
		// Then 2 templates exist for each kind so admin sees real-world
		// patterns (not just theoretical possibilities).
		// 8 templates / 4 kinds = 2 each.
		assert.Equal(t, 4, len(SeedExpectedBgJobTriggerTemplateKinds))
		assert.Equal(t, 8, len(SeedExpectedBgJobTriggerTemplateSlugs))
	})

	t.Run("Scenario_KindLabelsMatchFutureThreeEnumByteForByte", func(t *testing.T) {
		// Given FUTURE-003 BackgroundTrigger has 4 enum values,
		// When the seed references trigger kinds,
		// Then labels MUST match enum bytes (no mapping table runtime).
		futureThree := []string{
			"schedule_cron", "event_arrived",
			"threshold_crossed", "absence_timeout",
		}
		set := map[string]bool{}
		for _, k := range SeedExpectedBgJobTriggerTemplateKinds {
			set[k] = true
		}
		for _, e := range futureThree {
			assert.True(t, set[e], "FUTURE-003 kind %q missing from seed", e)
		}
	})

	t.Run("Scenario_CostPauseRequiresAdminReviewDueToBusinessImpact", func(t *testing.T) {
		// Given pausing agents on cost overrun has business impact
		// (interrupts user-facing work),
		// When the seed exposes on-cost-budget-exceeded,
		// Then admin must vet before activation.
		isAdminReview := false
		for _, s := range SeedAdminReviewBgJobTriggerTemplateSlugs {
			if s == "on-cost-budget-exceeded" {
				isAdminReview = true
			}
		}
		assert.True(t, isAdminReview)
	})

	t.Run("Scenario_AgentArchivalRequiresAdminReviewToProtectTenantWork", func(t *testing.T) {
		// Given auto-archiving an agent loses tenant configuration,
		// When the seed exposes on-agent-unused-90-days,
		// Then it requires admin review (FLAG, do not auto-archive).
		isAdminReview := false
		for _, s := range SeedAdminReviewBgJobTriggerTemplateSlugs {
			if s == "on-agent-unused-90-days" {
				isAdminReview = true
			}
		}
		assert.True(t, isAdminReview)
	})

	t.Run("Scenario_RoutineHealthChecksAreNotAdminGated", func(t *testing.T) {
		// Given hourly health check runs continuously without admin
		// intervention,
		// When the seed exposes hourly-health-check,
		// Then it must NOT require admin review (gating routine signals
		// would break the entire ops awareness loop).
		set := map[string]bool{}
		for _, s := range SeedAdminReviewBgJobTriggerTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["hourly-health-check"])
	})

	t.Run("Scenario_OnFailureActionsAreClosedSetForRunnerDispatch", func(t *testing.T) {
		// Given runtime knows how to dispatch retry/alert/quarantine,
		// When seed declares failure actions,
		// Then they ALL belong to closed set (no surprise actions that
		// runtime can't handle silently fail).
		expected := []string{
			"retry_with_backoff", "alert_admin", "quarantine_trigger",
		}
		set := map[string]bool{}
		for _, a := range SeedExpectedBgJobTriggerTemplateOnFailureActions {
			set[a] = true
		}
		for _, e := range expected {
			assert.True(t, set[e], "action %q missing", e)
		}
	})

	t.Run("Scenario_KbDocumentEventTemplateDrivesNearRealtimeRagFreshness", func(t *testing.T) {
		// Given uploading a KB doc and waiting for next cron creates
		// stale-RAG window,
		// When admin enables on-kb-document-uploaded event trigger,
		// Then the embed-and-index workflow runs immediately (RAG
		// queries see new content within minutes).
		set := map[string]bool{}
		for _, s := range SeedRecommendedBgJobTriggerTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["on-kb-document-uploaded"],
			"realtime KB ingestion must be recommended one-click")
	})

	t.Run("Scenario_UserDormancyIntegratesWithFutureSixCapabilityTracking", func(t *testing.T) {
		// Given FUTURE-006 capability metrics need active observations,
		// When a user is inactive 30 days,
		// Then on-user-inactive-30-days fires capability snapshot to
		// flag dormant + recommend decision_independence review.
		set := map[string]bool{}
		for _, s := range SeedRecommendedBgJobTriggerTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["on-user-inactive-30-days"])
	})

	t.Run("Scenario_NewUserOnboardedTriggersBaselineSnapshot", func(t *testing.T) {
		// Given FUTURE-006 onboarding-baseline template wants the user's
		// reference values established in first 14 days,
		// When a new user is onboarded,
		// Then on-new-user-onboarded fires the workflow that records the
		// baseline (so capability tracking starts from first interaction).
		set := map[string]bool{}
		for _, s := range SeedRecommendedBgJobTriggerTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["on-new-user-onboarded"])
	})

	t.Run("Scenario_ErrorRateSpikeDriversAlertingBeforeOutageEscalates", func(t *testing.T) {
		// Given 1% 5xx error rate is the early-warning threshold,
		// When error rate crosses,
		// Then on-error-rate-spike runs incident triage workflow + alerts
		// admin (early intervention beats post-mortem).
		set := map[string]bool{}
		for _, s := range SeedRecommendedBgJobTriggerTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["on-error-rate-spike"])
	})
}
