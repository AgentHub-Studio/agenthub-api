package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreContextBudgetTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedContextBudgetTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedContextBudgetTemplateRowCount)
}

func TestCoreContextBudgetTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedContextBudgetTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreContextBudgetTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedContextBudgetTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreContextBudgetTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"general": true, "research": true,
		"conversation": true, "code": true,
	}
	for _, u := range SeedExpectedContextBudgetTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, len(expected), len(SeedExpectedContextBudgetTemplateUseCases))
}

func TestCoreContextBudgetTemplate_ModelFamiliesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"mid_tier": true, "small_local": true,
		"large_context": true, "tiny_legacy": true,
	}
	for _, m := range SeedExpectedContextBudgetTemplateModelFamilies {
		assert.True(t, expected[m])
	}
}

func TestCoreContextBudgetTemplate_RecommendedExcludesMinimumViable(t *testing.T) {
	// minimum-viable-4k is last-resort, not a recommended starting point.
	set := map[string]bool{}
	for _, s := range SeedRecommendedContextBudgetTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["minimum-viable-4k"],
		"minimum-viable must be opt-in only, not one-click default")
	assert.Equal(t, 5, len(SeedRecommendedContextBudgetTemplateSlugs))
}

func TestCoreContextBudgetTemplate_PerSectionCapsMapsToCTX001Sections(t *testing.T) {
	tmpl := CoreContextSectionBudgetTemplate{
		CapSystem:         100,
		CapMemory:         200,
		CapRules:          300,
		CapSkillCatalog:   400,
		CapToolCatalog:    500,
		CapKBSummary:      600,
		CapRecentMessages: 700,
		CapAuxPrompt:      800,
		CapScratchpad:     900,
	}
	caps := tmpl.PerSectionCaps()
	expected := map[string]int{
		"system": 100, "memory": 200, "rules": 300,
		"skill_catalog": 400, "tool_catalog": 500, "kb_summary": 600,
		"recent_messages": 700, "aux_prompt": 800, "scratchpad": 900,
	}
	assert.Equal(t, expected, caps)
}

func TestCoreContextBudgetTemplate_PerSectionCapsCoversAllNineCTX001Sections(t *testing.T) {
	// CTX-001 has 9 ContextSectionKind values. PerSectionCaps must
	// expose ALL nine so any CTX-001 section has a corresponding cap.
	tmpl := CoreContextSectionBudgetTemplate{}
	caps := tmpl.PerSectionCaps()
	assert.Equal(t, 9, len(caps),
		"PerSectionCaps must expose one entry per CTX-001 ContextSectionKind")
}
