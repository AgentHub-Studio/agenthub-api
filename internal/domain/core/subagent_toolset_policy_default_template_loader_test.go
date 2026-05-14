package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreSTPDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 4, len(SeedExpectedSTPDTemplateSlugs))
	assert.Equal(t, 4, SeedExpectedSTPDTemplateRowCount)
}

func TestCoreSTPDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedSTPDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreSTPDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedSTPDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreSTPDTemplate_ModesMatchSUB005Enum(t *testing.T) {
	expected := map[string]bool{
		"explicit_allowlist": true, "parent_minus_blocklist": true,
		"depth_filtered": true, "categorical_exclusion": true,
	}
	for _, m := range SeedExpectedSTPDTemplateModes {
		assert.True(t, expected[m], "mode %q outside SUB-005 enum", m)
	}
	assert.Equal(t, 4, len(SeedExpectedSTPDTemplateModes))
}

func TestCoreSTPDTemplate_UseCasesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"documentation_generator": true, "research_assistant": true,
		"recursion_safety": true, "sensitive_surface_protection": true,
	}
	for _, u := range SeedExpectedSTPDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 4, len(SeedExpectedSTPDTemplateUseCases))
}

func TestCoreSTPDTemplate_SafetyPosturesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"strict": true, "balanced": true, "conservative": true,
	}
	for _, p := range SeedExpectedSTPDTemplateSafetyPostures {
		assert.True(t, expected[p])
	}
}

func TestCoreSTPDTemplate_TenantKindsClosedSet(t *testing.T) {
	for _, k := range SeedExpectedSTPDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCoreSTPDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedSTPDTemplateSlugs,
		SeedRecommendedSTPDTemplateSlugs)
}

func TestCoreSTPDTemplate_AdminReviewExcludesResearchRoutine(t *testing.T) {
	set := map[string]bool{}
	for _, s := range SeedAdminReviewSTPDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["research-minus-sharp-edges"])
	assert.True(t, set["documentation-readonly-allowlist"])
	assert.True(t, set["recursion-safety-depth-filtered"])
	assert.True(t, set["admin-surface-categorical-exclusion"])
}
