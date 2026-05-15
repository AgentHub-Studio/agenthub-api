package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreLLMModelSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsLLMCatalogSoAgentsCanRun", func(t *testing.T) {
		// Given a fresh tenant cannot RUN any agent without a configured
		//       LLM (PDF §4 — LLM is the engine of the agentic loop),
		// When ah_core LLM models are loaded,
		// Then ≥4 entries appear so the tenant has at least one model
		//      per provider tier ready to use.
		assert.GreaterOrEqual(t, len(SeedExpectedLLMModelSlugs), 4,
			"fresh tenant must inherit ≥4 LLM models")
	})

	t.Run("Scenario_AllFourProviderTiersAreAvailable", func(t *testing.T) {
		// Given different deployments prefer different providers
		//       (Anthropic for quality, OpenAI for ecosystem,
		//       OpenRouter for variety, Ollama for self-host),
		// When the seed providers are inspected,
		// Then all 4 tiers exist.
		set := map[string]bool{}
		for _, p := range SeedExpectedLLMModelProviders {
			set[p] = true
		}
		for _, expected := range []string{"anthropic", "openai", "openrouter", "ollama"} {
			assert.True(t, set[expected], "provider %q must be in seed", expected)
		}
	})

	t.Run("Scenario_SlugsCarryProviderPrefixForRoutingDispatch", func(t *testing.T) {
		// Given the runner dispatches to provider clients by parsing
		//       the slug prefix (no separate provider lookup needed),
		// When the seed slugs are inspected,
		// Then every slug starts with "<provider>/".
		for _, s := range SeedExpectedLLMModelSlugs {
			idx := strings.Index(s, "/")
			assert.Greater(t, idx, 0, "slug %q must use provider/model format", s)
		}
	})

	t.Run("Scenario_RecommendedSetGuidesUIPicker", func(t *testing.T) {
		// Given the agent-create UI shows a "Recommended" section
		//       to spare users from picking among 7+ models,
		// When the recommended set is inspected,
		// Then exactly one model per provider is recommended.
		seenProvider := map[string]bool{}
		for _, slug := range SeedRecommendedLLMModelSlugs {
			provider := slug[:strings.Index(slug, "/")]
			assert.False(t, seenProvider[provider],
				"provider %q has more than one recommended", provider)
			seenProvider[provider] = true
		}
		assert.Equal(t, len(SeedExpectedLLMModelProviders), len(SeedRecommendedLLMModelSlugs),
			"one recommended per provider — covers all tiers")
	})

	t.Run("Scenario_FreeAndPaidTiersAreBothPresent", func(t *testing.T) {
		// Given budget-sensitive tenants need a free option
		//       (ollama local) AND quality-sensitive tenants need a
		//       paid one (claude-sonnet),
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedLLMModelSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["ollama/llama3.2"], "free local option required")
		assert.True(t, seedSet["anthropic/claude-sonnet-4-6"], "high-quality paid option required")
	})

	t.Run("Scenario_BDDValidatedModelIsRecommendedForOpenRouter", func(t *testing.T) {
		// Given mistral-nemo is the BDD-validated stable OpenRouter
		//       model (memory: free :free models are unstable),
		// When the OpenRouter recommended slug is inspected,
		// Then it is mistral-nemo — protects test infrastructure
		//      from picking unreliable models by default.
		recSet := map[string]bool{}
		for _, s := range SeedRecommendedLLMModelSlugs {
			recSet[s] = true
		}
		assert.True(t, recSet["openrouter/mistralai/mistral-nemo"],
			"OpenRouter recommended = mistral-nemo (BDD-validated)")
	})

	t.Run("Scenario_ToolSupportEnablesAgenticLoop", func(t *testing.T) {
		// Given PDF §6.1 — agentic loop requires tool_calls support,
		//       a model without tool support is useless for AgentHub,
		// When the seed contract is inspected (integration validates DB),
		// Then ALL seed models support tools (tested in integration).
		// Constant-level proxy: the count = SeedExpectedLLMModelSlugs
		//                       and the integration test asserts
		//                       supports_tools=true for each.
		assert.NotEmpty(t, SeedExpectedLLMModelSlugs)
	})

	t.Run("Scenario_LocalOllamaCostIsZeroForFreeTier", func(t *testing.T) {
		// Given Ollama runs locally → cost = 0,
		// When the seed catalog is inspected (integration validates),
		// Then the ollama entry exists. Integration test asserts
		//      cost_per_1m_*=0 in DB.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedLLMModelSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["ollama/llama3.2"],
			"ollama entry required as free tier representative")
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		// Given external systems (UI catalog, billing dashboards)
		//       bind to count = 7,
		assert.Equal(t, 7, len(SeedExpectedLLMModelSlugs))
	})

	t.Run("Scenario_EveryRecommendedSlugIsAlsoInSeed", func(t *testing.T) {
		// Given the recommended subset cannot reference removed models,
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedLLMModelSlugs {
			seedSet[s] = true
		}
		for _, r := range SeedRecommendedLLMModelSlugs {
			assert.True(t, seedSet[r],
				"recommended %q must exist in canonical seed", r)
		}
	})
}
