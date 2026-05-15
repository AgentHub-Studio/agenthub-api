package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreSTSDTemplate_SlugsCount(t *testing.T) {
	assert.Equal(t, 3, len(SeedExpectedSTSDTemplateSlugs))
	assert.Equal(t, 3, SeedExpectedSTSDTemplateRowCount)
}

func TestCoreSTSDTemplate_SlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedSTSDTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreSTSDTemplate_SlugsAreKebabCase(t *testing.T) {
	for _, s := range SeedExpectedSTSDTemplateSlugs {
		assert.Equal(t, strings.ToLower(s), s)
		assert.NotContains(t, s, "_")
	}
}

func TestCoreSTSDTemplate_StatusesMatchOBS006Enum(t *testing.T) {
	expected := map[string]bool{
		"completed": true, "failed": true, "killed": true,
	}
	for _, s := range SeedExpectedSTSDTemplateStatuses {
		assert.True(t, expected[s], "status %q outside OBS-006 enum", s)
	}
	assert.Equal(t, 3, len(SeedExpectedSTSDTemplateStatuses))
}

func TestCoreSTSDTemplate_UseCasesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"routine_completion": true, "error_diagnosis": true, "harness_kill": true,
	}
	for _, u := range SeedExpectedSTSDTemplateUseCases {
		assert.True(t, expected[u])
	}
	assert.Equal(t, 3, len(SeedExpectedSTSDTemplateUseCases))
}

func TestCoreSTSDTemplate_EventTypesClosedSet(t *testing.T) {
	expected := map[string]bool{
		"subtask_start": true, "subtask_complete": true,
		"text_delta": true, "tool_call_start": true,
		"tool_result": true, "error": true,
	}
	for _, e := range SeedExpectedSTSDTemplateEventTypes {
		assert.True(t, expected[e])
	}
	assert.Equal(t, 6, len(SeedExpectedSTSDTemplateEventTypes))
}

func TestCoreSTSDTemplate_TenantKindsClosedSet(t *testing.T) {
	for _, k := range SeedExpectedSTSDTemplateTenantKinds {
		assert.Equal(t, "general", k)
	}
}

func TestCoreSTSDTemplate_AllPresetsRecommended(t *testing.T) {
	assert.ElementsMatch(t,
		SeedExpectedSTSDTemplateSlugs,
		SeedRecommendedSTSDTemplateSlugs)
}

func TestCoreSTSDTemplate_AdminReviewExcludesCompletedRoutine(t *testing.T) {
	// Routine completed traces are the baseline — no admin review.
	// failed/killed signal misconfiguration or runaway behavior.
	set := map[string]bool{}
	for _, s := range SeedAdminReviewSTSDTemplateSlugs {
		set[s] = true
	}
	assert.False(t, set["completed-routine-trace"])
	assert.True(t, set["failed-error-context-trace"])
	assert.True(t, set["killed-budget-or-depth-trace"])
}
