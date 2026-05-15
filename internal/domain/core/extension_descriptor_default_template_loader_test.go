package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreEDDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedEDDTemplateSlugs))
	assert.Equal(t, 5, SeedExpectedEDDTemplateRowCount)
}

func TestCoreEDDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedEDDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreEDDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedEDDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreEDDTemplate_SourcesMatchEXT001Enum(t *testing.T) {
	expected := map[string]bool{
		"builtin": true, "marketplace": true,
		"git": true, "url": true,
	}
	for _, s := range SeedExpectedEDDTemplateSources {
		assert.True(t, expected[s], "source %q outside EXT-001 enum", s)
	}
	assert.Equal(t, 4, len(SeedExpectedEDDTemplateSources))
}

func TestCoreEDDTemplate_UseCasesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"platform_baseline": true, "rag_search": true,
		"engineering": true, "internal_tooling": true, "vendor_delivery": true,
	}
	for _, u := range SeedExpectedEDDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 5, len(SeedExpectedEDDTemplateUseCases))
}

func TestCoreEDDTemplate_PosturesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"balanced": true, "conservative": true, "strict": true,
	}
	for _, p := range SeedExpectedEDDTemplateSafetyPostures {
		assert.True(t, expected[p])
	}
}

func TestCoreEDDTemplate_ComponentsMatchEXT001Enum(t *testing.T) {
	expected := map[string]bool{
		"agents": true, "tools": true, "skills": true, "commands": true,
		"hooks": true, "rules": true, "mcp_servers": true, "output_styles": true,
	}
	for _, c := range SeedExpectedEDDTemplateComponents {
		assert.True(t, expected[c], "component %q outside EXT-001 enum", c)
	}
	assert.Equal(t, 8, len(SeedExpectedEDDTemplateComponents))
}

func TestCoreEDDTemplate_TenantKindsClosedSet(t *testing.T) {
	for _, k := range SeedExpectedEDDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCoreEDDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedEDDTemplateSlugs,
		SeedRecommendedEDDTemplateSlugs)
}

func TestCoreEDDTemplate_AdminReviewSubset(t *testing.T) {
	// builtin + marketplace-rag are routine. Engineering (broader
	// surface) + git + url require admin review.
	expected := []string{
		"marketplace-engineering-pack", "git-internal-tools", "url-vendor-skills",
	}
	assert.ElementsMatch(t, expected, SeedAdminReviewEDDTemplateSlugs)
}
