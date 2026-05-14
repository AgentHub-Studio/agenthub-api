package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreMCPServer_SeedExpectedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedExpectedMCPServerSlugs {
		assert.False(t, seen[slug], "duplicate slug in seed expectations: %q", slug)
		seen[slug] = true
	}
}

func TestCoreMCPServer_SeedExpectedSlugs_AllNonEmpty(t *testing.T) {
	for i, slug := range SeedExpectedMCPServerSlugs {
		assert.NotEmpty(t, slug, "seed slug at index %d must be non-empty", i)
	}
}

func TestCoreMCPServer_SeedExpectedSlugs_CanonicalCount(t *testing.T) {
	// 9 servers: search(2) + development(3) + monitoring(1) +
	// collaboration(1) + utility(2).
	assert.Equal(t, 9, len(SeedExpectedMCPServerSlugs),
		"9 MCP servers in canonical seed (refactor must update migration too)")
}

func TestCoreMCPServer_SeedExpectedCategories_AreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range SeedExpectedMCPServerCategories {
		assert.False(t, seen[c], "duplicate category %q", c)
		seen[c] = true
	}
}

func TestCoreMCPServer_SeedExpectedCategories_CanonicalCount(t *testing.T) {
	// 5 categories used by seed.
	assert.Equal(t, 5, len(SeedExpectedMCPServerCategories),
		"5 categories in canonical seed")
}

func TestCoreMCPServer_AuthTypeAllowlist_IsClosed(t *testing.T) {
	allowed := map[string]bool{}
	for _, a := range SeedExpectedMCPServerAuthTypes {
		allowed[a] = true
	}
	for _, want := range []string{"none", "api_key", "bearer_token", "oauth2", "basic"} {
		assert.True(t, allowed[want], "auth_type %q must be in seed allowlist", want)
	}
	// Foreign / dangerous auth types must NOT appear in the allowlist.
	assert.False(t, allowed["plaintext_password_in_url"],
		"plaintext password in URL must not be allowed")
	assert.False(t, allowed["digest"], "digest auth not seeded")
}

func TestCoreMCPServer_TransportAllowlist_IsHTTPOnly(t *testing.T) {
	// AgentHub is web → seed catalogue MUST be http-only. stdio implies
	// subprocess which is not portable to multi-tenant cloud.
	allowed := map[string]bool{}
	for _, tr := range SeedExpectedMCPServerTransports {
		allowed[tr] = true
	}
	assert.Equal(t, 1, len(SeedExpectedMCPServerTransports),
		"only http transport in seed catalogue (web product)")
	assert.True(t, allowed["http"], "http transport required")
	assert.False(t, allowed["stdio"], "stdio NOT seeded — subprocess not portable")
}

func TestCoreMCPServer_AuthRequiredSlugs_AllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedMCPServerSlugs {
		seedSet[s] = true
	}
	for _, authSlug := range SeedAuthRequiredMCPServerSlugs {
		assert.True(t, seedSet[authSlug],
			"auth-required slug %q must also appear in SeedExpectedMCPServerSlugs", authSlug)
	}
}

func TestCoreMCPServer_AuthRequiredSlugs_CanonicalCount(t *testing.T) {
	// 6 of 9 seed servers require auth (only web-fetch / memory / time
	// are no-auth utilities).
	assert.Equal(t, 6, len(SeedAuthRequiredMCPServerSlugs),
		"6 auth-required servers expected")
}

func TestCoreMCPServer_NoAuthSlugs_AreUtilityFetchers(t *testing.T) {
	// The 3 no-auth slugs MUST be the 3 zero-side-effect utility fetchers.
	authSet := map[string]bool{}
	for _, a := range SeedAuthRequiredMCPServerSlugs {
		authSet[a] = true
	}
	noAuth := []string{}
	for _, s := range SeedExpectedMCPServerSlugs {
		if !authSet[s] {
			noAuth = append(noAuth, s)
		}
	}
	assert.Equal(t, 3, len(noAuth), "3 no-auth slugs expected")
	noAuthSet := map[string]bool{}
	for _, s := range noAuth {
		noAuthSet[s] = true
	}
	assert.True(t, noAuthSet["web-fetch"])
	assert.True(t, noAuthSet["memory"])
	assert.True(t, noAuthSet["time"])
}
