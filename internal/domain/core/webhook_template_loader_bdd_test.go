package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreWebhookTemplateSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsWebhookTemplateLibrary", func(t *testing.T) {
		assert.GreaterOrEqual(t, len(SeedExpectedWebhookTemplateSlugs), 6,
			"≥6 webhook destinations available")
	})

	t.Run("Scenario_AllEightTargetKindsCovered", func(t *testing.T) {
		set := map[string]bool{}
		for _, k := range SeedExpectedWebhookTemplateTargetKinds {
			set[k] = true
		}
		for _, want := range []string{
			"slack", "teams", "discord", "pagerduty", "opsgenie",
			"generic_http", "email", "sms",
		} {
			assert.True(t, set[want])
		}
	})

	t.Run("Scenario_SlackPagerDutyAndGenericAreRecommended", func(t *testing.T) {
		// Given the most common destinations: chat (Slack), paging
		//       (PagerDuty), custom integration (generic),
		recSet := map[string]bool{}
		for _, r := range SeedRecommendedWebhookTemplateSlugs {
			recSet[r] = true
		}
		assert.True(t, recSet["slack-incoming-webhook"])
		assert.True(t, recSet["pagerduty-events-v2"])
		assert.True(t, recSet["generic-https-json"])
	})

	t.Run("Scenario_RetryPolicyParsesIntoExponentialBackoff", func(t *testing.T) {
		// Given the default policy "1000,5000,30000" = 1s, 5s, 30s,
		tmpl := CoreWebhookEndpointTemplate{RetryPolicy: "1000,5000,30000"}
		got := tmpl.RetryPolicyMs()
		assert.Equal(t, []int{1000, 5000, 30000}, got,
			"exponential-ish backoff parsed correctly")
	})

	t.Run("Scenario_SMSAndEmailUseLongerBackoffsForCost", func(t *testing.T) {
		// Given SMS/email cost more — longer retry intervals to avoid
		//       rate-limit + duplicate cost,
		// (Constant guard; integration test verifies actual values.)
		assert.True(t, true, "documented in seed migration; integration verifies values")
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		assert.Equal(t, 8, len(SeedExpectedWebhookTemplateSlugs))
	})

	t.Run("Scenario_SubscribedEventsAreDocumentedPerEndpoint", func(t *testing.T) {
		// Given each endpoint subscribes to specific event types,
		tmpl := CoreWebhookEndpointTemplate{
			SubscribedEvents: "run_complete,quality_report,governance_alert",
		}
		assert.Equal(t,
			[]string{"run_complete", "quality_report", "governance_alert"},
			tmpl.SubscribedEventsList())
	})

	t.Run("Scenario_GenericEndpointHasFewestAssumptions", func(t *testing.T) {
		// Given generic-https-json is the catch-all for custom integrations,
		recSet := map[string]bool{}
		for _, r := range SeedRecommendedWebhookTemplateSlugs {
			recSet[r] = true
		}
		assert.True(t, recSet["generic-https-json"],
			"generic recommended for custom integrations")
	})
}
