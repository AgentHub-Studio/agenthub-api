package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreBSLDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 4, len(SeedExpectedBSLDTemplateSlugs))
	assert.Equal(t, 4, SeedExpectedBSLDTemplateRowCount)
}

func TestCoreBSLDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedBSLDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreBSLDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedBSLDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
		assert.NotContains(t, s, " ")
	}
}

func TestCoreBSLDTemplate_TaskClassesMatchSUB002Vocabulary(t *testing.T) {
	// SUB-002 BuiltinSubagentDescriptor task_class vocabulary covers
	// investigation/planning/implementation/exploration/curation/etc.
	// Lane task classes must be a subset of that vocabulary.
	expected := map[string]bool{
		"exploration": true, "planning": true,
		"investigation": true, "implementation": true,
	}
	for _, c := range SeedExpectedBSLDTemplateTaskClasses {
		assert.True(t, expected[c], "task_class %q outside SUB-002 vocabulary", c)
	}
	assert.Equal(t, 4, len(SeedExpectedBSLDTemplateTaskClasses))
}

func TestCoreBSLDTemplate_TaskClassesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range SeedExpectedBSLDTemplateTaskClasses {
		assert.False(t, seen[c])
		seen[c] = true
	}
}

func TestCoreBSLDTemplate_PrioritiesClosedSet(t *testing.T) {
	expected := map[string]bool{"high": true, "normal": true}
	for _, p := range SeedExpectedBSLDTemplatePriorities {
		assert.True(t, expected[p])
	}
}

func TestCoreBSLDTemplate_RetryPosturesClosedSet(t *testing.T) {
	expected := map[string]bool{"none": true}
	for _, p := range SeedExpectedBSLDTemplateRetryPostures {
		assert.True(t, expected[p])
	}
}

func TestCoreBSLDTemplate_BudgetBoundsConsistent(t *testing.T) {
	assert.Less(t, SeedMinBSLDTimeoutBudgetSeconds, SeedMaxBSLDTimeoutBudgetSeconds)
	assert.GreaterOrEqual(t, SeedMinBSLDTimeoutBudgetSeconds, 30)
	assert.LessOrEqual(t, SeedMaxBSLDTimeoutBudgetSeconds, 600)
}

func TestCoreBSLDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t, SeedExpectedBSLDTemplateSlugs, SeedRecommendedBSLDTemplateSlugs)
}

func TestCoreBSLDTemplate_SingleConcurrencyLanesArePlannerAndCoder(t *testing.T) {
	// Planning and writes must be serial per parent (no divergent plans;
	// no concurrent file edits).
	assert.ElementsMatch(t,
		[]string{"planner-lane", "coder-lane"},
		SeedSingleConcurrencyBSLDTemplateSlugs)
}
