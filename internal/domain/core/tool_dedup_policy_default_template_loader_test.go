package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreTDPDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedTDPDTemplateSlugs))
	assert.Equal(t, 5, SeedExpectedTDPDTemplateRowCount)
}

func TestCoreTDPDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedTDPDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreTDPDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedTDPDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreTDPDTemplate_PoliciesMatchTOOL004Enum(t *testing.T) {
	expected := map[string]bool{
		"deny_collision": true, "prefer_source_rank": true,
		"prefer_latest_version": true, "prefer_pinned": true,
		"keep_all_versions": true,
	}
	for _, p := range SeedExpectedTDPDTemplatePolicies {
		assert.True(t, expected[p], "policy %q outside TOOL-004 enum", p)
	}
	assert.Equal(t, 5, len(SeedExpectedTDPDTemplatePolicies))
}

func TestCoreTDPDTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"general_default": true, "continuous_upgrade": true,
		"compliance_pin": true, "migration_window": true,
		"audit_strict": true,
	}
	for _, u := range SeedExpectedTDPDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 5, len(SeedExpectedTDPDTemplateUseCases))
}

func TestCoreTDPDTemplate_SafetyPosturesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"balanced": true, "progressive": true, "conservative": true,
		"permissive": true, "strict": true,
	}
	for _, p := range SeedExpectedTDPDTemplateSafetyPostures {
		assert.True(t, expected[p])
	}
	assert.Equal(t, 5, len(SeedExpectedTDPDTemplateSafetyPostures))
}

func TestCoreTDPDTemplate_TenantKindsAreClosedSet(t *testing.T) {
	for _, k := range SeedExpectedTDPDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCoreTDPDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedTDPDTemplateSlugs,
		SeedRecommendedTDPDTemplateSlugs)
}

func TestCoreTDPDTemplate_AdminReviewIsConservativeAndStrict(t *testing.T) {
	assert.ElementsMatch(t,
		[]string{"admin-pinned-conservative", "fail-fast-no-drift"},
		SeedAdminReviewTDPDTemplateSlugs)
}
