package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for the capability command seed constants (migration 000092).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityCommandCount_MatchesSlugList(t *testing.T) {
	assert.Equal(t, SeedCapabilityCommandCount, len(SeedCapabilityCommandSlugs),
		"SeedCapabilityCommandCount must match len(SeedCapabilityCommandSlugs)")
}

func TestSeedCapabilityCommandSlugs_ContainsResearch(t *testing.T) {
	assert.Contains(t, SeedCapabilityCommandSlugs, SeedCapabilityResearchSlug,
		"capability commands must include the /research command")
}

func TestSeedCapabilityCommandSlugs_ContainsAnalyze(t *testing.T) {
	assert.Contains(t, SeedCapabilityCommandSlugs, SeedCapabilityAnalyzeSlug,
		"capability commands must include the /analyze command")
}

func TestSeedCapabilityCommandSlugs_ContainsPlan(t *testing.T) {
	assert.Contains(t, SeedCapabilityCommandSlugs, SeedCapabilityPlanSlug,
		"capability commands must include the /plan command")
}

func TestSeedCapabilityCommandSlugs_ContainsSummarizeDoc(t *testing.T) {
	assert.Contains(t, SeedCapabilityCommandSlugs, "summarize-doc",
		"capability commands must include /summarize-doc (distinct from platform /summarize)")
}

func TestSeedCapabilityCommandSlugs_ContainsTasks(t *testing.T) {
	assert.Contains(t, SeedCapabilityCommandSlugs, "tasks",
		"capability commands must include /tasks for task list display")
}

func TestSeedCapabilityCommandCategory_IsCapability(t *testing.T) {
	assert.Equal(t, "capability", SeedCapabilityCommandCategory,
		"capability commands must use the 'capability' category tag")
}

func TestSeedCapabilityCommandCategory_IsDistinctFromPlatformCategories(t *testing.T) {
	// Platform categories are session/discovery/analytics/collaboration/feedback.
	// 'capability' must NOT appear in that closed set — it is a new grouping
	// for commands that invoke capability-layer agents.
	platformCatSet := map[string]bool{}
	for _, c := range SeedExpectedCategories {
		platformCatSet[c] = true
	}
	assert.False(t, platformCatSet[SeedCapabilityCommandCategory],
		"capability command category %q must NOT be in SeedExpectedCategories (platform-only set)",
		SeedCapabilityCommandCategory)
}

func TestSeedCapabilityCommandHandlerType_IsPrompt(t *testing.T) {
	assert.Equal(t, "prompt", SeedCapabilityCommandHandlerType,
		"all capability commands use handler_type='prompt' — runner expands prompt_template with args")
}

func TestSeedCapabilityCommandSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range SeedCapabilityCommandSlugs {
		assert.False(t, seen[slug], "duplicate capability command slug %q", slug)
		seen[slug] = true
	}
}

func TestSeedCapabilityCommandSlugs_NoOverlapWithPlatformCommands(t *testing.T) {
	// Capability command slugs must not clash with the 13-row platform seed.
	// The 'summarize-doc' slug is intentionally chosen (not 'summarize') to
	// avoid collision with the platform /summarize session command.
	platformSet := map[string]bool{}
	for _, s := range SeedExpectedSlugs {
		platformSet[s] = true
	}
	for _, slug := range SeedCapabilityCommandSlugs {
		assert.False(t, platformSet[slug],
			"capability command slug %q must NOT collide with platform command slug",
			slug)
	}
}

func TestSeedCapabilityCommandSlugs_AllUseKebabCase(t *testing.T) {
	// Slugs must use only lowercase letters, digits, and hyphens.
	// No underscores (use hyphens), no spaces, no uppercase.
	for _, slug := range SeedCapabilityCommandSlugs {
		assert.NotEmpty(t, slug, "slug must not be empty")
		for _, r := range slug {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
			assert.True(t, ok,
				"capability command slug %q has invalid char %q (must be lowercase + digits + hyphen only)",
				slug, r)
		}
	}
}

func TestSeedCapabilityCommandSlugs_CountIsFive(t *testing.T) {
	// 5 commands is the initial capability surface (research, analyze, plan,
	// summarize-doc, tasks). Changing this requires updating the migration.
	assert.Equal(t, 5, SeedCapabilityCommandCount,
		"5 capability commands in migration 000092 — change requires updating both migration and constant")
	assert.Equal(t, 5, len(SeedCapabilityCommandSlugs),
		"slug list must have exactly 5 entries matching SeedCapabilityCommandCount")
}
