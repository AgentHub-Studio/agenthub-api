package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCorePMETemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 7, len(SeedExpectedPMETemplateSlugs))
	assert.Equal(t, 7, SeedExpectedPMETemplateRowCount)
}

func TestCorePMETemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPMETemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCorePMETemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedPMETemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCorePMETemplate_KindsMatchEXT004Enum(t *testing.T) {
	expected := map[string]bool{
		"agent": true, "skill_pack": true, "tool_pack": true,
		"hook_pack": true, "rule_pack": true, "theme_pack": true,
		"bundle": true,
	}
	for _, k := range SeedExpectedPMETemplateKinds {
		assert.True(t, expected[k], "kind %q outside EXT-004 enum", k)
	}
	assert.Equal(t, 7, len(SeedExpectedPMETemplateKinds))
}

func TestCorePMETemplate_AudiencesClosedSet(t *testing.T) {
	expected := map[string]bool{"extension_author": true}
	for _, a := range SeedExpectedPMETemplateAudiences {
		assert.True(t, expected[a])
	}
}

func TestCorePMETemplate_AllRecommendedSafeSkeletons(t *testing.T) {
	// All 7 are recommended (educational examples; no risk).
	assert.ElementsMatch(t,
		SeedExpectedPMETemplateSlugs,
		SeedRecommendedPMETemplateSlugs)
}

func TestCorePMETemplate_ParseExampleValidatesJSON(t *testing.T) {
	tmpl := CorePluginManifestExampleTemplate{
		Slug: "x",
		ExampleManifestJSON: `{"id":"x","version":"1.0.0","name":"test"}`,
	}
	got, err := tmpl.ParseExample()
	require.NoError(t, err)
	assert.Equal(t, "x", got["id"])
}

func TestCorePMETemplate_ParseExampleRejectsMalformed(t *testing.T) {
	tmpl := CorePluginManifestExampleTemplate{
		Slug: "x",
		ExampleManifestJSON: "not json",
	}
	_, err := tmpl.ParseExample()
	assert.Error(t, err)
}
