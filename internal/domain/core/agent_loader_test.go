package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreAgent_SeedExpectedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedAgentSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreAgent_SeedExpectedSlugs_AllNonEmpty(t *testing.T) {
	for i, s := range SeedExpectedAgentSlugs {
		assert.NotEmpty(t, s, "slug at %d must be non-empty", i)
	}
}

func TestCoreAgent_SeedExpectedSlugs_AllUseCorePrefix(t *testing.T) {
	for _, s := range SeedExpectedAgentSlugs {
		assert.True(t, strings.HasPrefix(s, SeedExpectedAgentSlugPrefix),
			"slug %q must start with %q", s, SeedExpectedAgentSlugPrefix)
	}
}

func TestCoreAgent_SeedExpectedSlugs_CanonicalCount(t *testing.T) {
	// 12 specialist agents per migration 000006 + spec §21.4.
	assert.Equal(t, 12, len(SeedExpectedAgentSlugs),
		"12 specialist agents in canonical seed")
}

func TestCoreAgent_AssistantSlugIsInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedAgentSlugs {
		seedSet[s] = true
	}
	assert.True(t, seedSet[SeedAssistantSlug],
		"%q must appear in canonical seed", SeedAssistantSlug)
}

func TestCoreAgent_DeprecatedSlugIsInSeed(t *testing.T) {
	// Pipeline specialist is deprecated but kept in catalog as read-only.
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedAgentSlugs {
		seedSet[s] = true
	}
	assert.True(t, seedSet[SeedDeprecatedAgentSlug],
		"deprecated %q must remain in catalog (read-only mode)",
		SeedDeprecatedAgentSlug)
}

func TestCoreAgent_AgentTypesAreBounded(t *testing.T) {
	allowed := map[string]bool{}
	for _, t := range SeedExpectedAgentTypes {
		allowed[t] = true
	}
	assert.True(t, allowed["ASSISTANT"])
	assert.True(t, allowed["SPECIALIST"])
	assert.Equal(t, 2, len(SeedExpectedAgentTypes),
		"only 2 agent types in seed: ASSISTANT + SPECIALIST")
}

func TestCoreAgent_SlugsAreFilesystemAndURLSafe(t *testing.T) {
	for _, s := range SeedExpectedAgentSlugs {
		for _, r := range s {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
			assert.True(t, ok, "slug %q has invalid char %q", s, r)
		}
	}
}

func TestCoreAgent_BindingsCountIsKnownDiscrepancy(t *testing.T) {
	// The migration has a known slug-mismatch — only the core-assistant
	// binding produces rows (cross-product with all skills = 7).
	// This constant DOCUMENTS the observed-state. If the migration is
	// fixed, this constant + the integration test must be updated together.
	assert.Equal(t, 7, SeedExpectedAgentSkillBindingsCount,
		"observed binding count = 7 (known discrepancy — see loader docs)")
}
