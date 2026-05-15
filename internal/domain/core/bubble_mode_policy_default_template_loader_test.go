package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreBMPDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedBMPDTemplateSlugs))
	assert.Equal(t, 5, SeedExpectedBMPDTemplateRowCount)
}

func TestCoreBMPDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedBMPDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreBMPDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedBMPDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreBMPDTemplate_UseCasesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"leaf_subagent": true, "two_tier": true,
		"three_tier_orchestration": true,
		"multi_stage_pipeline": true, "research_curator_network": true,
	}
	for _, u := range SeedExpectedBMPDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 5, len(SeedExpectedBMPDTemplateUseCases))
}

func TestCoreBMPDTemplate_PosturesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"strict": true, "balanced": true, "permissive": true,
	}
	for _, p := range SeedExpectedBMPDTemplateSafetyPostures {
		assert.True(t, expected[p])
	}
}

func TestCoreBMPDTemplate_TenantKindsClosedSet(t *testing.T) {
	for _, k := range SeedExpectedBMPDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCoreBMPDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedBMPDTemplateSlugs,
		SeedRecommendedBMPDTemplateSlugs)
}

func TestCoreBMPDTemplate_AdminReviewExcludesSingleHop(t *testing.T) {
	// single-hop is the routine 2-tier baseline; others change escalation
	// depth materially and require admin review.
	set := map[string]bool{}
	for _, s := range SeedAdminReviewBMPDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["single-hop"])
	assert.True(t, set["leaf-auto-deny"])
	assert.True(t, set["two-hop-routine"])
	assert.True(t, set["three-hop-orchestration"])
	assert.True(t, set["five-hop-research"])
}
