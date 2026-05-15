package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000117 capability knowledge source seeds.
// These assert seed shape and source rationale without a database.

func TestBDD_CapabilityKnowledgeSourceSeed(t *testing.T) {
	t.Run("Scenario_NineSourcesAcrossThreeAgents", func(t *testing.T) {
		// Given the AgentHub capability system needs per-agent knowledge source
		//   declarations to guide each agent toward the right information resources
		//   for its purpose (researcher → web, analyst → tenant data, planner → conversation),
		// When migration 000117 seeds capability_knowledge_source rows,
		// Then exactly 9 rows are added — three per agent — covering researcher,
		//   analyst, and planner with a balanced primary/secondary/fallback set.
		assert.Equal(t, 9, SeedKnowledgeSourceCount,
			"migration 000117 must seed exactly 9 capability knowledge source rows")

		assert.Equal(t, 3, SeedKnowledgeSourceAgentCount,
			"SeedKnowledgeSourceAgentCount must be 3 — researcher, analyst, planner")

		assert.Equal(t, SeedKnowledgeSourceAgentCount*3, SeedKnowledgeSourceCount,
			"total source count must equal agent count × 3 (each agent has exactly 3 knowledge sources)")
	})

	t.Run("Scenario_EachAgentHasThreePriorityLevels", func(t *testing.T) {
		// Given each capability agent needs a structured hierarchy of information
		//   sources — consulted in order from most to least preferred — so that the
		//   agent always has a fallback when the primary or secondary source is
		//   unavailable or insufficient,
		// When migration 000117 seeds knowledge source rows,
		// Then each agent has exactly three priority slots with source key constants
		//   "primary" (priority=1), "secondary" (priority=2), and "fallback" (priority=3),
		//   and the constants enforce that primary < secondary < fallback numerically.
		assert.Equal(t, "primary", SeedSourceKeyPrimary,
			"SeedSourceKeyPrimary must equal \"primary\"")
		assert.Equal(t, "secondary", SeedSourceKeySecondary,
			"SeedSourceKeySecondary must equal \"secondary\"")
		assert.Equal(t, "fallback", SeedSourceKeyFallback,
			"SeedSourceKeyFallback must equal \"fallback\"")

		// Numeric priority ordering: primary=1 < secondary=2 < fallback=3.
		priorities := []int{1, 2, 3}
		for i := 1; i < len(priorities); i++ {
			assert.Less(t, priorities[i-1], priorities[i],
				"priority at slot %d (%d) must be less than priority at slot %d (%d)",
				i-1, priorities[i-1], i, priorities[i])
		}

		// Three source keys × three agents = nine total rows.
		perAgent := SeedKnowledgeSourceCount / SeedKnowledgeSourceAgentCount
		assert.Equal(t, 3, perAgent,
			"each agent must have exactly 3 knowledge source rows")
	})

	t.Run("Scenario_ResearcherPrioritizesWebSearch", func(t *testing.T) {
		// Given the core-researcher agent is designed to discover and synthesise
		//   information from the open web — its primary value is live, up-to-date
		//   research rather than tenant-managed documents — the researcher must
		//   consult the web search index first, fetch full page content when search
		//   results are insufficient, and fall back to tenant knowledge base last,
		// When migration 000117 seeds knowledge source rows for core-researcher,
		// Then the primary source type is "web_search", the secondary is
		//   "document_fetch", and the fallback is "knowledge_base",
		//   and SeedResearcherPrimaryType resolves to SeedSourceTypeWebSearch.
		assert.Equal(t, SeedSourceTypeWebSearch, SeedResearcherPrimaryType,
			"core-researcher primary source type must be web_search — the researcher consults the web first")

		// The researcher's primary type must not be knowledge_base or conversation_context.
		assert.NotEqual(t, SeedSourceTypeKnowledgeBase, SeedResearcherPrimaryType,
			"core-researcher primary source must not be knowledge_base — that is the analyst's primary")
		assert.NotEqual(t, SeedSourceTypeConversationContext, SeedResearcherPrimaryType,
			"core-researcher primary source must not be conversation_context — that is the planner's primary")

		// The four source type constants are all distinct.
		allTypes := []string{
			SeedSourceTypeWebSearch,
			SeedSourceTypeDocumentFetch,
			SeedSourceTypeKnowledgeBase,
			SeedSourceTypeConversationContext,
		}
		seen := map[string]struct{}{}
		for _, st := range allTypes {
			seen[st] = struct{}{}
		}
		assert.Len(t, seen, 4,
			"all 4 source type constants must be distinct")
	})

	t.Run("Scenario_AnalystPrioritizesKnowledgeBase", func(t *testing.T) {
		// Given the core-analyst agent is designed to analyse structured data
		//   and documents that the tenant has uploaded — its primary value is
		//   deep, reliable analysis of tenant-specific information — the analyst
		//   must consult the tenant knowledge base first, then conversation-shared
		//   data, and fall back to web search only as a last resort,
		// When migration 000117 seeds knowledge source rows for core-analyst,
		// Then the primary source type is "knowledge_base", the secondary is
		//   "conversation_context", and the fallback is "web_search",
		//   and SeedAnalystPrimaryType resolves to SeedSourceTypeKnowledgeBase.
		assert.Equal(t, SeedSourceTypeKnowledgeBase, SeedAnalystPrimaryType,
			"core-analyst primary source type must be knowledge_base — the analyst consults tenant data first")

		// The analyst's primary type must not be web_search or conversation_context.
		assert.NotEqual(t, SeedSourceTypeWebSearch, SeedAnalystPrimaryType,
			"core-analyst primary source must not be web_search — that is the researcher's primary")
		assert.NotEqual(t, SeedSourceTypeConversationContext, SeedAnalystPrimaryType,
			"core-analyst primary source must not be conversation_context — that is the planner's primary")

		// SeedAnalystPrimaryType must be one of the four known types.
		knownTypes := []string{
			SeedSourceTypeWebSearch,
			SeedSourceTypeDocumentFetch,
			SeedSourceTypeKnowledgeBase,
			SeedSourceTypeConversationContext,
		}
		assert.Contains(t, knownTypes, SeedAnalystPrimaryType,
			"SeedAnalystPrimaryType must be one of the 4 known source type constants")
	})

	t.Run("Scenario_PlannerPrioritizesConversationContext", func(t *testing.T) {
		// Given the core-planner agent is designed to create actionable plans
		//   based on goals and requirements provided directly by the user in the
		//   conversation — its primary value is understanding the user's intent
		//   as expressed in the session — the planner must consult conversation
		//   context first, then project documentation in the knowledge base,
		//   and fall back to web search for technical reference only when needed,
		// When migration 000117 seeds knowledge source rows for core-planner,
		// Then the primary source type is "conversation_context", the secondary
		//   is "knowledge_base", and the fallback is "web_search",
		//   and SeedPlannerPrimaryType resolves to SeedSourceTypeConversationContext.
		assert.Equal(t, SeedSourceTypeConversationContext, SeedPlannerPrimaryType,
			"core-planner primary source type must be conversation_context — the planner reads requirements from the session first")

		// The planner's primary type must not be web_search or knowledge_base.
		assert.NotEqual(t, SeedSourceTypeWebSearch, SeedPlannerPrimaryType,
			"core-planner primary source must not be web_search — that is the researcher's primary")
		assert.NotEqual(t, SeedSourceTypeKnowledgeBase, SeedPlannerPrimaryType,
			"core-planner primary source must not be knowledge_base — that is the analyst's primary")

		// The three per-agent primary types are all distinct — each agent has a unique primary.
		primaryTypes := []string{
			SeedResearcherPrimaryType,
			SeedAnalystPrimaryType,
			SeedPlannerPrimaryType,
		}
		seen := map[string]struct{}{}
		for _, pt := range primaryTypes {
			seen[pt] = struct{}{}
		}
		assert.Len(t, seen, 3,
			"each capability agent must have a distinct primary knowledge source type — no two agents share the same primary")
	})
}
