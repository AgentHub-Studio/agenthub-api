package core

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCoreCRDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedCRDTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedCRDTemplateRowCount)
}

func TestCoreCRDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedCRDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreCRDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedCRDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreCRDTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"general": true, "research": true, "code": true,
	}
	for _, u := range SeedExpectedCRDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, len(expected), len(SeedExpectedCRDTemplateUseCases))
}

func TestCoreCRDTemplate_TenantKindsAreClosedSet(t *testing.T) {
	expected := map[string]bool{"general": true, "dev_local": true}
	for _, k := range SeedExpectedCRDTemplateTenantKinds {
		assert.True(t, expected[k])
	}
}

func TestCoreCRDTemplate_KindLabelsMatchCTX009Enum(t *testing.T) {
	expected := map[string]bool{
		"kb_chunk": true, "tool_result": true, "file_blob": true,
		"web_fetch": true, "memory_snapshot": true,
	}
	for _, k := range SeedExpectedCRDTemplateKindLabels {
		assert.True(t, expected[k], "kind %q outside CTX-009 enum", k)
	}
	assert.Equal(t, 5, len(SeedExpectedCRDTemplateKindLabels))
}

func TestCoreCRDTemplate_RecommendedExcludesDevDebug(t *testing.T) {
	set := map[string]bool{}
	for _, s := range SeedRecommendedCRDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["dev-debug"])
	assert.Equal(t, 5, len(SeedRecommendedCRDTemplateSlugs))
}

func TestCoreCRDTemplate_AdminReviewSubsetIsCostStrict(t *testing.T) {
	assert.Equal(t, []string{"cost-strict"}, SeedAdminReviewCRDTemplateSlugs)
}

func TestCoreCRDTemplate_EnabledKindsListParser(t *testing.T) {
	tmpl := CoreContentReferenceDefaultTemplate{
		EnabledKinds: "kb_chunk, tool_result, file_blob",
	}
	got := tmpl.EnabledKindsList()
	assert.Equal(t, []string{"kb_chunk", "tool_result", "file_blob"}, got)
}

func TestCoreCRDTemplate_EnabledKindsListEmpty(t *testing.T) {
	tmpl := CoreContentReferenceDefaultTemplate{EnabledKinds: ""}
	assert.Nil(t, tmpl.EnabledKindsList())

	tmpl2 := CoreContentReferenceDefaultTemplate{EnabledKinds: "  "}
	assert.Nil(t, tmpl2.EnabledKindsList())
}

func TestCoreCRDTemplate_IdleGCDurationConverts(t *testing.T) {
	tmpl := CoreContentReferenceDefaultTemplate{IdleGCSeconds: 86400}
	assert.Equal(t, 24*time.Hour, tmpl.IdleGCDuration())

	never := CoreContentReferenceDefaultTemplate{IdleGCSeconds: 0}
	assert.Equal(t, time.Duration(0), never.IdleGCDuration())
}
