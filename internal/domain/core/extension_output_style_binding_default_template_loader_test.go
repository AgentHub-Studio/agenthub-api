package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreEOSBDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedEOSBDTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedEOSBDTemplateRowCount)
}

func TestCoreEOSBDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedEOSBDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreEOSBDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedEOSBDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreEOSBDTemplate_ScopesMatchEXT009Enum(t *testing.T) {
	expected := map[string]bool{
		"explicit": true, "agent": true, "tenant": true, "platform": true,
	}
	for _, s := range SeedExpectedEOSBDTemplateScopes {
		assert.True(t, expected[s], "scope %q outside EXT-009 enum", s)
	}
	assert.Equal(t, 4, len(SeedExpectedEOSBDTemplateScopes))
}

func TestCoreEOSBDTemplate_FormatsMatchEXT009Enum(t *testing.T) {
	expected := map[string]bool{
		"markdown": true, "json": true, "plain": true, "html_sanitized": true,
	}
	for _, f := range SeedExpectedEOSBDTemplateFormats {
		assert.True(t, expected[f], "format %q outside EXT-009 enum", f)
	}
	assert.Equal(t, 4, len(SeedExpectedEOSBDTemplateFormats))
}

func TestCoreEOSBDTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"general": true, "engineering": true, "data_extraction": true,
		"debugging": true, "user_facing_ui": true, "fallback": true,
	}
	for _, u := range SeedExpectedEOSBDTemplateUseCases {
		assert.True(t, expected[u])
	}
}

func TestCoreEOSBDTemplate_TenantKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{"general": true}
	for _, k := range SeedExpectedEOSBDTemplateTenantKinds {
		assert.True(t, expected[k])
	}
}

func TestCoreEOSBDTemplate_AllRecommendedSafePresets(t *testing.T) {
	// All 6 presets are recommended (each implements a documented EXT-009 pattern).
	assert.ElementsMatch(t,
		SeedExpectedEOSBDTemplateSlugs,
		SeedRecommendedEOSBDTemplateSlugs)
}

func TestCoreEOSBDTemplate_AdminReviewSubsetIsHTMLSanitized(t *testing.T) {
	// HTML sanitized is the only binding with security implications
	// (rendering posture change).
	assert.Equal(t, []string{"tenant-html-sanitized-ui"}, SeedAdminReviewEOSBDTemplateSlugs)
}
