package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability agent tag seed constants (migration 000109).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityAgentTagCount_IsTwelve(t *testing.T) {
	assert.Equal(t, 12, SeedCapabilityAgentTagCount,
		"migration 000109 seeds exactly 12 capability agent tag rows (four per capability agent)")
}

func TestSeedCapabilityAgentTagAgentSlugs_HasThreeEntries(t *testing.T) {
	assert.Len(t, SeedCapabilityAgentTagAgentSlugs, 3,
		"SeedCapabilityAgentTagAgentSlugs must have exactly 3 entries (researcher, analyst, planner)")
}

func TestSeedResearcherTagCount_IsFour(t *testing.T) {
	assert.Equal(t, 4, SeedResearcherTagCount,
		"SeedResearcherTagCount must be 4 — research/web/sources/knowledge")
}

func TestSeedAnalystTagCount_IsFour(t *testing.T) {
	assert.Equal(t, 4, SeedAnalystTagCount,
		"SeedAnalystTagCount must be 4 — analysis/documents/insights/patterns")
}

func TestSeedPlannerTagCount_IsFour(t *testing.T) {
	assert.Equal(t, 4, SeedPlannerTagCount,
		"SeedPlannerTagCount must be 4 — planning/tasks/projects/workflows")
}

func TestSeedPerAgentTagCounts_SumToTotal(t *testing.T) {
	sum := SeedResearcherTagCount + SeedAnalystTagCount + SeedPlannerTagCount
	assert.Equal(t, SeedCapabilityAgentTagCount, sum,
		"researcher (%d) + analyst (%d) + planner (%d) must equal total count (%d)",
		SeedResearcherTagCount, SeedAnalystTagCount, SeedPlannerTagCount,
		SeedCapabilityAgentTagCount)
}

func TestSeedResearcherTags_HasFourEntries(t *testing.T) {
	assert.Len(t, SeedResearcherTags, 4,
		"SeedResearcherTags must have exactly 4 entries")
}

func TestSeedAnalystTags_HasFourEntries(t *testing.T) {
	assert.Len(t, SeedAnalystTags, 4,
		"SeedAnalystTags must have exactly 4 entries")
}

func TestSeedPlannerTags_HasFourEntries(t *testing.T) {
	assert.Len(t, SeedPlannerTags, 4,
		"SeedPlannerTags must have exactly 4 entries")
}

func TestSeedCapabilityAgentAllTags_HasTwelveEntries(t *testing.T) {
	assert.Len(t, SeedCapabilityAgentAllTags, 12,
		"SeedCapabilityAgentAllTags must have exactly 12 entries (all agents combined)")
}

func TestSeedCapabilityAgentAllTags_AllNonEmptyLowercase(t *testing.T) {
	for _, tag := range SeedCapabilityAgentAllTags {
		assert.NotEmpty(t, tag, "every tag must be non-empty")
		assert.Equal(t, strings.ToLower(tag), tag,
			"tag %q must be lowercase", tag)
	}
}

func TestSeedCapabilityAgentAllTags_AllTwelveAreDistinct(t *testing.T) {
	unique := map[string]struct{}{}
	for _, tag := range SeedCapabilityAgentAllTags {
		unique[tag] = struct{}{}
	}
	assert.Len(t, unique, 12,
		"all 12 tags must be distinct across agents — no duplicates")
}

func TestSeedResearcherTags_ContainsResearch(t *testing.T) {
	assert.Contains(t, SeedResearcherTags, "research",
		"SeedResearcherTags must contain 'research'")
}

func TestSeedPlannerTags_ContainsTasks(t *testing.T) {
	assert.Contains(t, SeedPlannerTags, "tasks",
		"SeedPlannerTags must contain 'tasks'")
}

func TestSeedAnalystTags_ContainsDocuments(t *testing.T) {
	assert.Contains(t, SeedAnalystTags, "documents",
		"SeedAnalystTags must contain 'documents'")
}

func TestSeedCapabilityAgentAllTags_NoTagAppearsInMoreThanOneAgent(t *testing.T) {
	// Build per-agent sets.
	researcherSet := map[string]struct{}{}
	for _, tag := range SeedResearcherTags {
		researcherSet[tag] = struct{}{}
	}
	analystSet := map[string]struct{}{}
	for _, tag := range SeedAnalystTags {
		analystSet[tag] = struct{}{}
	}
	plannerSet := map[string]struct{}{}
	for _, tag := range SeedPlannerTags {
		plannerSet[tag] = struct{}{}
	}

	// Check every analyst tag is absent from researcher and planner.
	for _, tag := range SeedAnalystTags {
		_, inResearcher := researcherSet[tag]
		_, inPlanner := plannerSet[tag]
		assert.False(t, inResearcher, "analyst tag %q must not appear in SeedResearcherTags", tag)
		assert.False(t, inPlanner, "analyst tag %q must not appear in SeedPlannerTags", tag)
	}

	// Check every planner tag is absent from researcher and analyst.
	for _, tag := range SeedPlannerTags {
		_, inResearcher := researcherSet[tag]
		_, inAnalyst := analystSet[tag]
		assert.False(t, inResearcher, "planner tag %q must not appear in SeedResearcherTags", tag)
		assert.False(t, inAnalyst, "planner tag %q must not appear in SeedAnalystTags", tag)
	}

	// Check every researcher tag is absent from analyst and planner.
	for _, tag := range SeedResearcherTags {
		_, inAnalyst := analystSet[tag]
		_, inPlanner := plannerSet[tag]
		assert.False(t, inAnalyst, "researcher tag %q must not appear in SeedAnalystTags", tag)
		assert.False(t, inPlanner, "researcher tag %q must not appear in SeedPlannerTags", tag)
	}
}
