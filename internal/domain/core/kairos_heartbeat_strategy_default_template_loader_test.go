package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreKairosStrategy_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedKairosHeartbeatStrategySlugs))
	assert.Equal(t, 6, SeedExpectedKairosHeartbeatStrategyRowCount)
}

func TestCoreKairosStrategy_SlugsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedKairosHeartbeatStrategySlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreKairosStrategy_AllSlugsMatchKebabRegex(t *testing.T) {
	for _, s := range SeedExpectedKairosHeartbeatStrategySlugs {
		assert.True(t, SeedKairosStrategySlugRE.MatchString(s), "slug %q must match kebab regex", s)
	}
}

func TestCoreKairosStrategy_RowCountEqualsSlugCount(t *testing.T) {
	assert.Equal(t, SeedExpectedKairosHeartbeatStrategyRowCount, len(SeedExpectedKairosHeartbeatStrategySlugs))
}

func TestCoreKairosStrategy_DefaultSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedKairosHeartbeatStrategySlugs {
		if s == SeedKairosDefaultSlug {
			found = true
		}
	}
	assert.True(t, found, "on-demand must be in slug list as the default")
}

func TestCoreKairosStrategy_MinimumHeartbeatSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedKairosHeartbeatStrategySlugs {
		if s == SeedKairosMinimumHeartbeatSlug {
			found = true
		}
	}
	assert.True(t, found, "heartbeat-5m (KAIROS economic minimum) must be in slug list")
}

func TestCoreKairosStrategy_RecommendedSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedKairosHeartbeatStrategySlugs {
		if s == SeedKairosRecommendedSlug {
			found = true
		}
	}
	assert.True(t, found, "heartbeat-15m must be in slug list as recommended")
}

func TestCoreKairosStrategy_ScheduleTypesCount(t *testing.T) {
	assert.Equal(t, 3, len(SeedKairosScheduleTypes),
		"3 schedule types: on-demand, heartbeat, cron")
}

func TestCoreKairosStrategy_ScheduleTypesUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedKairosScheduleTypes {
		assert.False(t, seen[s], "duplicate type %q", s)
		seen[s] = true
	}
}

func TestCoreKairosStrategy_ScheduleTypesCoverAllVariants(t *testing.T) {
	typeSet := map[string]bool{}
	for _, t2 := range SeedKairosScheduleTypes {
		typeSet[t2] = true
	}
	for _, want := range []string{"on-demand", "heartbeat", "cron"} {
		assert.True(t, typeSet[want], "type %q must be in SeedKairosScheduleTypes", want)
	}
}

func TestCoreKairosStrategy_MinimumHeartbeatInterval_Is5Minutes(t *testing.T) {
	// §11.6: 5 minutes is the economic minimum (prompt cache TTL).
	assert.Equal(t, "heartbeat-5m", SeedKairosMinimumHeartbeatSlug)
}
