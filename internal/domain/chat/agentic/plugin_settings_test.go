package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginSettingsFieldType_FiveTypes(t *testing.T) {
	assert.Equal(t, 5, len(AllPluginSettingsFieldTypes))
}

func TestPluginSettingsFieldTypeRegistry_Profile_String(t *testing.T) {
	reg := NewPluginSettingsFieldTypeRegistry()
	p, ok := reg.Profile(PluginSettingsFieldString)
	assert.True(t, ok)
	assert.False(t, p.IsMaskable)
	assert.False(t, p.RequiresOptions)
	assert.False(t, p.IsNumeric)
	assert.Equal(t, "text_input", p.UIWidget)
}

func TestPluginSettingsFieldTypeRegistry_Profile_Boolean(t *testing.T) {
	reg := NewPluginSettingsFieldTypeRegistry()
	p, ok := reg.Profile(PluginSettingsFieldBoolean)
	assert.True(t, ok)
	assert.Equal(t, "toggle", p.UIWidget)
}

func TestPluginSettingsFieldTypeRegistry_Profile_Number(t *testing.T) {
	reg := NewPluginSettingsFieldTypeRegistry()
	p, ok := reg.Profile(PluginSettingsFieldNumber)
	assert.True(t, ok)
	assert.True(t, p.IsNumeric)
	assert.Equal(t, "number_input", p.UIWidget)
}

func TestPluginSettingsFieldTypeRegistry_Profile_Select(t *testing.T) {
	reg := NewPluginSettingsFieldTypeRegistry()
	p, ok := reg.Profile(PluginSettingsFieldSelect)
	assert.True(t, ok)
	assert.True(t, p.RequiresOptions, "select must declare an options list")
	assert.Equal(t, "dropdown", p.UIWidget)
}

func TestPluginSettingsFieldTypeRegistry_Profile_Secret(t *testing.T) {
	reg := NewPluginSettingsFieldTypeRegistry()
	p, ok := reg.Profile(PluginSettingsFieldSecret)
	assert.True(t, ok)
	assert.True(t, p.IsMaskable, "secret values must be masked in UI and logs")
	assert.Equal(t, "password_input", p.UIWidget)
}

func TestPluginSettingsFieldTypeRegistry_Profile_Unknown(t *testing.T) {
	reg := NewPluginSettingsFieldTypeRegistry()
	_, ok := reg.Profile("not_a_type")
	assert.False(t, ok)
}

func TestPluginSettingsFieldTypeRegistry_AllFieldTypes_IsDefensiveCopy(t *testing.T) {
	reg := NewPluginSettingsFieldTypeRegistry()
	all := reg.AllFieldTypes()
	all[0] = "mutated"
	assert.Equal(t, PluginSettingsFieldString, reg.AllFieldTypes()[0])
}

func TestPluginSettingsFieldTypeRegistry_MaskableFieldTypes_OnlySecret(t *testing.T) {
	reg := NewPluginSettingsFieldTypeRegistry()
	m := reg.MaskableFieldTypes()
	assert.Equal(t, 1, len(m))
	assert.Equal(t, PluginSettingsFieldSecret, m[0])
}

func TestPluginSettingsFieldTypeRegistry_FieldTypesRequiringOptions_OnlySelect(t *testing.T) {
	reg := NewPluginSettingsFieldTypeRegistry()
	r := reg.FieldTypesRequiringOptions()
	assert.Equal(t, 1, len(r))
	assert.Equal(t, PluginSettingsFieldSelect, r[0])
}

func TestPluginSettingsFieldTypeRegistry_IsValidFieldType(t *testing.T) {
	reg := NewPluginSettingsFieldTypeRegistry()
	assert.True(t, reg.IsValidFieldType(PluginSettingsFieldString))
	assert.True(t, reg.IsValidFieldType(PluginSettingsFieldSecret))
	assert.False(t, reg.IsValidFieldType("enum"))
}

func TestExtensionComponentKind_SettingsAndUserConfigInEnum(t *testing.T) {
	assert.True(t, IsValidExtensionComponentKind(ExtensionComponentSettings))
	assert.True(t, IsValidExtensionComponentKind(ExtensionComponentUserConfiguration))
}

func TestExtensionComponentKind_TenKindsTotal(t *testing.T) {
	assert.Equal(t, 10, len(AllExtensionComponentKinds()))
}

func TestExtensionComponentKind_LSPServersNotApplicableWeb(t *testing.T) {
	assert.False(t, IsValidExtensionComponentKind(ExtensionComponentKind("lsp_servers")))
}

func TestExtensionComponentKind_ChannelsNotApplicableWeb(t *testing.T) {
	assert.False(t, IsValidExtensionComponentKind(ExtensionComponentKind("channels")))
}
