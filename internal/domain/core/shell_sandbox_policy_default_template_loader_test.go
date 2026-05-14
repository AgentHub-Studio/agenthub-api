package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreSSPDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedSSPDTemplateSlugs))
	assert.Equal(t, 5, SeedExpectedSSPDTemplateRowCount)
}

func TestCoreSSPDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedSSPDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreSSPDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedSSPDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreSSPDTemplate_SafetyPosturesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"strict": true, "balanced": true, "progressive": true,
		"permissive": true, "conservative": true,
	}
	for _, p := range SeedExpectedSSPDTemplateSafetyPostures {
		assert.True(t, expected[p])
	}
	assert.Equal(t, 5, len(SeedExpectedSSPDTemplateSafetyPostures))
}

func TestCoreSSPDTemplate_UseCasesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"audit_session": true, "standard_chat": true,
		"engineering": true, "cicd_pipeline": true,
		"incident_response": true,
	}
	for _, u := range SeedExpectedSSPDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 5, len(SeedExpectedSSPDTemplateUseCases))
}

func TestCoreSSPDTemplate_TenantKindsClosedSet(t *testing.T) {
	for _, k := range SeedExpectedSSPDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCoreSSPDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedSSPDTemplateSlugs,
		SeedRecommendedSSPDTemplateSlugs)
}

func TestCoreSSPDTemplate_AdminReviewExcludesWebSafeDefault(t *testing.T) {
	// web-safe is the routine default for fresh tenants; the others
	// materially change posture (lockdown = no FS; dev = network on;
	// cicd = long runtime; incident = read /etc & /var/log).
	set := map[string]bool{}
	for _, s := range SeedAdminReviewSSPDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["web-safe-default"])
	assert.True(t, set["locked-down"])
	assert.True(t, set["dev-workstation"])
	assert.True(t, set["cicd-runner"])
	assert.True(t, set["incident-response-readonly"])
}
