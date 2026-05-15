package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability agent–webhook binding seed constants (migration 000099).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityAgentWebhookBindingCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedCapabilityAgentWebhookBindingCount,
		"migration 000099 seeds exactly 3 capability agent–webhook bindings (one per capability agent)")
}

func TestSeedCapabilityAgentWebhookBindings_HasLengthThree(t *testing.T) {
	assert.Len(t, SeedCapabilityAgentWebhookBindings, 3,
		"SeedCapabilityAgentWebhookBindings must have exactly 3 entries — one per capability agent")
}

func TestSeedCapabilityAgentWebhookBindings_MatchesCount(t *testing.T) {
	assert.Equal(t, SeedCapabilityAgentWebhookBindingCount, len(SeedCapabilityAgentWebhookBindings),
		"SeedCapabilityAgentWebhookBindingCount must match len(SeedCapabilityAgentWebhookBindings)")
}

func TestSeedCapabilityAgentWebhookBindings_AllHaveNonEmptyAgentSlug(t *testing.T) {
	for _, b := range SeedCapabilityAgentWebhookBindings {
		assert.NotEmpty(t, b.AgentSlug,
			"every capability agent–webhook binding must have a non-empty AgentSlug")
	}
}

func TestSeedCapabilityAgentWebhookBindings_AllHaveNonEmptyWebhookSlug(t *testing.T) {
	for _, b := range SeedCapabilityAgentWebhookBindings {
		assert.NotEmpty(t, b.WebhookSlug,
			"every capability agent–webhook binding must have a non-empty WebhookSlug")
	}
}

func TestSeedCapabilityAgentWebhookBindings_ResearcherBoundToResearchCompleteWebhook(t *testing.T) {
	found := false
	for _, b := range SeedCapabilityAgentWebhookBindings {
		if b.AgentSlug == "core-researcher" && b.WebhookSlug == "capability-research-complete" {
			found = true
			break
		}
	}
	assert.True(t, found,
		"core-researcher must be bound to capability-research-complete webhook (migration 000099)")
}

func TestSeedCapabilityAgentWebhookBindings_AnalystBoundToAnalysisDoneWebhook(t *testing.T) {
	found := false
	for _, b := range SeedCapabilityAgentWebhookBindings {
		if b.AgentSlug == "core-analyst" && b.WebhookSlug == "capability-analysis-done" {
			found = true
			break
		}
	}
	assert.True(t, found,
		"core-analyst must be bound to capability-analysis-done webhook (migration 000099)")
}

func TestSeedCapabilityAgentWebhookBindings_PlannerBoundToTasksUpdatedWebhook(t *testing.T) {
	found := false
	for _, b := range SeedCapabilityAgentWebhookBindings {
		if b.AgentSlug == "core-planner" && b.WebhookSlug == "capability-tasks-updated" {
			found = true
			break
		}
	}
	assert.True(t, found,
		"core-planner must be bound to capability-tasks-updated webhook (migration 000099)")
}

func TestSeedCapabilityAgentWebhookBindings_AllAgentSlugsStartWithCorePrefix(t *testing.T) {
	for _, b := range SeedCapabilityAgentWebhookBindings {
		assert.True(t, strings.HasPrefix(b.AgentSlug, "core-"),
			"capability agent slug %q must start with 'core-' (capability-layer namespace contract)",
			b.AgentSlug)
	}
}

func TestSeedCapabilityAgentWebhookBindings_AllWebhookSlugsStartWithCapabilityPrefix(t *testing.T) {
	for _, b := range SeedCapabilityAgentWebhookBindings {
		assert.True(t, strings.HasPrefix(b.WebhookSlug, "capability-"),
			"capability webhook slug %q must start with 'capability-' (capability-layer namespace contract)",
			b.WebhookSlug)
	}
}

func TestSeedCapabilityAgentWebhookBindings_NoDuplicatePairs(t *testing.T) {
	seen := map[string]bool{}
	for _, b := range SeedCapabilityAgentWebhookBindings {
		key := b.AgentSlug + "→" + b.WebhookSlug
		assert.False(t, seen[key],
			"duplicate capability agent–webhook binding pair %q", key)
		seen[key] = true
	}
}

func TestSeedCapabilityAgentWebhookBindings_NoDuplicateAgentSlugs(t *testing.T) {
	seen := map[string]bool{}
	for _, b := range SeedCapabilityAgentWebhookBindings {
		assert.False(t, seen[b.AgentSlug],
			"duplicate agent slug %q in SeedCapabilityAgentWebhookBindings — each capability agent must appear at most once",
			b.AgentSlug)
		seen[b.AgentSlug] = true
	}
}

func TestSeedCapabilityAgentWebhookBindingLabels_MatchExpectedFormat(t *testing.T) {
	assert.Equal(t, "core-researcher→capability-research-complete", SeedResearcherWebhookBinding,
		"SeedResearcherWebhookBinding must use '→' separator and match the seeded pair exactly")
	assert.Equal(t, "core-analyst→capability-analysis-done", SeedAnalystWebhookBinding,
		"SeedAnalystWebhookBinding must use '→' separator and match the seeded pair exactly")
	assert.Equal(t, "core-planner→capability-tasks-updated", SeedPlannerWebhookBinding,
		"SeedPlannerWebhookBinding must use '→' separator and match the seeded pair exactly")
}
