package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreCRPDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedCRPDTemplateSlugs))
	assert.Equal(t, 5, SeedExpectedCRPDTemplateRowCount)
}

func TestCoreCRPDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedCRPDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreCRPDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedCRPDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreCRPDTemplate_PoliciesMatchEXT005Enum(t *testing.T) {
	expected := map[string]bool{
		"first_install_wins": true, "latest_install_wins": true,
		"require_explicit_pin": true, "error_on_conflict": true,
	}
	for _, p := range SeedExpectedCRPDTemplatePolicies {
		assert.True(t, expected[p], "policy %q outside EXT-005 enum", p)
	}
	assert.Equal(t, 4, len(SeedExpectedCRPDTemplatePolicies))
}

func TestCoreCRPDTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{"general": true, "regulated": true, "staging": true}
	for _, u := range SeedExpectedCRPDTemplateUseCases {
		assert.True(t, expected[u])
	}
}

func TestCoreCRPDTemplate_TenantKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{"general": true, "regulated": true, "dev_local": true}
	for _, k := range SeedExpectedCRPDTemplateTenantKinds {
		assert.True(t, expected[k])
	}
}

func TestCoreCRPDTemplate_RecommendedExcludesDevDebug(t *testing.T) {
	set := map[string]bool{}
	for _, s := range SeedRecommendedCRPDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["dev-debug-incumbent"])
	assert.Equal(t, 4, len(SeedRecommendedCRPDTemplateSlugs))
}

func TestCoreCRPDTemplate_AdminReviewSubsetIsStrictPinned(t *testing.T) {
	assert.Equal(t, []string{"strict-pinned"}, SeedAdminReviewCRPDTemplateSlugs)
}
