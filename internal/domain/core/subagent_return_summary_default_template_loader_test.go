package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreSRSDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 5, len(SeedExpectedSRSDTemplateSlugs))
	assert.Equal(t, 5, SeedExpectedSRSDTemplateRowCount)
}

func TestCoreSRSDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedSRSDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreSRSDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedSRSDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreSRSDTemplate_OutcomesMatchSUB010Enum(t *testing.T) {
	expected := map[string]bool{
		"success": true, "partial": true,
		"failed": true, "aborted": true,
	}
	for _, o := range SeedExpectedSRSDTemplateOutcomes {
		assert.True(t, expected[o], "outcome %q outside SUB-010 enum", o)
	}
	assert.Equal(t, 4, len(SeedExpectedSRSDTemplateOutcomes))
}

func TestCoreSRSDTemplate_UseCasesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"task_completion": true, "incremental_progress": true,
		"error_diagnosis": true, "interruption_handling": true,
		"compliance_export": true,
	}
	for _, u := range SeedExpectedSRSDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 5, len(SeedExpectedSRSDTemplateUseCases))
}

func TestCoreSRSDTemplate_TenantKindsClosedSet(t *testing.T) {
	for _, k := range SeedExpectedSRSDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCoreSRSDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedSRSDTemplateSlugs,
		SeedRecommendedSRSDTemplateSlugs)
}

func TestCoreSRSDTemplate_AdminReviewIsRiskyOutcomes(t *testing.T) {
	// success-with-artifacts + partial são routine (parent integra
	// resultado direto). failed/aborted/audit precisam review.
	expected := []string{
		"failed-error", "aborted-by-parent", "audit-with-redaction",
	}
	assert.ElementsMatch(t, expected, SeedAdminReviewSRSDTemplateSlugs)
}
