package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability webhook notification template seed constants
// (migration 000097). These run without a database and guard against
// accidental constant drift.

func TestSeedCapabilityWebhookTemplateCount_MatchesSlugList(t *testing.T) {
	assert.Equal(t, SeedCapabilityWebhookTemplateCount, len(SeedCapabilityWebhookTemplateSlugs),
		"SeedCapabilityWebhookTemplateCount must match len(SeedCapabilityWebhookTemplateSlugs)")
}

func TestSeedCapabilityWebhookTemplateSlugs_CountIsThree(t *testing.T) {
	assert.Equal(t, 3, SeedCapabilityWebhookTemplateCount,
		"migration 000097 seeds exactly 3 capability webhook templates (one per capability agent)")
	assert.Equal(t, 3, len(SeedCapabilityWebhookTemplateSlugs),
		"slug list must have exactly 3 entries matching SeedCapabilityWebhookTemplateCount")
}

func TestSeedCapabilityWebhookTemplateSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedCapabilityWebhookTemplateSlugs {
		assert.False(t, seen[slug], "duplicate capability webhook template slug %q", slug)
		seen[slug] = true
	}
}

func TestSeedCapabilityWebhookTemplateSlugs_AllNonEmpty(t *testing.T) {
	for i, slug := range SeedCapabilityWebhookTemplateSlugs {
		assert.NotEmpty(t, slug, "capability webhook template slug at index %d must be non-empty", i)
	}
}

func TestSeedCapabilityWebhookTemplateSlugs_AllStartWithCapabilityPrefix(t *testing.T) {
	for _, slug := range SeedCapabilityWebhookTemplateSlugs {
		assert.True(t, strings.HasPrefix(slug, "capability-"),
			"capability webhook template slug %q must start with 'capability-' (namespace contract)", slug)
	}
}

func TestSeedCapabilityWebhookTemplateEvents_CountIsThree(t *testing.T) {
	assert.Equal(t, 3, len(SeedCapabilityWebhookTemplateEvents),
		"3 capability event types: research_complete / analysis_done / tasks_updated")
}

func TestSeedCapabilityWebhookTemplateEvents_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, ev := range SeedCapabilityWebhookTemplateEvents {
		assert.False(t, seen[ev], "duplicate capability webhook event type %q", ev)
		seen[ev] = true
	}
}

func TestSeedCapabilityWebhookTemplateEvents_AllNonEmpty(t *testing.T) {
	for i, ev := range SeedCapabilityWebhookTemplateEvents {
		assert.NotEmpty(t, ev, "capability webhook event type at index %d must be non-empty", i)
	}
}

func TestSeedCapabilityWebhookTemplateEvents_AllSnakeCase(t *testing.T) {
	for _, ev := range SeedCapabilityWebhookTemplateEvents {
		assert.False(t, strings.Contains(ev, "-"),
			"capability webhook event type %q must use snake_case (no hyphens)", ev)
	}
}

func TestSeedResearchCompleteWebhookSlug_InSlugList(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedCapabilityWebhookTemplateSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet[SeedResearchCompleteWebhookSlug],
		"SeedResearchCompleteWebhookSlug %q must be in SeedCapabilityWebhookTemplateSlugs",
		SeedResearchCompleteWebhookSlug)
}

func TestSeedAnalysisDoneWebhookSlug_InSlugList(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedCapabilityWebhookTemplateSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet[SeedAnalysisDoneWebhookSlug],
		"SeedAnalysisDoneWebhookSlug %q must be in SeedCapabilityWebhookTemplateSlugs",
		SeedAnalysisDoneWebhookSlug)
}

func TestSeedTasksUpdatedWebhookSlug_InSlugList(t *testing.T) {
	slugSet := map[string]bool{}
	for _, s := range SeedCapabilityWebhookTemplateSlugs {
		slugSet[s] = true
	}
	assert.True(t, slugSet[SeedTasksUpdatedWebhookSlug],
		"SeedTasksUpdatedWebhookSlug %q must be in SeedCapabilityWebhookTemplateSlugs",
		SeedTasksUpdatedWebhookSlug)
}
