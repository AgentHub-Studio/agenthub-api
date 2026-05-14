package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreCodingAgentCategory_SlugsCount(t *testing.T) {
	assert.Equal(t, 4, len(SeedExpectedCodingAgentCategorySlugs))
	assert.Equal(t, 4, SeedExpectedCodingAgentCategoryRowCount)
}

func TestCoreCodingAgentCategory_SlugsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedCodingAgentCategorySlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreCodingAgentCategory_AllSlugsMatchRegex(t *testing.T) {
	for _, s := range SeedExpectedCodingAgentCategorySlugs {
		assert.True(t, SeedCodingAgentCategorySlugRE.MatchString(s), "slug %q must match snake_case regex", s)
	}
}

func TestCoreCodingAgentCategory_RowCountEqualsSlugCount(t *testing.T) {
	assert.Equal(t, SeedExpectedCodingAgentCategoryRowCount, len(SeedExpectedCodingAgentCategorySlugs))
}

func TestCoreCodingAgentCategory_DefaultSlugInSlugs(t *testing.T) {
	found := false
	for _, s := range SeedExpectedCodingAgentCategorySlugs {
		if s == SeedCodingAgentCategoryDefaultSlug {
			found = true
		}
	}
	assert.True(t, found, "chat_integrated must be the default AgentHub category")
}

func TestCoreCodingAgentCategory_AgentHubSlugsAreSubsetOfAll(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedCodingAgentCategorySlugs {
		all[s] = true
	}
	for _, s := range SeedCodingAgentCategoryAgentHubSlugs {
		assert.True(t, all[s], "AgentHub slug %q must be in the full slug set", s)
	}
}

func TestCoreCodingAgentCategory_AgentHubTargetCount(t *testing.T) {
	assert.Equal(t, 2, len(SeedCodingAgentCategoryAgentHubSlugs),
		"AgentHub targets exactly chat_integrated and agentic_cli")
}

func TestCoreCodingAgentCategory_GradientOrder(t *testing.T) {
	expected := []string{"inline_completion", "chat_integrated", "agentic_cli", "fully_autonomous"}
	assert.Equal(t, expected, SeedExpectedCodingAgentCategorySlugs,
		"categories must be in gradient order (passive → autonomous)")
}

func TestCoreCodingAgentCategory_ExecutionPatternsCount(t *testing.T) {
	assert.Equal(t, 4, len(SeedCodingAgentCategoryExecutionPatterns),
		"one execution pattern per category")
}

func TestCoreCodingAgentCategory_ExecutionPatternsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range SeedCodingAgentCategoryExecutionPatterns {
		assert.False(t, seen[p], "duplicate execution pattern %q", p)
		seen[p] = true
	}
}
