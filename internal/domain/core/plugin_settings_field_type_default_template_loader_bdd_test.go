package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for the §6.1 plugin settings field type seed in ah_core.

func TestBDD_AhCorePluginSettingsFieldTypeSeed(t *testing.T) {
	t.Run("Scenario_FiveFieldTypesCoverAllSettingsInputPatterns", func(t *testing.T) {
		// Given §6.1 "settings" component type requires structured field declarations
		// When the canonical field type list is checked
		// Then five types cover all input patterns: string/boolean/number/select/secret
		assert.Equal(t, 5, SeedExpectedPluginSettingsFieldTypeRowCount)
		assert.Contains(t, SeedExpectedPluginSettingsFieldTypeSlugs, "string")
		assert.Contains(t, SeedExpectedPluginSettingsFieldTypeSlugs, "secret")
	})

	t.Run("Scenario_SecretIsTheOnlyMaskedFieldType", func(t *testing.T) {
		// Given API keys and tokens must be hidden in the UI and excluded from audit logs
		// When the maskable field types are listed
		// Then only secret has is_maskable=true
		assert.Equal(t, 1, len(SeedPluginSettingsMaskableFieldTypeSlugs))
		assert.Equal(t, "secret", SeedPluginSettingsMaskableFieldTypeSlugs[0])
	})

	t.Run("Scenario_SelectRequiresOptionsAndNothingElseDoes", func(t *testing.T) {
		// Given a dropdown cannot render without declared choices
		// When requires_options is checked across all field types
		// Then only select requires an options array
		assert.Equal(t, 1, len(SeedPluginSettingsOptionsRequiredFieldTypeSlugs))
		assert.Equal(t, "select", SeedPluginSettingsOptionsRequiredFieldTypeSlugs[0])
	})

	t.Run("Scenario_NumberIsTheOnlyNumericFieldType", func(t *testing.T) {
		// Given numeric min/max validation applies only to number fields
		// When numeric field types are listed
		// Then only number has is_numeric=true
		assert.Equal(t, 1, len(SeedPluginSettingsNumericFieldTypeSlugs))
		assert.Equal(t, "number", SeedPluginSettingsNumericFieldTypeSlugs[0])
	})

	t.Run("Scenario_AllSubsetSlugsAreInCanonicalList", func(t *testing.T) {
		// Given constants must be internally consistent
		// When maskable/options-required/numeric slugs are checked against the full list
		// Then every slug exists in SeedExpectedPluginSettingsFieldTypeSlugs
		all := map[string]bool{}
		for _, s := range SeedExpectedPluginSettingsFieldTypeSlugs {
			all[s] = true
		}
		for _, s := range SeedPluginSettingsMaskableFieldTypeSlugs {
			assert.True(t, all[s])
		}
		for _, s := range SeedPluginSettingsOptionsRequiredFieldTypeSlugs {
			assert.True(t, all[s])
		}
		for _, s := range SeedPluginSettingsNumericFieldTypeSlugs {
			assert.True(t, all[s])
		}
	})
}
