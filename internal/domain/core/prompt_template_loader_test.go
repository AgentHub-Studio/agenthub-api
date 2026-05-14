package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorePromptTemplate_SlugsNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedPromptTemplateSlugs {
		assert.False(t, seen[s], "duplicate %q", s)
		seen[s] = true
	}
}

func TestCorePromptTemplate_SlugsCanonicalCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedPromptTemplateSlugs),
		"8 templates: assistant + coder + analyst + researcher + writer + translator + support + extractor")
}

func TestCorePromptTemplate_KindsAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range SeedExpectedPromptTemplateKinds {
		assert.False(t, seen[k])
		seen[k] = true
	}
}

func TestCorePromptTemplate_KindsCanonicalCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedPromptTemplateKinds))
}

func TestCorePromptTemplate_RecommendedAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedPromptTemplateSlugs {
		seedSet[s] = true
	}
	for _, r := range SeedRecommendedPromptTemplateSlugs {
		assert.True(t, seedSet[r], "recommended %q must be in seed", r)
	}
}

func TestCorePromptTemplate_RenderSystemPrompt_SubstitutesPlaceholders(t *testing.T) {
	tmpl := CorePromptTemplate{
		SystemPrompt: "You help {{userName}} with {{taskKind}}.",
	}
	got := tmpl.RenderSystemPrompt(map[string]string{
		"userName": "Alice", "taskKind": "data analysis",
	})
	assert.Equal(t, "You help Alice with data analysis.", got)
}

func TestCorePromptTemplate_RenderSystemPrompt_MissingKeysRemainLiteral(t *testing.T) {
	tmpl := CorePromptTemplate{SystemPrompt: "Hello {{name}}, do {{task}}."}
	got := tmpl.RenderSystemPrompt(map[string]string{"name": "Bob"})
	assert.Equal(t, "Hello Bob, do {{task}}.", got)
}

func TestCorePromptTemplate_RenderSystemPrompt_EmptyValuesMap(t *testing.T) {
	tmpl := CorePromptTemplate{SystemPrompt: "Static text"}
	assert.Equal(t, "Static text", tmpl.RenderSystemPrompt(nil))
}

func TestCorePromptTemplate_RequiresToolsList_Parses(t *testing.T) {
	tmpl := CorePromptTemplate{RequiresTools: "web_search,knowledge_base_search"}
	got := tmpl.RequiresToolsList()
	assert.Equal(t, []string{"web_search", "knowledge_base_search"}, got)
}

func TestCorePromptTemplate_PlaceholdersList_Parses(t *testing.T) {
	tmpl := CorePromptTemplate{Placeholders: "tenantName, productName"}
	got := tmpl.PlaceholdersList()
	assert.Equal(t, []string{"tenantName", "productName"}, got)
}

func TestCorePromptTemplate_SlugsKebabCase(t *testing.T) {
	for _, s := range SeedExpectedPromptTemplateSlugs {
		assert.False(t, strings.Contains(s, "_"), "slug %q must use kebab-case", s)
	}
}
