package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePARDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedPARDTemplateSlugs))
	assert.Equal(t, 5, SeedExpectedPARDTemplateRowCount)
}

func TestCorePARDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPARDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCorePARDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedPARDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCorePARDTemplate_PosturesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"minimal": true, "balanced": true, "regulated": true,
		"pii_strict": true, "forensic_hold": true,
	}
	for _, p := range SeedExpectedPARDTemplatePostures {
		assert.True(t, expected[p])
	}
	assert.Equal(t, 5, len(SeedExpectedPARDTemplatePostures))
}

func TestCorePARDTemplate_UseCasesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"debug_only": true, "standard_audit": true,
		"compliance_audit": true, "gdpr_lgpd": true, "legal_hold": true,
	}
	for _, u := range SeedExpectedPARDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 5, len(SeedExpectedPARDTemplateUseCases))
}

func TestCorePARDTemplate_TenantKindsClosedSet(t *testing.T) {
	for _, k := range SeedExpectedPARDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCorePARDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedPARDTemplateSlugs,
		SeedRecommendedPARDTemplateSlugs)
}

func TestCorePARDTemplate_AdminReviewIsCompliancePostures(t *testing.T) {
	expected := []string{
		"regulated-7y-deny-2y-confirm",
		"strict-pii-redacted-export",
		"forensic-hold-never-expires",
	}
	assert.ElementsMatch(t, expected, SeedAdminReviewPARDTemplateSlugs)
}
