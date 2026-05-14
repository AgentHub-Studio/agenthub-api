package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreEmbeddingProvider_SeedExpectedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedEmbeddingProviderSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreEmbeddingProvider_SeedExpectedSlugs_AllNonEmpty(t *testing.T) {
	for i, s := range SeedExpectedEmbeddingProviderSlugs {
		assert.NotEmpty(t, s, "slug at %d must be non-empty", i)
	}
}

func TestCoreEmbeddingProvider_SeedExpectedSlugs_CanonicalCount(t *testing.T) {
	// 5 providers = 1 local + 2 openai + 1 huggingface + 1 ollama.
	assert.Equal(t, 5, len(SeedExpectedEmbeddingProviderSlugs),
		"5 embedding providers in canonical seed")
}

func TestCoreEmbeddingProvider_SlugsUseProviderPrefix(t *testing.T) {
	allowed := map[string]bool{}
	for _, p := range SeedExpectedEmbeddingProviders {
		allowed[p] = true
	}
	for _, s := range SeedExpectedEmbeddingProviderSlugs {
		idx := strings.Index(s, "/")
		assert.Greater(t, idx, 0, "slug %q must have provider prefix", s)
		prefix := s[:idx]
		assert.True(t, allowed[prefix], "slug %q has prefix %q outside allowed", s, prefix)
	}
}

func TestCoreEmbeddingProvider_ProvidersAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range SeedExpectedEmbeddingProviders {
		assert.False(t, seen[p], "duplicate provider %q", p)
		seen[p] = true
	}
}

func TestCoreEmbeddingProvider_DefaultSlugIsInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedEmbeddingProviderSlugs {
		seedSet[s] = true
	}
	assert.True(t, seedSet[SeedDefaultEmbeddingProviderSlug],
		"default %q must be in canonical seed", SeedDefaultEmbeddingProviderSlug)
}

func TestCoreEmbeddingProvider_DefaultIsLocalE5Large(t *testing.T) {
	// CLAUDE.md says agenthub-embedding E5-Large 1024dim is the AgentHub default.
	assert.Equal(t, "local/e5-large", SeedDefaultEmbeddingProviderSlug,
		"AgentHub historical default = local/e5-large per CLAUDE.md")
}

func TestCoreEmbeddingProvider_RecommendedAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedEmbeddingProviderSlugs {
		seedSet[s] = true
	}
	for _, r := range SeedRecommendedEmbeddingProviderSlugs {
		assert.True(t, seedSet[r], "recommended %q must be in seed", r)
	}
}

func TestCoreEmbeddingProvider_DimensionsAreSensiblePowersOrCommon(t *testing.T) {
	// Common embedding dimensions — used for pgvector compatibility check.
	common := map[int]bool{384: true, 768: true, 1024: true, 1536: true, 3072: true}
	for _, d := range SeedExpectedEmbeddingDimensions {
		assert.True(t, common[d],
			"dimension %d not in common embedding sizes (384/768/1024/1536/3072)", d)
	}
}
