package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreTPPDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedTPPDTemplateSlugs))
	assert.Equal(t, 6, SeedExpectedTPPDTemplateRowCount)
}

func TestCoreTPPDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedTPPDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreTPPDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedTPPDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreTPPDTemplate_SourcesMatchTOOL003Enum(t *testing.T) {
	expected := map[string]bool{
		"builtin": true, "skill": true, "mcp": true,
		"subagent": true, "extension": true,
	}
	for _, s := range SeedExpectedTPPDTemplateSources {
		assert.True(t, expected[s], "source %q outside TOOL-003 enum", s)
	}
	assert.Equal(t, 5, len(SeedExpectedTPPDTemplateSources))
}

func TestCoreTPPDTemplate_UseCasesAreClosedSet(t *testing.T) {
	expected := map[string]bool{
		"inspection": true, "mutation": true, "rag_search": true,
		"external_integration": true, "delegation_safety": true,
		"observability": true,
	}
	for _, u := range SeedExpectedTPPDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 6, len(SeedExpectedTPPDTemplateUseCases))
}

func TestCoreTPPDTemplate_TenantKindsAreClosedSet(t *testing.T) {
	for _, k := range SeedExpectedTPPDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCoreTPPDTemplate_AllRecommendedSafePresets(t *testing.T) {
	// All 6 presets are recommended (each implements a documented
	// TOOL-003 source pattern).
	assert.ElementsMatch(t,
		SeedExpectedTPPDTemplateSlugs,
		SeedRecommendedTPPDTemplateSlugs)
}

func TestCoreTPPDTemplate_AdminReviewSubsetIsMutating(t *testing.T) {
	// Mutating builtins are the only admin-review preset (Bash/Edit/Write
	// are the destructive surface; rest are read-only or sandboxed).
	assert.Equal(t,
		[]string{"builtin-mutating-core"},
		SeedAdminReviewTPPDTemplateSlugs)
}
