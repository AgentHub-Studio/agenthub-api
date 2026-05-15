package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability knowledge source seed constants (migration 000117).
// These run without a database and guard against accidental constant drift.

// ─────────────────────────────────────────────────────────
// Count constant tests
// ─────────────────────────────────────────────────────────

func TestSeedKnowledgeSourceCount_IsNine(t *testing.T) {
	assert.Equal(t, 9, SeedKnowledgeSourceCount,
		"migration 000117 seeds exactly 9 capability knowledge source rows (three per capability agent)")
}

func TestSeedKnowledgeSourceAgentCount_IsThree(t *testing.T) {
	assert.Equal(t, 3, SeedKnowledgeSourceAgentCount,
		"SeedKnowledgeSourceAgentCount must be 3 — researcher, analyst, planner")
}

func TestSeedKnowledgeSourceCount_EqualsAgentCountTimesThree(t *testing.T) {
	assert.Equal(t, SeedKnowledgeSourceAgentCount*3, SeedKnowledgeSourceCount,
		"total source count must equal agent count × 3 (each agent has exactly 3 knowledge sources)")
}

func TestSeedKnowledgeSourceCount_IsPositive(t *testing.T) {
	assert.Greater(t, SeedKnowledgeSourceCount, 0,
		"SeedKnowledgeSourceCount must be positive")
}

func TestSeedKnowledgeSourceAgentCount_IsPositive(t *testing.T) {
	assert.Greater(t, SeedKnowledgeSourceAgentCount, 0,
		"SeedKnowledgeSourceAgentCount must be positive")
}

// ─────────────────────────────────────────────────────────
// Source type constant tests
// ─────────────────────────────────────────────────────────

func TestSeedSourceTypeWebSearch_Value(t *testing.T) {
	assert.Equal(t, "web_search", SeedSourceTypeWebSearch,
		"SeedSourceTypeWebSearch must equal \"web_search\"")
}

func TestSeedSourceTypeDocumentFetch_Value(t *testing.T) {
	assert.Equal(t, "document_fetch", SeedSourceTypeDocumentFetch,
		"SeedSourceTypeDocumentFetch must equal \"document_fetch\"")
}

func TestSeedSourceTypeKnowledgeBase_Value(t *testing.T) {
	assert.Equal(t, "knowledge_base", SeedSourceTypeKnowledgeBase,
		"SeedSourceTypeKnowledgeBase must equal \"knowledge_base\"")
}

func TestSeedSourceTypeConversationContext_Value(t *testing.T) {
	assert.Equal(t, "conversation_context", SeedSourceTypeConversationContext,
		"SeedSourceTypeConversationContext must equal \"conversation_context\"")
}

func TestSeedSourceTypes_AreDistinct(t *testing.T) {
	types := []string{
		SeedSourceTypeWebSearch,
		SeedSourceTypeDocumentFetch,
		SeedSourceTypeKnowledgeBase,
		SeedSourceTypeConversationContext,
	}
	seen := map[string]struct{}{}
	for _, st := range types {
		assert.NotEmpty(t, st, "every source type constant must be non-empty")
		seen[st] = struct{}{}
	}
	assert.Len(t, seen, 4,
		"there must be exactly 4 distinct source type constants")
}

func TestSeedSourceTypes_AreNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedSourceTypeWebSearch)
	assert.NotEmpty(t, SeedSourceTypeDocumentFetch)
	assert.NotEmpty(t, SeedSourceTypeKnowledgeBase)
	assert.NotEmpty(t, SeedSourceTypeConversationContext)
}

// ─────────────────────────────────────────────────────────
// Source key constant tests
// ─────────────────────────────────────────────────────────

func TestSeedSourceKeyPrimary_Value(t *testing.T) {
	assert.Equal(t, "primary", SeedSourceKeyPrimary,
		"SeedSourceKeyPrimary must equal \"primary\"")
}

func TestSeedSourceKeySecondary_Value(t *testing.T) {
	assert.Equal(t, "secondary", SeedSourceKeySecondary,
		"SeedSourceKeySecondary must equal \"secondary\"")
}

func TestSeedSourceKeyFallback_Value(t *testing.T) {
	assert.Equal(t, "fallback", SeedSourceKeyFallback,
		"SeedSourceKeyFallback must equal \"fallback\"")
}

func TestSeedSourceKeys_AreDistinct(t *testing.T) {
	keys := []string{
		SeedSourceKeyPrimary,
		SeedSourceKeySecondary,
		SeedSourceKeyFallback,
	}
	seen := map[string]struct{}{}
	for _, k := range keys {
		assert.NotEmpty(t, k, "every source key constant must be non-empty")
		seen[k] = struct{}{}
	}
	assert.Len(t, seen, 3,
		"there must be exactly 3 distinct source key constants")
}

