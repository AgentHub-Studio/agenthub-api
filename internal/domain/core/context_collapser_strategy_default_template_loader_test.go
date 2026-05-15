package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreCCSDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedCCSDTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedCCSDTemplateRowCount)
}

func TestCoreCCSDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedCCSDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreCCSDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedCCSDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreCCSDTemplate_StrategiesMatchCTX012Enum(t *testing.T) {
	expected := map[string]bool{
		"minimal": true, "balanced": true, "aggressive": true,
	}
	for _, s := range SeedExpectedCCSDTemplateStrategies {
		assert.True(t, expected[s], "strategy %q outside CTX-012 enum", s)
	}
	assert.Equal(t, 3, len(SeedExpectedCCSDTemplateStrategies))
}

func TestCoreCCSDTemplate_ConsumerKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"llm_renderer": true, "debugger": true, "cost_dashboard": true,
		"audit_exporter": true, "ui_summary": true,
	}
	for _, k := range SeedExpectedCCSDTemplateConsumerKinds {
		assert.True(t, expected[k])
	}
}

func TestCoreCCSDTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"agent_runtime": true, "incident_replay": true, "analytics": true,
		"compliance_export": true, "user_facing_ui": true,
	}
	for _, u := range SeedExpectedCCSDTemplateUseCases {
		assert.True(t, expected[u])
	}
}

func TestCoreCCSDTemplate_TenantKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{"general": true, "regulated": true, "dev_local": true}
	for _, k := range SeedExpectedCCSDTemplateTenantKinds {
		assert.True(t, expected[k])
	}
}

func TestCoreCCSDTemplate_RecommendedExcludesDevDebug(t *testing.T) {
	set := map[string]bool{}
	for _, s := range SeedRecommendedCCSDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["dev-debug-verbose-no-recommendations"])
	assert.Equal(t, 5, len(SeedRecommendedCCSDTemplateSlugs))
}

func TestCoreCCSDTemplate_AdminReviewSubsetIsAuditExport(t *testing.T) {
	assert.Equal(t, []string{"audit-export-minimal"}, SeedAdminReviewCCSDTemplateSlugs)
}
