package agentic

// PluginSettingsFieldType identifies the data type of a settings or user_configuration
// field declared in a plugin manifest (§6.1 "settings" component type).
// Each type drives both validation and the form widget rendered in the AgentHub UI.
type PluginSettingsFieldType string

const (
	// PluginSettingsFieldString — free-text input; stored as a string.
	PluginSettingsFieldString PluginSettingsFieldType = "string"
	// PluginSettingsFieldBoolean — toggle; stored as true/false.
	PluginSettingsFieldBoolean PluginSettingsFieldType = "boolean"
	// PluginSettingsFieldNumber — numeric input with optional min/max.
	PluginSettingsFieldNumber PluginSettingsFieldType = "number"
	// PluginSettingsFieldSelect — single-choice from a declared option list.
	PluginSettingsFieldSelect PluginSettingsFieldType = "select"
	// PluginSettingsFieldSecret — like string but masked in the UI and
	// excluded from logs/audit exports (API keys, tokens).
	PluginSettingsFieldSecret PluginSettingsFieldType = "secret"
)

// PluginSettingsFieldTypeProfile holds UI and validation metadata for one field type.
type PluginSettingsFieldTypeProfile struct {
	FieldType      PluginSettingsFieldType
	IsMaskable     bool // true = value should be masked in UI/logs (only secret)
	RequiresOptions bool // true = field must declare an options list (only select)
	IsNumeric      bool // true = value must parse as a number (only number)
	UIWidget       string // hint for frontend: "text_input"|"toggle"|"number_input"|"dropdown"|"password_input"
}

var pluginSettingsFieldTypeProfiles = map[PluginSettingsFieldType]PluginSettingsFieldTypeProfile{
	PluginSettingsFieldString: {
		FieldType: PluginSettingsFieldString,
		IsMaskable: false, RequiresOptions: false, IsNumeric: false,
		UIWidget: "text_input",
	},
	PluginSettingsFieldBoolean: {
		FieldType: PluginSettingsFieldBoolean,
		IsMaskable: false, RequiresOptions: false, IsNumeric: false,
		UIWidget: "toggle",
	},
	PluginSettingsFieldNumber: {
		FieldType: PluginSettingsFieldNumber,
		IsMaskable: false, RequiresOptions: false, IsNumeric: true,
		UIWidget: "number_input",
	},
	PluginSettingsFieldSelect: {
		FieldType: PluginSettingsFieldSelect,
		IsMaskable: false, RequiresOptions: true, IsNumeric: false,
		UIWidget: "dropdown",
	},
	PluginSettingsFieldSecret: {
		FieldType: PluginSettingsFieldSecret,
		IsMaskable: true, RequiresOptions: false, IsNumeric: false,
		UIWidget: "password_input",
	},
}

// AllPluginSettingsFieldTypes is the canonical ordered slice of all field types.
var AllPluginSettingsFieldTypes = []PluginSettingsFieldType{
	PluginSettingsFieldString,
	PluginSettingsFieldBoolean,
	PluginSettingsFieldNumber,
	PluginSettingsFieldSelect,
	PluginSettingsFieldSecret,
}

// PluginSettingsFieldTypeRegistry provides queries over the §6.1 settings field type set.
type PluginSettingsFieldTypeRegistry struct{}

// NewPluginSettingsFieldTypeRegistry returns a ready-to-use registry.
func NewPluginSettingsFieldTypeRegistry() *PluginSettingsFieldTypeRegistry {
	return &PluginSettingsFieldTypeRegistry{}
}

// Profile returns the metadata for the given field type.
// Returns false if the type is unknown.
func (r *PluginSettingsFieldTypeRegistry) Profile(ft PluginSettingsFieldType) (PluginSettingsFieldTypeProfile, bool) {
	p, ok := pluginSettingsFieldTypeProfiles[ft]
	return p, ok
}

// AllFieldTypes returns all five field types as a defensive copy.
func (r *PluginSettingsFieldTypeRegistry) AllFieldTypes() []PluginSettingsFieldType {
	result := make([]PluginSettingsFieldType, len(AllPluginSettingsFieldTypes))
	copy(result, AllPluginSettingsFieldTypes)
	return result
}

// MaskableFieldTypes returns field types whose values are sensitive.
func (r *PluginSettingsFieldTypeRegistry) MaskableFieldTypes() []PluginSettingsFieldType {
	var result []PluginSettingsFieldType
	for _, ft := range AllPluginSettingsFieldTypes {
		if pluginSettingsFieldTypeProfiles[ft].IsMaskable {
			result = append(result, ft)
		}
	}
	return result
}

// FieldTypesRequiringOptions returns field types that must declare an options list.
func (r *PluginSettingsFieldTypeRegistry) FieldTypesRequiringOptions() []PluginSettingsFieldType {
	var result []PluginSettingsFieldType
	for _, ft := range AllPluginSettingsFieldTypes {
		if pluginSettingsFieldTypeProfiles[ft].RequiresOptions {
			result = append(result, ft)
		}
	}
	return result
}

// IsValidFieldType returns true when ft is in the bounded set.
func (r *PluginSettingsFieldTypeRegistry) IsValidFieldType(ft PluginSettingsFieldType) bool {
	_, ok := pluginSettingsFieldTypeProfiles[ft]
	return ok
}
