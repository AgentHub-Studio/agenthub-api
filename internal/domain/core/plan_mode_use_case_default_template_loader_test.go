package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePMUDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedPMUDTemplateSlugs))
	assert.Equal(t, 5, SeedExpectedPMUDTemplateRowCount)
}

func TestCorePMUDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPMUDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCorePMUDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedPMUDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCorePMUDTemplate_UseCasesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"dry_run": true, "scoped_change": true,
		"destructive_audit": true, "multi_step_refactor": true,
		"cross_tenant_migration": true,
	}
	for _, u := range SeedExpectedPMUDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 5, len(SeedExpectedPMUDTemplateUseCases))
}

func TestCorePMUDTemplate_PosturesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"permissive": true, "balanced": true, "strict": true,
	}
	for _, p := range SeedExpectedPMUDTemplateSafetyPostures {
		assert.True(t, expected[p])
	}
}

func TestCorePMUDTemplate_TenantKindsClosedSet(t *testing.T) {
	for _, k := range SeedExpectedPMUDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCorePMUDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedPMUDTemplateSlugs,
		SeedRecommendedPMUDTemplateSlugs)
}

func TestCorePMUDTemplate_AdminReviewIsRiskyPostures(t *testing.T) {
	// dry_run + scoped_change são routine; destructive + multi_step +
	// cross_tenant mudam blast radius materialmente.
	expected := []string{
		"destructive-audit", "multi-step-refactor", "cross-tenant-migration",
	}
	assert.ElementsMatch(t, expected, SeedAdminReviewPMUDTemplateSlugs)
}
