package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreEmbeddingProviderSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsEmbeddingProviderCatalogForRAG", func(t *testing.T) {
		// Given a fresh tenant cannot index documents without a configured
		//       embedding provider (PDF §4 — RAG pipeline retrieval),
		// When ah_core embedding providers are loaded,
		// Then ≥3 entries appear so the tenant has cost/quality options.
		assert.GreaterOrEqual(t, len(SeedExpectedEmbeddingProviderSlugs), 3,
			"fresh tenant must inherit ≥3 embedding providers")
	})

	t.Run("Scenario_LocalE5LargeIsThePlatformDefault", func(t *testing.T) {
		// Given CLAUDE.md says agenthub-embedding E5-Large 1024dim is
		//       the historical AgentHub default (zero external cost,
		//       runs in-cluster),
		// When the platform default is inspected,
		// Then it is local/e5-large.
		assert.Equal(t, "local/e5-large", SeedDefaultEmbeddingProviderSlug)
	})

	t.Run("Scenario_AllFourProviderTiersAreAvailable", func(t *testing.T) {
		// Given different deployments prefer different providers,
		set := map[string]bool{}
		for _, p := range SeedExpectedEmbeddingProviders {
			set[p] = true
		}
		for _, expected := range []string{"local", "openai", "huggingface", "ollama"} {
			assert.True(t, set[expected], "provider %q must be in seed", expected)
		}
	})

	t.Run("Scenario_SlugsCarryProviderPrefixForRoutingDispatch", func(t *testing.T) {
		// Given the runner dispatches to provider clients by parsing
		//       the slug prefix,
		for _, s := range SeedExpectedEmbeddingProviderSlugs {
			idx := strings.Index(s, "/")
			assert.Greater(t, idx, 0, "slug %q must use provider/model format", s)
		}
	})

	t.Run("Scenario_FreeAndPaidTiersAreBothPresent", func(t *testing.T) {
		// Given budget-sensitive tenants need a free option (local/ollama)
		//       AND quality-sensitive tenants need a paid one (openai-large),
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedEmbeddingProviderSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["local/e5-large"], "free in-cluster option required")
		assert.True(t, seedSet["openai/text-embedding-3-large"], "high-quality paid option required")
	})

	t.Run("Scenario_MultipleDimensionsCoverPgvectorCompatibility", func(t *testing.T) {
		// Given different KBs may use different pgvector column sizes
		//       (existing KBs may have hardcoded 1024 from E5-Large
		//       legacy default),
		// When the dimension catalog is inspected,
		// Then ≥3 distinct dimensions exist so a tenant can find a
		//      compatible provider.
		assert.GreaterOrEqual(t, len(SeedExpectedEmbeddingDimensions), 3,
			"≥3 distinct dimensions for pgvector compatibility")
	})

	t.Run("Scenario_RecommendedSetCoversCostSpectrum", func(t *testing.T) {
		// Given the picker UI shows "Suggested" providers,
		// When the recommended subset is inspected,
		// Then it covers free (local + ollama) AND cost-effective paid
		//      (openai-small) — but NOT the flagship paid (openai-large)
		//      since that's quality-tradeoff territory.
		recSet := map[string]bool{}
		for _, s := range SeedRecommendedEmbeddingProviderSlugs {
			recSet[s] = true
		}
		assert.True(t, recSet["local/e5-large"], "free in-cluster recommended")
		assert.True(t, recSet["openai/text-embedding-3-small"], "cost-effective paid recommended")
		assert.True(t, recSet["ollama/nomic-embed-text"], "free local-host recommended")
		assert.False(t, recSet["openai/text-embedding-3-large"],
			"flagship paid NOT recommended by default — explicit opt-in")
	})

	t.Run("Scenario_LocalProvidersExistForCostFreeRAG", func(t *testing.T) {
		// Given some deployments cannot/will not call external APIs
		//       (compliance, air-gap),
		// When the seed is inspected,
		// Then BOTH local (in-cluster) AND ollama (local-host) options exist.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedEmbeddingProviderSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["local/e5-large"], "in-cluster service required")
		assert.True(t, seedSet["ollama/nomic-embed-text"], "self-hosted Ollama required")
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		// Given external systems bind to count = 5,
		assert.Equal(t, 5, len(SeedExpectedEmbeddingProviderSlugs))
	})

	t.Run("Scenario_OpenAIProviderHasMultipleTiersForCostQualityTradeoff", func(t *testing.T) {
		// Given OpenAI has small (cheap) and large (high-quality) tiers,
		// When the OpenAI subset is inspected,
		// Then BOTH are present so tenants can pick by trade-off.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedEmbeddingProviderSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["openai/text-embedding-3-small"])
		assert.True(t, seedSet["openai/text-embedding-3-large"])
	})
}
