package core

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCoreCBDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedCBDTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedCBDTemplateRowCount)
}

func TestCoreCBDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedCBDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreCBDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedCBDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreCBDTemplate_TriggerKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"hybrid_balanced": true, "turn_count": true,
		"token_pressure": true, "cost": true,
		"idle": true, "dev_debug": true,
	}
	for _, k := range SeedExpectedCBDTemplateTriggerKinds {
		assert.True(t, expected[k])
	}
	assert.Equal(t, len(expected), len(SeedExpectedCBDTemplateTriggerKinds))
}

func TestCoreCBDTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{"general": true, "research": true}
	for _, u := range SeedExpectedCBDTemplateUseCases {
		assert.True(t, expected[u])
	}
}

func TestCoreCBDTemplate_ModelFamiliesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"mid_tier": true, "large_context": true, "dev_local": true,
	}
	for _, m := range SeedExpectedCBDTemplateModelFamilies {
		assert.True(t, expected[m])
	}
}

func TestCoreCBDTemplate_RecommendedExcludesDevDebug(t *testing.T) {
	set := map[string]bool{}
	for _, s := range SeedRecommendedCBDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["dev-debug-no-compact"])
	assert.Equal(t, 5, len(SeedRecommendedCBDTemplateSlugs))
}

func TestCoreCBDTemplate_AdminReviewSubsetIsCostStrict(t *testing.T) {
	assert.Equal(t, []string{"cost-strict"}, SeedAdminReviewCBDTemplateSlugs)
}

func TestCoreCBDTemplate_IdleDurationConverts(t *testing.T) {
	tmpl := CoreCompactBoundaryDefaultTemplate{IdleSeconds: 1800}
	assert.Equal(t, 30*time.Minute, tmpl.IdleDuration())

	disabled := CoreCompactBoundaryDefaultTemplate{IdleSeconds: 0}
	assert.Equal(t, time.Duration(0), disabled.IdleDuration())
}
