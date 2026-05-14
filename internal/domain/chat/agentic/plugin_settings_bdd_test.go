package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for §6.1 plugin settings component type and field type schema.

func TestBDD_PluginSettingsComponentKind(t *testing.T) {
	reg := NewPluginSettingsFieldTypeRegistry()

	t.Run("Scenario_SettingsAndUserConfigurationCompleteThe10TypePluginManifestSchema", func(t *testing.T) {
		// Given §6.1 PluginManifestSchema accepts ten component types
		// When web-applicable kinds are enumerated (LSP/channels excluded)
		// Then 10 kinds exist including settings and user_configuration
		assert.Equal(t, 10, len(AllExtensionComponentKinds()))
		assert.True(t, IsValidExtensionComponentKind(ExtensionComponentSettings))
		assert.True(t, IsValidExtensionComponentKind(ExtensionComponentUserConfiguration))
	})

	t.Run("Scenario_SecretFieldTypeIsMaskedInUIAndLogs", func(t *testing.T) {
		// Given API keys and tokens must never appear in logs or audit exports
		// When the secret field type profile is inspected
		// Then IsMaskable=true and UIWidget=password_input
		p, ok := reg.Profile(PluginSettingsFieldSecret)
		assert.True(t, ok)
		assert.True(t, p.IsMaskable)
		assert.Equal(t, "password_input", p.UIWidget)
	})

	t.Run("Scenario_SelectFieldTypeRequiresOptionsList", func(t *testing.T) {
		// Given a dropdown cannot render without declared choices
		// When the select field type profile is inspected
		// Then RequiresOptions=true
		p, ok := reg.Profile(PluginSettingsFieldSelect)
		assert.True(t, ok)
		assert.True(t, p.RequiresOptions)
	})

	t.Run("Scenario_OnlyOneFieldTypeMasksValues", func(t *testing.T) {
		// Given only secret values need masking (string, boolean, number, select are safe to display)
		// When maskable types are listed
		// Then only secret appears
		maskable := reg.MaskableFieldTypes()
		assert.Equal(t, 1, len(maskable))
		assert.Equal(t, PluginSettingsFieldSecret, maskable[0])
	})

	t.Run("Scenario_LSPServersAndChannelsRemainNotApplicableWeb", func(t *testing.T) {
		// Given LSP servers are IDE-specific and channels are CLI-terminal-specific
		// When these kinds are validated against the web-applicable enum
		// Then they are rejected
		assert.False(t, IsValidExtensionComponentKind(ExtensionComponentKind("lsp_servers")))
		assert.False(t, IsValidExtensionComponentKind(ExtensionComponentKind("channels")))
	})
}
