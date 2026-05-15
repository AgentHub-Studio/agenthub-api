package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreTool_SeedExpectedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedExpectedToolSlugs {
		assert.False(t, seen[slug], "duplicate slug in seed expectations: %q", slug)
		seen[slug] = true
	}
}

func TestCoreTool_SeedExpectedSlugs_AllNonEmpty(t *testing.T) {
	for i, slug := range SeedExpectedToolSlugs {
		assert.NotEmpty(t, slug, "seed slug at index %d must be non-empty", i)
	}
}

func TestCoreTool_SeedExpectedSlugs_AllUseCorePrefix(t *testing.T) {
	// Every seed tool MUST use the "core-" prefix to distinguish from
	// tenant-custom tools. This is a namespace contract.
	for _, slug := range SeedExpectedToolSlugs {
		assert.True(t, len(slug) > len(SeedExpectedToolSlugPrefix) && slug[:len(SeedExpectedToolSlugPrefix)] == SeedExpectedToolSlugPrefix,
			"slug %q must start with %q", slug, SeedExpectedToolSlugPrefix)
	}
}

func TestCoreTool_SeedExpectedSlugs_CanonicalCount(t *testing.T) {
	// 31 tools in canonical seed.
	assert.Equal(t, 31, len(SeedExpectedToolSlugs),
		"31 tools in canonical seed (refactor must update migration too)")
}

func TestCoreTool_SeedExpectedToolType_IsHTTP(t *testing.T) {
	// All seed tools use HTTP type — they proxy REST calls back to the
	// AgentHub backend. SQL/DOCUMENT_SEARCH/CUSTOM tools are not seeded.
	assert.Equal(t, "HTTP", SeedExpectedToolType,
		"seed tools MUST use HTTP type (proxy contract)")
}

func TestCoreTool_RequiredConfigKeys_AreAllowedKeys(t *testing.T) {
	allowed := map[string]bool{}
	for _, k := range SeedExpectedToolConfigKeys {
		allowed[k] = true
	}
	for _, k := range SeedRequiredToolConfigKeys {
		assert.True(t, allowed[k],
			"required key %q must also appear in allowed-keys list", k)
	}
}

func TestCoreTool_RequiredConfigKeys_CanonicalSet(t *testing.T) {
	// HTTP tool MUST always specify url + method + useCallerToken.
	assert.ElementsMatch(t,
		[]string{"url", "method", "useCallerToken"},
		SeedRequiredToolConfigKeys,
		"required config keys must be exactly url+method+useCallerToken")
}

func TestCoreTool_SlugsAreFilesystemAndURLSafe(t *testing.T) {
	for _, slug := range SeedExpectedToolSlugs {
		for _, r := range slug {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
			assert.True(t, ok, "slug %q has invalid char %q (must be [a-z0-9-])", slug, r)
		}
	}
}
