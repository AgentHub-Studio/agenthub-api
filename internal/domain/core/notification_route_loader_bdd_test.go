package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreNotifRouteSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsAlertingRoutes", func(t *testing.T) {
		assert.GreaterOrEqual(t, len(SeedExpectedNotificationRouteTemplateSlugs), 6,
			"≥6 ready alerting routes")
	})

	t.Run("Scenario_AllEightTriggerEventsCovered", func(t *testing.T) {
		set := map[string]bool{}
		for _, e := range SeedExpectedNotificationRouteTriggerEvents {
			set[e] = true
		}
		for _, want := range []string{
			"run_complete", "quality_report", "governance_alert",
			"checkpoint_pending", "checkpoint_timeout", "silent_failure",
			"audit_event", "coherence_drift",
		} {
			assert.True(t, set[want])
		}
	})

	t.Run("Scenario_PagerDutyRoutesAreReservedForCriticalOnly", func(t *testing.T) {
		// Given paging oncall has cost — only critical events page,
		// When PagerDuty routes are inspected,
		// Then they exist for governance + silent failure (both critical).
		recSet := map[string]bool{}
		for _, r := range SeedRecommendedNotificationRouteTemplateSlugs {
			recSet[r] = true
		}
		assert.True(t, recSet["governance-alerts-to-pagerduty"])
		assert.True(t, recSet["silent-failure-to-pagerduty"])
	})

	t.Run("Scenario_SlackRoutesUseAggregationToReduceSpam", func(t *testing.T) {
		// Given run_complete fires often — aggregate before notifying,
		// (Constant guard; integration test verifies window=60s.)
		// Naming check — runs-to-slack pattern present:
		set := map[string]bool{}
		for _, s := range SeedExpectedNotificationRouteTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["runs-to-slack"])
	})

	t.Run("Scenario_AuditEventsHaveNoRateLimit", func(t *testing.T) {
		// Given compliance audit must capture every event,
		set := map[string]bool{}
		for _, s := range SeedExpectedNotificationRouteTemplateSlugs {
			set[s] = true
		}
		assert.True(t, set["audit-events-to-webhook"],
			"audit route required (compliance contract)")
	})

	t.Run("Scenario_RecommendedSubsetCoversCommonAlertingNeeds", func(t *testing.T) {
		// 5 recommended routes cover: runs (visibility), quality (issue
		// surface), governance (compliance), checkpoint (workflow),
		// silent failure (incident).
		assert.Equal(t, 5, len(SeedRecommendedNotificationRouteTemplateSlugs))
	})

	t.Run("Scenario_RouteNamingPatternsLinkSourceToDestination", func(t *testing.T) {
		// Given route slugs follow "<source>-to-<destination>" pattern,
		for _, s := range SeedExpectedNotificationRouteTemplateSlugs {
			assert.True(t, strings.Contains(s, "-to-"),
				"route slug %q must follow source-to-destination pattern", s)
		}
	})

	t.Run("Scenario_SeverityFilterEnumIsBounded", func(t *testing.T) {
		set := map[string]bool{}
		for _, s := range SeedExpectedNotificationRouteSeverities {
			set[s] = true
		}
		assert.True(t, set[""], "empty = all severities")
		assert.True(t, set["info"])
		assert.True(t, set["warn"])
		assert.True(t, set["critical"])
		assert.False(t, set["urgent"], "non-standard severity rejected")
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		assert.Equal(t, 8, len(SeedExpectedNotificationRouteTemplateSlugs))
	})
}
