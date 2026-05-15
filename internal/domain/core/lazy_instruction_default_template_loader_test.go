package core

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCoreLIDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedLIDTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedLIDTemplateRowCount)
}

func TestCoreLIDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedLIDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreLIDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedLIDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreLIDTemplate_SourceKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"ah_core_seed": true, "tenant_db": true, "external_http": true,
		"compliance_store": true, "memory_hierarchy": true,
	}
	for _, k := range SeedExpectedLIDTemplateSourceKinds {
		assert.True(t, expected[k])
	}
	assert.Equal(t, len(expected), len(SeedExpectedLIDTemplateSourceKinds))
}

func TestCoreLIDTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"rule_lookup": true, "config_lookup": true, "kb_lookup": true,
		"policy_lookup": true, "user_lookup": true,
	}
	for _, u := range SeedExpectedLIDTemplateUseCases {
		assert.True(t, expected[u])
	}
}

func TestCoreLIDTemplate_TenantKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{"general": true, "regulated": true, "dev_local": true}
	for _, k := range SeedExpectedLIDTemplateTenantKinds {
		assert.True(t, expected[k])
	}
}

func TestCoreLIDTemplate_RecommendedExcludesDevDebug(t *testing.T) {
	set := map[string]bool{}
	for _, s := range SeedRecommendedLIDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["dev-debug-no-cache"])
	assert.Equal(t, 5, len(SeedRecommendedLIDTemplateSlugs))
}

func TestCoreLIDTemplate_AdminReviewSubsetIsRegulated(t *testing.T) {
	expected := []string{"regulated-policy-strict-ttl"}
	assert.ElementsMatch(t, expected, SeedAdminReviewLIDTemplateSlugs)
}

func TestCoreLIDTemplate_TTLDurationConverts(t *testing.T) {
	tmpl := CoreLazyInstructionDefaultTemplate{TTLSeconds: 3600}
	assert.Equal(t, time.Hour, tmpl.TTLDuration())

	never := CoreLazyInstructionDefaultTemplate{TTLSeconds: 0}
	assert.Equal(t, time.Duration(0), never.TTLDuration())
}
