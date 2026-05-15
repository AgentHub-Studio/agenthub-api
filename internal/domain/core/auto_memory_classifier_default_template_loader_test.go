package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreAMCDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedAMCDTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedAMCDTemplateRowCount)
}

func TestCoreAMCDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedAMCDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreAMCDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedAMCDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreAMCDTemplate_PosturesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"balanced": true, "strict": true, "lenient": true,
		"privacy_first": true, "pii_strict": true, "dev_debug": true,
	}
	for _, p := range SeedExpectedAMCDTemplatePostures {
		assert.True(t, expected[p])
	}
	assert.Equal(t, len(expected), len(SeedExpectedAMCDTemplatePostures))
}

func TestCoreAMCDTemplate_TenantKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"general": true, "regulated": true, "dev_local": true,
	}
	for _, k := range SeedExpectedAMCDTemplateTenantKinds {
		assert.True(t, expected[k])
	}
}

func TestCoreAMCDTemplate_RecommendedExcludesDevDebug(t *testing.T) {
	// dev-debug deliberately not recommended — production tenants
	// should not enable verbose debug posture.
	set := map[string]bool{}
	for _, s := range SeedRecommendedAMCDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["dev-debug"])
	assert.Equal(t, 5, len(SeedRecommendedAMCDTemplateSlugs))
}

func TestCoreAMCDTemplate_AdminReviewIsPrivacyAndPIISubset(t *testing.T) {
	expected := []string{"privacy-first", "pii-strict"}
	assert.ElementsMatch(t, expected, SeedAdminReviewAMCDTemplateSlugs)
}

func TestCoreAMCDTemplate_AdminReviewExcludesNonPrivacyTemplates(t *testing.T) {
	// balanced/strict/lenient don't change privacy posture — no admin review.
	set := map[string]bool{}
	for _, s := range SeedAdminReviewAMCDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["balanced-default"])
	assert.False(t, set["strict-conservative"])
	assert.False(t, set["lenient-exploration"])
}

func TestCoreAMCDTemplate_BlockedKeysListParserHandlesEmpty(t *testing.T) {
	tmpl := CoreAutoMemoryClassifierDefaultTemplate{AdminBlockedKeys: ""}
	assert.Nil(t, tmpl.AdminBlockedKeysList())

	tmpl2 := CoreAutoMemoryClassifierDefaultTemplate{AdminBlockedKeys: "  "}
	assert.Nil(t, tmpl2.AdminBlockedKeysList())
}

func TestCoreAMCDTemplate_BlockedKeysListParserHandlesCSV(t *testing.T) {
	tmpl := CoreAutoMemoryClassifierDefaultTemplate{
		AdminBlockedKeys: "password, ssn, credit_card",
	}
	got := tmpl.AdminBlockedKeysList()
	assert.Equal(t, []string{"password", "ssn", "credit_card"}, got)
}
