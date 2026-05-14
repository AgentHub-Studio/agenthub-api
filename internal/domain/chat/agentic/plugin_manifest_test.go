package agentic

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validManifest() PluginManifest {
	return PluginManifest{
		SchemaVersion:      "1.0.0",
		ID:                 "vendor/research-pack",
		Name:               "Research Pack",
		Version:            "1.2.3",
		Description:        "Research workflows for AgentHub.",
		Author:             "Vendor Co",
		Kind:               PluginManifestKindSkillPack,
		MinPlatformVersion: "1.0.0",
		Components: []PluginManifestComponent{
			{Kind: ExtensionComponentSkills, Slug: "web-research", EntryPath: "skills/web-research.yaml"},
			{Kind: ExtensionComponentSkills, Slug: "kb-research", EntryPath: "skills/kb-research.yaml"},
		},
		License: "Apache-2.0",
	}
}

func TestPluginManifestKind_EnumIsBounded(t *testing.T) {
	for _, k := range AllPluginManifestKinds() {
		assert.True(t, IsValidPluginManifestKind(k))
	}
	assert.False(t, IsValidPluginManifestKind(PluginManifestKind("invalid")))
	assert.Equal(t, 7, len(AllPluginManifestKinds()))
}

func TestManifest_Validate_AcceptsValid(t *testing.T) {
	require.NoError(t, validManifest().Validate())
}

func TestManifest_Validate_RejectsInvalidSchemaVersion(t *testing.T) {
	m := validManifest()
	m.SchemaVersion = "1.x"
	err := m.Validate()
	assert.True(t, errors.Is(err, ErrPluginManifestInvalidSchemaVersion))
}

func TestManifest_Validate_RejectsEmptyID(t *testing.T) {
	m := validManifest()
	m.ID = ""
	err := m.Validate()
	assert.True(t, errors.Is(err, ErrPluginManifestIDEmpty))
}

func TestManifest_Validate_RejectsInvalidIDFormat(t *testing.T) {
	for _, bad := range []string{"UPPER", "with spaces", "-leading", "trailing-", "with!special"} {
		m := validManifest()
		m.ID = bad
		err := m.Validate()
		assert.True(t, errors.Is(err, ErrPluginManifestIDInvalid), "id %q must error", bad)
	}
}

func TestManifest_Validate_AllowsNamespacedID(t *testing.T) {
	m := validManifest()
	m.ID = "vendor/plugin-name.v2"
	assert.NoError(t, m.Validate())
}

func TestManifest_Validate_RejectsEmptyName(t *testing.T) {
	m := validManifest()
	m.Name = "  "
	err := m.Validate()
	assert.True(t, errors.Is(err, ErrPluginManifestNameEmpty))
}

func TestManifest_Validate_RejectsInvalidVersion(t *testing.T) {
	m := validManifest()
	m.Version = "1.2"
	err := m.Validate()
	assert.True(t, errors.Is(err, ErrPluginManifestInvalidVersion))
}

func TestManifest_Validate_RejectsLongDescription(t *testing.T) {
	m := validManifest()
	m.Description = strings.Repeat("x", 501)
	err := m.Validate()
	assert.True(t, errors.Is(err, ErrPluginManifestDescriptionTooLong))
}

func TestManifest_Validate_RejectsInvalidKind(t *testing.T) {
	m := validManifest()
	m.Kind = "invalid"
	err := m.Validate()
	assert.True(t, errors.Is(err, ErrPluginManifestInvalidKind))
}

func TestManifest_Validate_RequiresComponentsExceptBundle(t *testing.T) {
	m := validManifest()
	m.Components = nil
	err := m.Validate()
	assert.True(t, errors.Is(err, ErrPluginManifestComponentRequired))

	// Bundle with empty components is allowed.
	m.Kind = PluginManifestKindBundle
	assert.NoError(t, m.Validate())
}

func TestManifest_Validate_RejectsInvalidComponentKind(t *testing.T) {
	m := validManifest()
	m.Components = []PluginManifestComponent{
		{Kind: "worktrees", Slug: "x", EntryPath: "x"},
	}
	err := m.Validate()
	assert.True(t, errors.Is(err, ErrPluginManifestComponentInvalid))
}

func TestManifest_Validate_RejectsEmptyComponentSlug(t *testing.T) {
	m := validManifest()
	m.Components[0].Slug = " "
	err := m.Validate()
	assert.True(t, errors.Is(err, ErrPluginManifestComponentSlugEmpty))
}

func TestManifest_Validate_RejectsDuplicateComponents(t *testing.T) {
	m := validManifest()
	m.Components = append(m.Components, m.Components[0]) // duplicate
	err := m.Validate()
	assert.True(t, errors.Is(err, ErrPluginManifestComponentDuplicate))
}

