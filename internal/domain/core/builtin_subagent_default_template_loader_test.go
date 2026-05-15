package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreBSDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 7, len(SeedExpectedBSDTemplateSlugs))
	assert.Equal(t, 7, SeedExpectedBSDTemplateRowCount)
}

func TestCoreBSDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedBSDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreBSDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedBSDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
		assert.NotContains(t, s, " ")
	}
}

func TestCoreBSDTemplate_RolesMatchSUB002Enum(t *testing.T) {
	// Byte-for-byte alignment with BuiltinSubagentRole enum in
	// internal/domain/chat/agentic/builtin_subagents.go
	expected := map[string]bool{
		"researcher": true, "coder": true, "reviewer": true,
		"explorer": true, "planner": true, "curator": true,
		"documenter": true,
	}
	for _, r := range SeedExpectedBSDTemplateRoles {
		assert.True(t, expected[r], "role %q outside SUB-002 enum", r)
	}
	assert.Equal(t, 7, len(SeedExpectedBSDTemplateRoles))
}

func TestCoreBSDTemplate_RolesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range SeedExpectedBSDTemplateRoles {
		assert.False(t, seen[r])
		seen[r] = true
	}
}

func TestCoreBSDTemplate_ToolsetSlugsClosedSet(t *testing.T) {
	expected := map[string]bool{
		"documentation-readonly-allowlist": true,
		"code-write-scoped":                true,
		"docs-write-scoped":                true,
	}
	for _, s := range SeedExpectedBSDTemplateToolsetSlugs {
		assert.True(t, expected[s])
	}
}

func TestCoreBSDTemplate_InheritanceSlugsClosedSet(t *testing.T) {
	expected := map[string]bool{
		"extend-parent-rights": true,
		"restrict-to-readonly": true,
	}
	for _, s := range SeedExpectedBSDTemplateInheritanceSlugs {
		assert.True(t, expected[s])
	}
}

func TestCoreBSDTemplate_SummarySlugsClosedSet(t *testing.T) {
	expected := map[string]bool{
		"success-with-artifacts": true,
		"structured-findings":    true,
		"plan-only":              true,
	}
	for _, s := range SeedExpectedBSDTemplateSummarySlugs {
		assert.True(t, expected[s])
	}
}

func TestCoreBSDTemplate_TaskClassesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"investigation": true, "implementation": true, "review": true,
		"exploration": true, "planning": true, "curation": true,
		"documentation": true,
	}
	for _, c := range SeedExpectedBSDTemplateTaskClasses {
		assert.True(t, expected[c])
	}
}

func TestCoreBSDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedBSDTemplateSlugs,
		SeedRecommendedBSDTemplateSlugs)
}

func TestCoreBSDTemplate_AdminReviewEmpty(t *testing.T) {
	assert.Empty(t, SeedAdminReviewBSDTemplateSlugs,
		"builtin defaults should not require admin review by default")
}
