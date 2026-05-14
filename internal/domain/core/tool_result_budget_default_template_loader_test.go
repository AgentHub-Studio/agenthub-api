package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreTRBDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedTRBDTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedTRBDTemplateRowCount)
}

func TestCoreTRBDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedTRBDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreTRBDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedTRBDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreTRBDTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"general": true, "research": true, "code": true,
	}
	for _, u := range SeedExpectedTRBDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, len(expected), len(SeedExpectedTRBDTemplateUseCases))
}

func TestCoreTRBDTemplate_ModelFamiliesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"mid_tier": true, "small_local": true,
		"large_context": true, "dev_local": true,
	}
	for _, m := range SeedExpectedTRBDTemplateModelFamilies {
		assert.True(t, expected[m])
	}
}

func TestCoreTRBDTemplate_RecommendedExcludesDevDebug(t *testing.T) {
	set := map[string]bool{}
	for _, s := range SeedRecommendedTRBDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["dev-debug"])
	assert.Equal(t, 5, len(SeedRecommendedTRBDTemplateSlugs))
}

func TestCoreTRBDTemplate_AdminReviewSubsetIsCostStrict(t *testing.T) {
	// cost-strict has business impact — degrades agent visibility.
	assert.Equal(t, []string{"cost-strict"}, SeedAdminReviewTRBDTemplateSlugs)
}
