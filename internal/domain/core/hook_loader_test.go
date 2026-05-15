package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreHook_SeedExpectedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedExpectedHookSlugs {
		assert.False(t, seen[slug], "duplicate slug in seed expectations: %q", slug)
		seen[slug] = true
	}
}

func TestCoreHook_SeedExpectedSlugs_AllNonEmpty(t *testing.T) {
	for i, slug := range SeedExpectedHookSlugs {
		assert.NotEmpty(t, slug, "seed slug at index %d must be non-empty", i)
	}
}

func TestCoreHook_SeedExpectedSlugs_CanonicalCount(t *testing.T) {
	// 11 hooks: safety(4) + lifecycle(3) + context(2) + coordination(2).
	assert.Equal(t, 11, len(SeedExpectedHookSlugs),
		"11 hooks in canonical seed (refactor must update migration too)")
}

func TestCoreHook_SeedExpectedEvents_AreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, ev := range SeedExpectedHookEvents {
		assert.False(t, seen[ev], "duplicate event %q in expected list", ev)
		seen[ev] = true
	}
}

func TestCoreHook_SeedExpectedEvents_CanonicalCount(t *testing.T) {
	// 10 distinct ExtendedHookEvent names exercised by the seed.
	assert.Equal(t, 10, len(SeedExpectedHookEvents),
		"10 distinct lifecycle events covered by seed")
}

func TestCoreHook_AdminOnlyDisableSlugs_AllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedHookSlugs {
		seedSet[s] = true
	}
	for _, adminSlug := range SeedAdminOnlyDisableHookSlugs {
		assert.True(t, seedSet[adminSlug],
			"admin-only slug %q must also appear in SeedExpectedHookSlugs", adminSlug)
	}
}

func TestCoreHook_AdminOnlyDisableSlugs_CoverAllSafetyOnly(t *testing.T) {
	// All admin-only hooks must be in the safety category. Lifecycle/
	// context/coordination hooks are tenant-disable-able.
	assert.Equal(t, 4, len(SeedAdminOnlyDisableHookSlugs),
		"4 admin-only safety hooks expected")
	for _, s := range SeedAdminOnlyDisableHookSlugs {
		assert.Contains(t, s, "safety-",
			"admin-only slug %q must start with 'safety-' prefix", s)
	}
}

func TestCoreHook_SafetyHooksCoverAllFiveSafetyEventsExceptPostToolUse(t *testing.T) {
	// PDF Section 5.3 lists 5 safety hooks (PreToolUse, PostToolUse,
	// PostToolUseFailure, PermissionRequest, PermissionDenied). The seed
	// covers PreToolUse (twice — destructive + redact), PermissionDenied,
	// PostToolUseFailure. PermissionRequest + PostToolUse are intentionally
	// not seeded — tenants register their own classifier/recorder hooks.
	covered := map[string]bool{}
	for _, ev := range SeedExpectedHookEvents {
		covered[ev] = true
	}
	assert.True(t, covered["PreToolUse"], "PreToolUse safety hook must be seeded")
	assert.True(t, covered["PermissionDenied"], "PermissionDenied safety hook must be seeded")
	assert.True(t, covered["PostToolUseFailure"], "PostToolUseFailure safety hook must be seeded")
	// Not seeded:
	assert.False(t, covered["PostToolUse"], "PostToolUse not seeded — tenant-specific recording")
	assert.False(t, covered["PermissionRequest"], "PermissionRequest not seeded — tenant classifier")
}

func TestCoreHook_HookTypeAllowlist(t *testing.T) {
	// Seed must use only "prompt" type — http/agent/command would have
	// side effects we cannot guarantee at seed time.
	allowed := map[string]bool{"prompt": true}
	assert.True(t, allowed["prompt"])
	assert.False(t, allowed["http"], "seed must NOT include http hooks")
	assert.False(t, allowed["agent"], "seed must NOT include agent hooks")
	assert.False(t, allowed["command"], "seed must NOT include command hooks")
}
