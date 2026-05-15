package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreLLMModel_SeedExpectedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedLLMModelSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreLLMModel_SeedExpectedSlugs_AllNonEmpty(t *testing.T) {
	for i, s := range SeedExpectedLLMModelSlugs {
		assert.NotEmpty(t, s, "slug at %d must be non-empty", i)
	}
}

func TestCoreLLMModel_SeedExpectedSlugs_CanonicalCount(t *testing.T) {
	// 7 models = 2 anthropic + 2 openai + 2 openrouter + 1 ollama.
	assert.Equal(t, 7, len(SeedExpectedLLMModelSlugs),
		"7 LLM models in canonical seed")
}

func TestCoreLLMModel_SlugsUseProviderPrefix(t *testing.T) {
	// Format: "<provider>/<model>" — provider extractable from slug.
	allowed := map[string]bool{}
	for _, p := range SeedExpectedLLMModelProviders {
		allowed[p] = true
	}
	for _, s := range SeedExpectedLLMModelSlugs {
		idx := strings.Index(s, "/")
		assert.Greater(t, idx, 0, "slug %q must have provider prefix", s)
		prefix := s[:idx]
		assert.True(t, allowed[prefix],
			"slug %q has provider prefix %q outside allowed list", s, prefix)
	}
}

func TestCoreLLMModel_ProvidersAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range SeedExpectedLLMModelProviders {
		assert.False(t, seen[p], "duplicate provider %q", p)
		seen[p] = true
	}
}

func TestCoreLLMModel_ProvidersCanonicalCount(t *testing.T) {
	assert.Equal(t, 4, len(SeedExpectedLLMModelProviders),
		"4 providers: anthropic + openai + openrouter + ollama")
}

func TestCoreLLMModel_RecommendedSlugsAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedLLMModelSlugs {
		seedSet[s] = true
	}
	for _, r := range SeedRecommendedLLMModelSlugs {
		assert.True(t, seedSet[r],
			"recommended slug %q must appear in canonical seed", r)
	}
}

func TestCoreLLMModel_RecommendedCountMatchesProviders(t *testing.T) {
	// One recommended per provider tier (4 providers → 4 recommended).
	assert.Equal(t, 4, len(SeedRecommendedLLMModelSlugs),
		"one recommended per provider tier")
}

func TestCoreLLMModel_RecommendedSlugsHaveOnePerProvider(t *testing.T) {
	seenProvider := map[string]bool{}
	for _, slug := range SeedRecommendedLLMModelSlugs {
		idx := strings.Index(slug, "/")
		require := assert.New(t)
		require.Greater(idx, 0)
		provider := slug[:idx]
		assert.False(t, seenProvider[provider],
			"provider %q has more than one recommended (%q)", provider, slug)
		seenProvider[provider] = true
	}
}

func TestCoreLLMModel_OpenrouterRecommendedIsBDDValidatedModel(t *testing.T) {
	// Memory note: mistral-nemo is the validated stable BDD model
	// (free models on OpenRouter are unreliable per
	// reference_openrouter_models_for_bdd.md).
	recSet := map[string]bool{}
	for _, s := range SeedRecommendedLLMModelSlugs {
		recSet[s] = true
	}
	assert.True(t, recSet["openrouter/mistralai/mistral-nemo"],
		"OpenRouter recommended must be mistral-nemo (BDD-validated stable)")
}
