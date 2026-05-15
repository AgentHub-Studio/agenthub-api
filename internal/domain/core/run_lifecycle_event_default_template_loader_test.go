package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreRLET_SlugsCount(t *testing.T) {
	assert.Equal(t, 6, len(SeedExpectedRLETSlugs))
	assert.Equal(t, 6, SeedExpectedRLETRowCount)
}

func TestCoreRLET_SlugsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedRLETSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreRLET_AllSlugsMatchKebabRegex(t *testing.T) {
	for _, s := range SeedExpectedRLETSlugs {
		assert.True(t, RLETSlugRE.MatchString(s), "slug %q must match kebab regex", s)
	}
}

func TestCoreRLET_HookEventsClosedSet(t *testing.T) {
	expected := map[string]bool{
		"RunStarted": true, "RunComplete": true,
		"ContextWindowAlert": true, "KnowledgeBaseQueried": true,
	}
	for _, e := range SeedRLETHookEvents {
		assert.True(t, expected[e], "hook_event %q outside closed set", e)
	}
	assert.Equal(t, 4, len(SeedRLETHookEvents))
}

func TestCoreRLET_HookEventsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range SeedRLETHookEvents {
		assert.False(t, seen[e], "duplicate hook_event %q", e)
		seen[e] = true
	}
}

func TestCoreRLET_HandlerKindsClosedSet(t *testing.T) {
	expected := map[string]bool{"webhook": true, "notification": true, "audit": true}
	for _, k := range SeedRLETHandlerKinds {
		assert.True(t, expected[k], "handler_kind %q outside closed set", k)
	}
	assert.Equal(t, 3, len(SeedRLETHandlerKinds))
}

func TestCoreRLET_HandlerKindsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range SeedRLETHandlerKinds {
		assert.False(t, seen[k], "duplicate handler_kind %q", k)
		seen[k] = true
	}
}

func TestCoreRLET_EnabledByDefaultSlugsAreSubset(t *testing.T) {
	all := map[string]bool{}
	for _, s := range SeedExpectedRLETSlugs {
		all[s] = true
	}
	for _, s := range SeedRLETEnabledByDefaultSlugs {
		assert.True(t, all[s], "enabled_by_default slug %q not in all slugs", s)
	}
}

func TestCoreRLET_EnabledByDefaultCount(t *testing.T) {
	// 3 out of 6 templates are enabled by default
	assert.Equal(t, 3, len(SeedRLETEnabledByDefaultSlugs))
}

func TestCoreRLET_RowCountEqualsSlugCount(t *testing.T) {
	assert.Equal(t, SeedExpectedRLETRowCount, len(SeedExpectedRLETSlugs))
}

func TestCoreRLET_HookEventsMatchEXT002aWebAdapted(t *testing.T) {
	// The hook_events in this seed must exactly match the 4 web-adapted events added in EXT-002a.
	expected := map[string]bool{
		"RunStarted": true, "RunComplete": true,
		"ContextWindowAlert": true, "KnowledgeBaseQueried": true,
	}
	assert.Equal(t, 4, len(SeedRLETHookEvents))
	for _, e := range SeedRLETHookEvents {
		assert.True(t, expected[e],
			"RLET hook event %q must be one of the EXT-002a web-adapted events", e)
	}
}
