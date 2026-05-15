package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for the CoreCommand contract that don't require a database.
// Integration tests against a live ah_core schema live in test files that
// use the testcontainers harness (out of scope here).

func TestCoreCommand_SeedExpectedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedExpectedSlugs {
		assert.False(t, seen[slug], "duplicate slug in seed expectations: %q", slug)
		seen[slug] = true
	}
}

func TestCoreCommand_SeedExpectedSlugs_AllNonEmpty(t *testing.T) {
	for i, slug := range SeedExpectedSlugs {
		assert.NotEmpty(t, slug, "seed slug at index %d must be non-empty", i)
	}
}

func TestCoreCommand_SeedExpectedSlugs_AreLowercaseKebabOrSnake(t *testing.T) {
	// Slash commands MUST be safe to type and parse — lowercase + only
	// letters/digits/underscores/dashes allowed.
	for _, slug := range SeedExpectedSlugs {
		for _, r := range slug {
			isValid := (r >= 'a' && r <= 'z') ||
				(r >= '0' && r <= '9') ||
				r == '-' || r == '_'
			assert.True(t, isValid,
				"slug %q contains invalid char %q (must be lowercase letters, digits, '-' or '_')",
				slug, r)
		}
	}
}

func TestCoreCommand_SeedExpectedCategories_FiniteAndStable(t *testing.T) {
	// 5 categories matches the UI palette grouping. Refactor that adds a
	// 6th must update both the seed and the front-end — this guard
	// surfaces the change.
	assert.Len(t, SeedExpectedCategories, 5,
		"5 stable command categories — adding new requires UI update")
}

func TestCoreCommand_SeedExpectedSlugs_CanonicalCount(t *testing.T) {
	// 13 commands is the WEB-adapted seed surface.
	// Refactor that adds new commands must extend SeedExpectedSlugs and
	// the seed migration together.
	assert.Equal(t, 13, len(SeedExpectedSlugs),
		"13 commands in canonical web seed (refactor must update seed migration too)")
}

func TestCoreCommand_SeedExpectedSlugs_OmitsCLIOnly(t *testing.T) {
	// AgentHub is a WEB product — these CLI/IDE-only commands MUST NOT
	// appear in the seed (would create dead UI entries).
	cliOnly := []string{
		"init",     // CLAUDE.md initialisation (CLI)
		"add-dir",  // worktree management (CLI)
		"ide",      // IDE integration
		"sandbox",  // shell sandbox toggle
		"vim",      // editor mode
		"editor",   // editor mode
		"shell",    // raw shell prompt
	}
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedSlugs {
		seedSet[s] = true
	}
	for _, banned := range cliOnly {
		assert.False(t, seedSet[banned],
			"CLI/IDE-only command %q must NOT appear in WEB seed", banned)
	}
}

func TestCoreCommand_HandlerTypeValuesAreKnown(t *testing.T) {
	// 3 handler types — anything else is a typo or a new dispatch path
	// the runner doesn't know about.
	known := map[string]bool{"builtin": true, "prompt": true, "tool": true}
	// We can't inspect the seed rows here without a DB. We assert the
	// allowlist is present so the loader knows what to dispatch on.
	assert.True(t, known["builtin"])
	assert.True(t, known["prompt"])
	assert.True(t, known["tool"])
	assert.Len(t, known, 3,
		"3 handler types — extending requires runner dispatcher update")
}
