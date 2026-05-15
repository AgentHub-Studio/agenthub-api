package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreMACPDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedMACPDTemplateSlugs))
	assert.Equal(t, 5, SeedExpectedMACPDTemplateRowCount)
}

func TestCoreMACPDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedMACPDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreMACPDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedMACPDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreMACPDTemplate_StrategiesMatchSUB011Enum(t *testing.T) {
	expected := map[string]bool{
		"sequential": true, "parallel": true,
		"pipeline": true, "dag": true,
	}
	for _, s := range SeedExpectedMACPDTemplateStrategies {
		assert.True(t, expected[s], "strategy %q outside SUB-011 enum", s)
	}
	assert.Equal(t, 4, len(SeedExpectedMACPDTemplateStrategies))
}

func TestCoreMACPDTemplate_FailurePoliciesMatchSUB011Enum(t *testing.T) {
	expected := map[string]bool{
		"abort_on_failure": true, "continue_on_failure": true,
		"skip_downstream_on_failure": true,
	}
	for _, p := range SeedExpectedMACPDTemplateFailurePolicies {
		assert.True(t, expected[p], "failure policy %q outside SUB-011 enum", p)
	}
	assert.Equal(t, 3, len(SeedExpectedMACPDTemplateFailurePolicies))
}

func TestCoreMACPDTemplate_UseCasesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"linear_workflow": true, "independent_research": true,
		"cicd_pipeline": true, "data_pipeline": true, "complex_orchestration": true,
	}
	for _, u := range SeedExpectedMACPDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 5, len(SeedExpectedMACPDTemplateUseCases))
}

func TestCoreMACPDTemplate_TenantKindsClosedSet(t *testing.T) {
	for _, k := range SeedExpectedMACPDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCoreMACPDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedMACPDTemplateSlugs,
		SeedRecommendedMACPDTemplateSlugs)
}

func TestCoreMACPDTemplate_AdminReviewSubsetIsDAG(t *testing.T) {
	// Only DAG templates require admin review (non-trivial topology +
	// skip_downstream policy implications).
	expected := []string{"dag-build-test-deploy", "dag-with-skip-downstream"}
	assert.ElementsMatch(t, expected, SeedAdminReviewMACPDTemplateSlugs)
}
