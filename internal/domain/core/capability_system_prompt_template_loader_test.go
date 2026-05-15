package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability system prompt template seed constants (migration 000100).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilitySystemPromptTemplateCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedCapabilitySystemPromptTemplateCount,
		"migration 000100 seeds exactly 3 capability system prompt templates (one per capability agent)")
}

func TestSeedCapabilitySystemPromptTemplateSlugs_HasLengthThree(t *testing.T) {
	assert.Len(t, SeedCapabilitySystemPromptTemplateSlugs, 3,
		"SeedCapabilitySystemPromptTemplateSlugs must have exactly 3 entries — one per capability agent")
}

func TestSeedCapabilitySystemPromptTemplateSlugs_MatchesCount(t *testing.T) {
	assert.Equal(t, SeedCapabilitySystemPromptTemplateCount, len(SeedCapabilitySystemPromptTemplateSlugs),
		"SeedCapabilitySystemPromptTemplateCount must match len(SeedCapabilitySystemPromptTemplateSlugs)")
}

func TestSeedCapabilitySystemPromptTemplateSlugs_ContainsResearcher(t *testing.T) {
	assert.Contains(t, SeedCapabilitySystemPromptTemplateSlugs, "capability-researcher-system-prompt",
		"SeedCapabilitySystemPromptTemplateSlugs must contain capability-researcher-system-prompt (migration 000100)")
}

func TestSeedCapabilitySystemPromptTemplateSlugs_ContainsAnalyst(t *testing.T) {
	assert.Contains(t, SeedCapabilitySystemPromptTemplateSlugs, "capability-analyst-system-prompt",
		"SeedCapabilitySystemPromptTemplateSlugs must contain capability-analyst-system-prompt (migration 000100)")
}

func TestSeedCapabilitySystemPromptTemplateSlugs_ContainsPlanner(t *testing.T) {
	assert.Contains(t, SeedCapabilitySystemPromptTemplateSlugs, "capability-planner-system-prompt",
		"SeedCapabilitySystemPromptTemplateSlugs must contain capability-planner-system-prompt (migration 000100)")
}

func TestSeedCapabilitySystemPromptTemplateSlugs_AllStartWithCapabilityPrefix(t *testing.T) {
	for _, slug := range SeedCapabilitySystemPromptTemplateSlugs {
		assert.True(t, strings.HasPrefix(slug, "capability-"),
			"capability system prompt slug %q must start with 'capability-' (capability-layer namespace contract)",
			slug)
	}
}

func TestSeedCapabilitySystemPromptTemplateSlugs_AllEndWithSystemPromptSuffix(t *testing.T) {
	for _, slug := range SeedCapabilitySystemPromptTemplateSlugs {
		assert.True(t, strings.HasSuffix(slug, "-system-prompt"),
			"capability system prompt slug %q must end with '-system-prompt' (naming convention)",
			slug)
	}
}

func TestSeedCapabilitySystemPromptAgentMap_HasThreeEntries(t *testing.T) {
	assert.Len(t, SeedCapabilitySystemPromptAgentMap, 3,
		"SeedCapabilitySystemPromptAgentMap must have exactly 3 entries — one per capability agent")
}

func TestSeedCapabilitySystemPromptAgentMap_AllValuesStartWithCorePrefix(t *testing.T) {
	for slug, agentSlug := range SeedCapabilitySystemPromptAgentMap {
		assert.True(t, strings.HasPrefix(agentSlug, "core-"),
			"agent slug %q mapped from prompt slug %q must start with 'core-' (capability-layer namespace contract)",
			agentSlug, slug)
	}
}

func TestSeedCapabilitySystemPromptAgentMap_ResearcherMapsToCorePesearcher(t *testing.T) {
	assert.Equal(t, "core-researcher", SeedCapabilitySystemPromptAgentMap["capability-researcher-system-prompt"],
		"capability-researcher-system-prompt must map to core-researcher in SeedCapabilitySystemPromptAgentMap")
}

func TestSeedCapabilitySystemPromptAgentMap_AnalystMapsToCoreAnalyst(t *testing.T) {
	assert.Equal(t, "core-analyst", SeedCapabilitySystemPromptAgentMap["capability-analyst-system-prompt"],
		"capability-analyst-system-prompt must map to core-analyst in SeedCapabilitySystemPromptAgentMap")
}

func TestSeedCapabilitySystemPromptAgentMap_PlannerMapsToCorePlanner(t *testing.T) {
	assert.Equal(t, "core-planner", SeedCapabilitySystemPromptAgentMap["capability-planner-system-prompt"],
		"capability-planner-system-prompt must map to core-planner in SeedCapabilitySystemPromptAgentMap")
}

func TestSeedCapabilitySystemPromptAgentMap_AllKeysArePresentInSlugs(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedCapabilitySystemPromptTemplateSlugs {
		slugSet[s] = true
	}
	for key := range SeedCapabilitySystemPromptAgentMap {
		assert.True(t, slugSet[key],
			"SeedCapabilitySystemPromptAgentMap key %q must appear in SeedCapabilitySystemPromptTemplateSlugs",
			key)
	}
}
