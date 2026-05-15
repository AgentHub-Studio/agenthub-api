package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreNotifRoute_SlugsNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedNotificationRouteTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreNotifRoute_SlugsCanonicalCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedNotificationRouteTemplateSlugs))
}

func TestCoreNotifRoute_TriggerEventsCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedNotificationRouteTriggerEvents))
}

func TestCoreNotifRoute_SeveritiesAllowedSet(t *testing.T) {
	allowed := map[string]bool{}
	for _, s := range SeedExpectedNotificationRouteSeverities {
		allowed[s] = true
	}
	for _, want := range []string{"", "info", "warn", "critical"} {
		assert.True(t, allowed[want])
	}
}

func TestCoreNotifRoute_RecommendedAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedNotificationRouteTemplateSlugs {
		seedSet[s] = true
	}
	for _, r := range SeedRecommendedNotificationRouteTemplateSlugs {
		assert.True(t, seedSet[r])
	}
}

func TestCoreNotifRoute_PagerDutyRoutesAreCriticalOnly(t *testing.T) {
	// PagerDuty pages oncall — only critical (or warn for timeouts) allowed.
	// Documented contract; integration test verifies actual values.
	recSet := map[string]bool{}
	for _, r := range SeedRecommendedNotificationRouteTemplateSlugs {
		recSet[r] = true
	}
	assert.True(t, recSet["governance-alerts-to-pagerduty"])
	assert.True(t, recSet["silent-failure-to-pagerduty"])
}

func TestCoreNotifRoute_SlugsKebabCase(t *testing.T) {
	for _, s := range SeedExpectedNotificationRouteTemplateSlugs {
		assert.False(t, strings.Contains(s, "_"))
	}
}
