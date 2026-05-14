package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreDSHDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedDSHDTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedDSHDTemplateRowCount)
}

func TestCoreDSHDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedDSHDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreDSHDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedDSHDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreDSHDTemplate_PhasesMatchEXT007Enum(t *testing.T) {
	expected := map[string]bool{
		"before_invocation": true, "before_tool_call": true,
		"after_tool_call": true, "after_invocation": true, "on_error": true,
	}
	for _, p := range SeedExpectedDSHDTemplatePhases {
		assert.True(t, expected[p], "phase %q outside EXT-007 enum", p)
	}
	assert.Equal(t, 5, len(SeedExpectedDSHDTemplatePhases))
}

func TestCoreDSHDTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"input_validation": true, "privacy_compliance": true,
		"audit_trail": true, "cost_analytics": true,
		"output_sanitization": true, "error_recovery": true,
	}
	for _, u := range SeedExpectedDSHDTemplateUseCases {
		assert.True(t, expected[u])
	}
}

func TestCoreDSHDTemplate_TenantKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{"general": true, "regulated": true}
	for _, k := range SeedExpectedDSHDTemplateTenantKinds {
		assert.True(t, expected[k])
	}
}

func TestCoreDSHDTemplate_AllRecommendedAreProvenPatterns(t *testing.T) {
	// All 6 are recommended (proven patterns from PDF §6.4).
	assert.ElementsMatch(t,
		SeedExpectedDSHDTemplateSlugs,
		SeedRecommendedDSHDTemplateSlugs)
}

func TestCoreDSHDTemplate_AdminReviewSubsetIsPrivacy(t *testing.T) {
	// Only privacy-related hook (redact-pii) needs admin review.
	assert.Equal(t, []string{"redact-pii"}, SeedAdminReviewDSHDTemplateSlugs)
}
