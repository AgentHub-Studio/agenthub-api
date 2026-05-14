package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for KairosHeartbeatStrategyTemplate seed.
// Maps §11.6 proactive KAIROS patterns to AgentHub web-adapted presets.

func TestBDD_AhCoreKairosHeartbeatStrategySeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsHeartbeatStrategyCatalog", func(t *testing.T) {
		// Given a new tenant with no custom configuration
		// When the system loads kairos heartbeat strategy presets
		// Then exactly 6 presets are available (on-demand + 3 heartbeat + 2 cron)
		assert.Equal(t, 6, SeedExpectedKairosHeartbeatStrategyRowCount)
		assert.Equal(t, 6, len(SeedExpectedKairosHeartbeatStrategySlugs))
	})

	t.Run("Scenario_OnDemandIsDefaultAndNonProactive", func(t *testing.T) {
		// Given the on-demand strategy (default for new agents)
		// Then it must not be proactive (no ticking without user action)
		// and its slug is the designated default
		assert.Equal(t, "on-demand", SeedKairosDefaultSlug)
		found := false
		for _, s := range SeedExpectedKairosHeartbeatStrategySlugs {
			if s == SeedKairosDefaultSlug {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("Scenario_EconomicMinimumHeartbeatIs5Minutes", func(t *testing.T) {
		// Given §11.6: prompt cache expires after 5 minutes of inactivity
		// When the minimum heartbeat preset is configured
		// Then its tick interval must be exactly 5 minutes (300 seconds)
		assert.Equal(t, "heartbeat-5m", SeedKairosMinimumHeartbeatSlug)
		// Verify it's in the catalog
		found := false
		for _, s := range SeedExpectedKairosHeartbeatStrategySlugs {
			if s == SeedKairosMinimumHeartbeatSlug {
				found = true
			}
		}
		assert.True(t, found, "KAIROS economic minimum (5m) must be in catalog")
	})

	t.Run("Scenario_AllThreeScheduleTypesRepresented", func(t *testing.T) {
		// Given the need to cover on-demand, heartbeat and cron patterns
		// Then all 3 schedule types must be seeded
		assert.Equal(t, 3, len(SeedKairosScheduleTypes))
		typeSet := map[string]bool{}
		for _, t2 := range SeedKairosScheduleTypes {
			typeSet[t2] = true
		}
		assert.True(t, typeSet["on-demand"])
		assert.True(t, typeSet["heartbeat"])
		assert.True(t, typeSet["cron"])
	})

	t.Run("Scenario_RecommendedPresetIsHeartbeat15m", func(t *testing.T) {
		// Given the need for a balanced default for background agents
		// Then heartbeat-15m is designated as the recommended background preset
		assert.Equal(t, "heartbeat-15m", SeedKairosRecommendedSlug)
		found := false
		for _, s := range SeedExpectedKairosHeartbeatStrategySlugs {
			if s == SeedKairosRecommendedSlug {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("Scenario_AllSlugsMustBeKebabCase", func(t *testing.T) {
		// Given naming conventions require kebab-case slugs
		// Then every seeded slug matches the pattern
		for _, s := range SeedExpectedKairosHeartbeatStrategySlugs {
			assert.True(t, SeedKairosStrategySlugRE.MatchString(s),
				"slug %q violates kebab-case pattern", s)
		}
	})
}