func TestManifest_Validate_AllowsSameSlugDifferentKind(t *testing.T) {
	m := validManifest()
	m.Components = []PluginManifestComponent{
		{Kind: ExtensionComponentSkills, Slug: "shared", EntryPath: "a"},
		{Kind: ExtensionComponentTools, Slug: "shared", EntryPath: "b"},
	}
	assert.NoError(t, m.Validate())
}

func TestManifest_Validate_RejectsBadDependencyVersion(t *testing.T) {
	m := validManifest()
	m.Dependencies = []PluginManifestDependency{
		{Slug: "other", MinVersion: "bad"},
	}
	err := m.Validate()
	assert.True(t, errors.Is(err, ErrPluginManifestInvalidDependency))
}

func TestManifest_Validate_RejectsEmptyDependencySlug(t *testing.T) {
	m := validManifest()
	m.Dependencies = []PluginManifestDependency{
		{Slug: "", MinVersion: "1.0.0"},
	}
	err := m.Validate()
	assert.True(t, errors.Is(err, ErrPluginManifestInvalidDependency))
}

func TestManifest_Validate_RejectsInvalidMinPlatformVersion(t *testing.T) {
	m := validManifest()
	m.MinPlatformVersion = "v1"
	err := m.Validate()
	assert.Error(t, err)
}

func TestParseManifestJSON_RoundTrips(t *testing.T) {
	m := validManifest()
	data, err := SerializeManifestJSON(m)
	require.NoError(t, err)
	got, err := ParseManifestJSON(data)
	require.NoError(t, err)
	assert.Equal(t, m.ID, got.ID)
	assert.Equal(t, m.Version, got.Version)
	assert.Equal(t, len(m.Components), len(got.Components))
}

func TestParseManifestJSON_RejectsMalformedJSON(t *testing.T) {
	_, err := ParseManifestJSON([]byte("not json"))
	assert.True(t, errors.Is(err, ErrPluginManifestParseFailed))
}

func TestParseManifestJSON_ValidatesAfterParse(t *testing.T) {
	bad := []byte(`{"id":"","name":"x","version":"1.0.0"}`)
	_, err := ParseManifestJSON(bad)
	assert.Error(t, err)
}

func TestSerializeManifestJSON_RejectsInvalidManifest(t *testing.T) {
	m := PluginManifest{ID: "ok"} // missing required fields
	_, err := SerializeManifestJSON(m)
	assert.Error(t, err)
}

func TestManifest_ComponentKinds_DeduplicatesAndSorts(t *testing.T) {
	m := validManifest()
	m.Components = []PluginManifestComponent{
		{Kind: ExtensionComponentTools, Slug: "a", EntryPath: "a"},
		{Kind: ExtensionComponentSkills, Slug: "b", EntryPath: "b"},
		{Kind: ExtensionComponentTools, Slug: "c", EntryPath: "c"},
	}
	got := m.ComponentKinds()
	assert.Equal(t, []ExtensionComponentKind{
		ExtensionComponentSkills, ExtensionComponentTools,
	}, got)
}

func TestManifest_HasComponent(t *testing.T) {
	m := validManifest()
	assert.True(t, m.HasComponent(ExtensionComponentSkills, "web-research"))
	assert.False(t, m.HasComponent(ExtensionComponentSkills, "missing"))
	assert.False(t, m.HasComponent(ExtensionComponentTools, "web-research"))
}

func TestManifest_RequiresPlugin(t *testing.T) {
	m := validManifest()
	m.Dependencies = []PluginManifestDependency{
		{Slug: "shared-utils", MinVersion: "1.0.0"},
	}
	assert.True(t, m.RequiresPlugin("shared-utils"))
	assert.False(t, m.RequiresPlugin("other"))
}

func TestManifest_IsCompatibleWith_AcceptsHigherPlatform(t *testing.T) {
	m := validManifest()
	m.MinPlatformVersion = "1.0.0"
	assert.NoError(t, m.IsCompatibleWith("1.5.0"))
	assert.NoError(t, m.IsCompatibleWith("2.0.0"))
	assert.NoError(t, m.IsCompatibleWith("1.0.0")) // exact match
}

func TestManifest_IsCompatibleWith_RejectsLowerPlatform(t *testing.T) {
	m := validManifest()
	m.MinPlatformVersion = "2.0.0"
	err := m.IsCompatibleWith("1.5.0")
	assert.Error(t, err)
}

func TestManifest_IsCompatibleWith_RejectsBadInput(t *testing.T) {
	m := validManifest()
	err := m.IsCompatibleWith("v1")
	assert.Error(t, err)
}

func TestManifest_Summary(t *testing.T) {
	m := validManifest()
	summary := m.ManifestSummary()
	assert.Contains(t, summary, "vendor/research-pack")
	assert.Contains(t, summary, "1.2.3")
	assert.Contains(t, summary, "skill_pack")
}

func TestManifest_BundleAllowsZeroComponents(t *testing.T) {
	m := validManifest()
	m.Kind = PluginManifestKindBundle
	m.Components = nil
	assert.NoError(t, m.Validate())
}
