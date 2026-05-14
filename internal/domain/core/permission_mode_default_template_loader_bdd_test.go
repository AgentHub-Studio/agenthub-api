package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for permission_mode_template seed.
// Maps §11.3 "Safety vs. autonomy" gradient to AgentHub web agent configuration.

func TestBDD_AhCorePermissionModeSeed(t *testing.T) {
	t.Run("Scenario_FiveModesFormACompleteGradient", func(t *testing.T) {
		// Given the §11.3 five-mode permission spectrum
		// When the platform loads permission mode presets
		// Then exactly 5 modes are available covering plan through bypassPermissions
		assert.Equal(t, 5, SeedExpectedPermissionModeRowCount)
		assert.Equal(t, 5, len(SeedExpectedPermissionModeSlugs))
	})

	t.Run("Scenario_DefaultModeIsUsedWhenNoModeConfigured", func(t *testing.T) {
		// Given an agent with no explicit permission configuration
		// Then the default mode is used (deny-first, confirmation required)
		assert.Equal(t, "default", SeedPermissionModeDefaultSlug)
		found := false
		for _, s := range SeedExpectedPermissionModeSlugs {
			if s == SeedPermissionModeDefaultSlug {
				found = true
			}
		}
		assert.True(t, found, "default must be in canonical slug list")
	})

	t.Run("Scenario_PlanModeIsSafestWithMaxOversight", func(t *testing.T) {
		// Given a regulated tenant requiring human approval for all actions
		// When the plan mode is selected
		// Then it is the first in the gradient (index 0, safety_score 100)
		assert.Equal(t, "plan", SeedPermissionModeSafestSlug)
		assert.Equal(t, "plan", SeedExpectedPermissionModeSlugs[0],
			"plan must be first in the gradient (safest)")
	})

	t.Run("Scenario_BypassPermissionsIsMostAutonomous", func(t *testing.T) {
		// Given a trusted automation pipeline with known scope
		// When bypassPermissions mode is configured
		// Then it is the last in the gradient (index 4, safety_score 0)
		assert.Equal(t, "bypassPermissions", SeedPermissionModeBypassSlug)
		assert.Equal(t, "bypassPermissions", SeedExpectedPermissionModeSlugs[4],
			"bypassPermissions must be last in the gradient (most autonomous)")
	})

	t.Run("Scenario_AllSlugsMustMatchRegexPattern", func(t *testing.T) {
		// Given naming conventions allow both kebab-case and camelCase
		// Then every slug matches the regex
		for _, s := range SeedExpectedPermissionModeSlugs {
			assert.True(t, SeedPermissionModeSlugRE.MatchString(s),
				"slug %q violates allowed pattern", s)
		}
	})
}