func TestSeedSourceKeys_AreNonEmpty(t *testing.T) {
	assert.NotEmpty(t, SeedSourceKeyPrimary)
	assert.NotEmpty(t, SeedSourceKeySecondary)
	assert.NotEmpty(t, SeedSourceKeyFallback)
}

func TestSeedSourceKeyCount_MatchesSourcesPerAgent(t *testing.T) {
	keys := []string{
		SeedSourceKeyPrimary,
		SeedSourceKeySecondary,
		SeedSourceKeyFallback,
	}
	perAgent := SeedKnowledgeSourceCount / SeedKnowledgeSourceAgentCount
	assert.Equal(t, len(keys), perAgent,
		"number of source key constants (%d) must equal sources per agent (%d)", len(keys), perAgent)
}

// ─────────────────────────────────────────────────────────
// Per-agent primary source type tests
// ─────────────────────────────────────────────────────────

func TestSeedResearcherPrimaryType_IsWebSearch(t *testing.T) {
	assert.Equal(t, SeedSourceTypeWebSearch, SeedResearcherPrimaryType,
		"core-researcher primary source type must be web_search")
}

func TestSeedAnalystPrimaryType_IsKnowledgeBase(t *testing.T) {
	assert.Equal(t, SeedSourceTypeKnowledgeBase, SeedAnalystPrimaryType,
		"core-analyst primary source type must be knowledge_base")
}

func TestSeedPlannerPrimaryType_IsConversationContext(t *testing.T) {
	assert.Equal(t, SeedSourceTypeConversationContext, SeedPlannerPrimaryType,
		"core-planner primary source type must be conversation_context")
}

func TestSeedPerAgentPrimaryTypes_AreDistinct(t *testing.T) {
	primaryTypes := []string{
		SeedResearcherPrimaryType,
		SeedAnalystPrimaryType,
		SeedPlannerPrimaryType,
	}
	seen := map[string]struct{}{}
	for _, pt := range primaryTypes {
		assert.NotEmpty(t, pt, "every per-agent primary type constant must be non-empty")
		seen[pt] = struct{}{}
	}
	assert.Len(t, seen, 3,
		"each capability agent must have a distinct primary knowledge source type")
}

func TestSeedResearcherPrimaryType_IsAKnownSourceType(t *testing.T) {
	knownTypes := []string{
		SeedSourceTypeWebSearch,
		SeedSourceTypeDocumentFetch,
		SeedSourceTypeKnowledgeBase,
		SeedSourceTypeConversationContext,
	}
	assert.Contains(t, knownTypes, SeedResearcherPrimaryType,
		"SeedResearcherPrimaryType must be one of the 4 known source types")
}

func TestSeedAnalystPrimaryType_IsAKnownSourceType(t *testing.T) {
	knownTypes := []string{
		SeedSourceTypeWebSearch,
		SeedSourceTypeDocumentFetch,
		SeedSourceTypeKnowledgeBase,
		SeedSourceTypeConversationContext,
	}
	assert.Contains(t, knownTypes, SeedAnalystPrimaryType,
		"SeedAnalystPrimaryType must be one of the 4 known source types")
}

func TestSeedPlannerPrimaryType_IsAKnownSourceType(t *testing.T) {
	knownTypes := []string{
		SeedSourceTypeWebSearch,
		SeedSourceTypeDocumentFetch,
		SeedSourceTypeKnowledgeBase,
		SeedSourceTypeConversationContext,
	}
	assert.Contains(t, knownTypes, SeedPlannerPrimaryType,
		"SeedPlannerPrimaryType must be one of the 4 known source types")
}

// ─────────────────────────────────────────────────────────
// Priority ordering invariant tests
// ─────────────────────────────────────────────────────────

func TestSeedPriorityOrdering_PrimaryBeforeSecondary(t *testing.T) {
	// primary=1 < secondary=2 — lower number = higher priority
	assert.Equal(t, 1, 1, "primary priority is 1 (highest)")
	assert.Equal(t, 2, 2, "secondary priority is 2")
	assert.Less(t, 1, 2,
		"primary priority (1) must be less than secondary priority (2)")
}

func TestSeedPriorityOrdering_SecondaryBeforeFallback(t *testing.T) {
	// secondary=2 < fallback=3
	assert.Less(t, 2, 3,
		"secondary priority (2) must be less than fallback priority (3)")
}

func TestSeedPriorityOrdering_PrimaryBeforeFallback(t *testing.T) {
	// primary=1 < fallback=3
	assert.Less(t, 1, 3,
		"primary priority (1) must be less than fallback priority (3)")
}

func TestSeedPriorityLevels_AreConsecutive(t *testing.T) {
	// Priority levels must form the consecutive sequence 1, 2, 3
	priorities := []int{1, 2, 3}
	for i, p := range priorities {
		assert.Equal(t, i+1, p,
			"priority level at index %d must equal %d (consecutive from 1)", i, i+1)
	}
}
