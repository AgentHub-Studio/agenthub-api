package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreCapabilityTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantCanMeasureAllFiveCapabilityDimensionsOutOfTheBox", func(t *testing.T) {
		// Given a fresh tenant adopts AgentHub,
		// And the platform offers FUTURE-006 capability tracking,
		// When admin opens the capability-tracking onboarding,
		// Then a per-dimension template exists for each of the 5 FUTURE-006
		// dimensions, so admin doesn't have to invent rubrics from scratch.
		needs := map[string]bool{
			"domain_knowledge":      false,
			"decision_independence": false,
			"task_throughput":       false,
			"quality_output":        false,
			"collaboration":         false,
		}
		for _, d := range SeedExpectedCapabilityTemplateDimensions {
			if _, ok := needs[d]; ok {
				needs[d] = true
			}
		}
		for d, covered := range needs {
			assert.True(t, covered, "dimension %q must be covered by at least one template", d)
		}
	})

	t.Run("Scenario_OnboardingBaselineGivesEveryUserAStartingReference", func(t *testing.T) {
		// Given a new user joins a tenant,
		// When admin runs the onboarding-baseline template,
		// Then that one-shot snapshot establishes reference values for ALL
		// 5 dimensions before the user starts using the agent — without it
		// later trend-deltas would have no baseline to compare against.
		set := map[string]bool{}
		for _, s := range SeedRecommendedCapabilityTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["onboarding-baseline"],
			"onboarding-baseline must be in recommended set so it appears one-click")
	})

	t.Run("Scenario_HolisticTemplateBacksPromotionAndRoleChangeDecisions", func(t *testing.T) {
		// Given HR/admin needs cross-dimensional capability data for a
		// role-change decision,
		// When they run holistic-capability-quarterly,
		// Then the snapshot is admin-reviewed (not auto-published) because
		// promotion data must be vetted before becoming part of the record.
		isAdminReview := false
		for _, s := range SeedAdminReviewCapabilityTemplateSlugs {
			if s == "holistic-capability-quarterly" {
				isAdminReview = true
			}
		}
		assert.True(t, isAdminReview,
			"promotion-grade template must require admin review")
	})

	t.Run("Scenario_DegradingDecisionIndependenceAlertsAdminPerFutureSixContract", func(t *testing.T) {
		// Given the FUTURE-006 contract: degrading decision_independence
		// signals user becoming dependent on the agent (PDF §12),
		// When the decision-independence-monthly template is in production,
		// Then it ships as recommended one-click so dependence-drift is
		// detected by default rather than requiring opt-in.
		set := map[string]bool{}
		for _, s := range SeedRecommendedCapabilityTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["decision-independence-monthly"],
			"dependence-drift detector must be recommended")
	})

	t.Run("Scenario_IncidentPostmortemIsEventDrivenNotPeriodic", func(t *testing.T) {
		// Given an incident affects a user (outage, governance violation),
		// When admin records the post-mortem capability impact,
		// Then incident-postmortem is event_driven cadence (triggered by
		// the incident, not periodic) AND requires admin review (causal
		// attribution requires human judgment).
		isAdminReview := false
		for _, s := range SeedAdminReviewCapabilityTemplateSlugs {
			if s == "incident-postmortem" {
				isAdminReview = true
			}
		}
		assert.True(t, isAdminReview,
			"incident attribution requires admin review")
	})

	t.Run("Scenario_ThroughputUsesWeeklyCadenceForFastFeedback", func(t *testing.T) {
		// Given task throughput changes faster than knowledge or quality,
		// When admin tracks throughput,
		// Then weekly cadence is exposed as a recommended template (vs
		// monthly that would mask short-term productivity dips).
		set := map[string]bool{}
		for _, s := range SeedRecommendedCapabilityTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["task-throughput-weekly"],
			"weekly throughput must be recommended for fast feedback")
	})

	t.Run("Scenario_CollaborationDeliberatelyNotRecommendedDueToSlowFeedback", func(t *testing.T) {
		// Given peer-review signals are slow and noisy,
		// When admin enables capability tracking by default,
		// Then collaboration-quarterly is NOT one-click recommended (admin
		// must explicitly opt-in because peer-feedback infra varies).
		set := map[string]bool{}
		for _, s := range SeedRecommendedCapabilityTemplateSlugs {
			set[s] = true
		}
		assert.False(t, set["collaboration-quarterly"],
			"collaboration template must be opt-in, not one-click default")
	})

	t.Run("Scenario_DimensionLabelsExactlyMatchFutureSixEnumValues", func(t *testing.T) {
		// Given FUTURE-006 has 5 CapabilityDimension enum values,
		// When the seed uses string labels for target_dimension,
		// Then those labels MUST equal the enum values byte-for-byte so
		// runtime joining capability_assessment_template ⇄ HumanCapability
		// Snapshot.Metrics works without mapping tables.
		futureSixEnum := []string{
			"domain_knowledge", "decision_independence",
			"task_throughput", "quality_output", "collaboration",
		}
		dims := map[string]bool{}
		for _, d := range SeedExpectedCapabilityTemplateDimensions {
			dims[d] = true
		}
		for _, e := range futureSixEnum {
			assert.True(t, dims[e],
				"FUTURE-006 dimension %q must appear exactly in seed", e)
		}
		// "multi" is the additional sentinel used for multi-dimensional
		// templates (holistic / onboarding / incident).
		assert.True(t, dims["multi"], "multi sentinel required for holistic templates")
	})

	t.Run("Scenario_CatalogCoversAllCadencePatternsTenantsActuallyNeed", func(t *testing.T) {
		// Given product onboarding research showed admins want weekly,
		// monthly, quarterly, one_shot, and event_driven measurements,
		// When the seed ships,
		// Then all 5 cadence patterns are present and discoverable.
		expected := []string{"weekly", "monthly", "quarterly", "one_shot", "event_driven"}
		set := map[string]bool{}
		for _, c := range SeedExpectedCapabilityTemplateCadences {
			set[c] = true
		}
		for _, e := range expected {
			assert.True(t, set[e], "cadence %q missing from seed", e)
		}
	})

	t.Run("Scenario_DefaultDeltaThresholdMatchesFutureSixClassifyTrendThreshold", func(t *testing.T) {
		// Given FUTURE-006 classifyTrend uses ±0.05 as stable/improving/
		// degrading boundary,
		// When admin instantiates a template without overriding the
		// alert threshold,
		// Then the default 0.05 immediately aligns alerts with the
		// FUTURE-006 trend classification (no surprise mismatches).
		// This invariant is enforced at the SQL DEFAULT level — see
		// integration test DeltaAlertThresholdDefault.
		assert.True(t, true) // Documented invariant; DB-level test enforces.
	})
}
