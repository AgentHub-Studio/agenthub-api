package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedPluginSettingsFieldType_ExpectedRowCount(t *testing.T) {
	assert.Equal(t, 5, SeedExpectedPluginSettingsFieldTypeRowCount)
}

func TestSeedPluginSettingsFieldType_SlugCountMatchesRowCount(t *testing.T) {
	assert.Equal(t, SeedExpectedPluginSettingsFieldTypeRowCount, len(SeedExpectedPluginSettingsFieldTypeSlugs))
}

func TestSeedPluginSettingsFieldType_SlugsContainString(t *testing.T) {
	assert.Contains(t, SeedExpectedPluginSettingsFieldTypeSlugs, "string")
}

func TestSeedPluginSettingsFieldType_SlugsContainBoolean(t *testing.T) {
	assert.Contains(t, SeedExpectedPluginSettingsFieldTypeSlugs, "boolean")
}

func TestSeedPluginSettingsFieldType_SlugsContainNumber(t *testing.T) {
	assert.Contains(t, SeedExpectedPluginSettingsFieldTypeSlugs, "number")
}

func TestSeedPluginSettingsFieldType_SlugsContainSelect(t *testing.T) {
	assert.Contains(t, SeedExpectedPluginSettingsFieldTypeSlugs, "select")
}

func TestSeedPluginSettingsFieldType_SlugsContainSecret(t *testing.T) {
	assert.Contains(t, SeedExpectedPluginSettingsFieldTypeSlugs, "secret")
}

func TestSeedPluginSettingsFieldType_OnlySecretIsMaskable(t *testing.T) {
	assert.Equal(t, 1, len(SeedPluginSettingsMaskableFieldTypeSlugs))
	assert.Contains(t, SeedPluginSettingsMaskableFieldTypeSlugs, "secret")
}

func TestSeedPluginSettingsFieldType_OnlySelectRequiresOptions(t *testing.T) {
	assert.Equal(t, 1, len(SeedPluginSettingsOptionsRequiredFieldTypeSlugs))
	assert.Contains(t, SeedPluginSettingsOptionsRequiredFieldTypeSlugs, "select")
}

func TestSeedPluginSettingsFieldType_OnlyNumberIsNumeric(t *testing.T) {
	assert.Equal(t, 1, len(SeedPluginSettingsNumericFieldTypeSlugs))
	assert.Contains(t, SeedPluginSettingsNumericFieldTypeSlugs, "number")
}

func TestSeedPluginSettingsFieldType_MaskableSlugsAreSubsetOfAll(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedPluginSettingsFieldTypeSlugs {
		all[s] = true
	}
	for _, s := range SeedPluginSettingsMaskableFieldTypeSlugs {
		assert.True(t, all[s], "maskable slug %q not in canonical list", s)
	}
}
