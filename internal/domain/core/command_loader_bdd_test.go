package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify the ah_core.command seed against the
// project goal: a fresh tenant, with NO commands of its own, must already
// see a usable set of slash commands inherited from ah_core.
//
// These scenarios assert seed shape + category coverage + WEB adaptation
// (omission of CLI/IDE-only commands) — backed by the SeedExpectedSlugs /
// SeedExpectedCategories constants which the integration test cross-checks
// against actual DB rows.

func TestBDD_AhCoreCommandSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsUsableCommandSurface", func(t *testing.T) {
		// Given a fresh tenant (no commands of its own),
		// When the runtime asks ah_core for available commands,
		// Then at least the canonical web set is seeded — user can /help
		//      immediately on day one.
		assert.GreaterOrEqual(t, len(SeedExpectedSlugs), 10,
			"fresh tenant must inherit at least 10 commands from ah_core")
	})

	t.Run("Scenario_SessionCategoryCoversCoreLifecycle", func(t *testing.T) {
		// Given the session category groups lifecycle commands (PDF
		//       Section 9: resume/fork/clear are core to session lifecycle),
		// When we list the slugs the seed claims to include,
		expectedSession := []string{"help", "clear", "reset", "summarize", "export", "resume", "branch"}

		// Then all 7 session-control commands are present.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedSlugs {
			seedSet[s] = true
		}
		for _, s := range expectedSession {
			assert.True(t, seedSet[s],
				"session category must include %q for lifecycle parity with Claude Code", s)
		}
	})

	t.Run("Scenario_DiscoveryCategoryExposesAgentAndSkillSurface", func(t *testing.T) {
		// Given a tenant with seeded ah_core agents + skills (existing
		//       seed migrations 003/004/006), users need a way to surface
		//       them in the UI palette,
		expectedDiscovery := []string{"skills", "agents", "memory"}

		// Then the seed includes discovery commands for each.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedSlugs {
			seedSet[s] = true
		}
		for _, s := range expectedDiscovery {
			assert.True(t, seedSet[s],
				"discovery category must include %q to surface seeded ah_core entities", s)
		}
	})

	t.Run("Scenario_AnalyticsCategoryExposesCostInsight", func(t *testing.T) {
		// Given the runtime tracks per-run cost (OBS-002 RunMetric.CostUSD),
		// When the user wants to see it inline,
		// Then /cost is the canonical slash command — not buried in a
		//      separate analytics page.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["cost"],
			"/cost must be in the seed for in-chat cost visibility")
	})

	t.Run("Scenario_CollaborationCategoryEnablesSessionSharing", func(t *testing.T) {
		// Given AgentHub is a WEB product, sharing is a first-class web
		//       capability (not present in Claude Code CLI),
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedSlugs {
			seedSet[s] = true
		}

		// Then /share is a web-native addition — explicitly NOT inherited
		//      from the Claude Code CLI but adapted to AgentHub's reality.
		assert.True(t, seedSet["share"],
			"/share is a web-native addition for session collaboration")
	})

	t.Run("Scenario_FeedbackCategoryEnablesUserVoice", func(t *testing.T) {
		// Given platform improvement depends on user signal,
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedSlugs {
			seedSet[s] = true
		}

		// Then /feedback is in the seed as a low-friction signal capture.
		assert.True(t, seedSet["feedback"],
			"/feedback must be available so users have a voice without leaving the chat")
	})

	t.Run("Scenario_NoCLIOnlyCommandsLeakIntoWebSeed", func(t *testing.T) {
		// Given AgentHub is WEB-only (PDF Section 6.1 Plugin component
		//       types include LSP servers / channels / shell integrations
		//       — these are CLI/IDE concepts that don't apply to a web UX),
		// When the seed catalogue is inspected,
		// Then no CLI/IDE-only command appears — preventing dead UI
		//      entries and confused users.
		webIncompatible := []string{
			"init", "add-dir", "ide", "sandbox", "vim", "editor", "shell",
			"mcp", // CLI-side MCP server config; AgentHub manages MCP via web UI
		}
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedSlugs {
			seedSet[s] = true
		}
		for _, banned := range webIncompatible {
			assert.False(t, seedSet[banned],
				"WEB seed must NOT include CLI/IDE-only command %q", banned)
		}
	})

	t.Run("Scenario_SeedExpectsFiveDistinctCategoriesForUIGrouping", func(t *testing.T) {
		// Given the UI command palette groups commands by category for
		//       discoverability,
		// When we list the seed-supported categories,
		// Then exactly 5 — refactor that adds a 6th must update the UI
		//      palette grid layout.
		assert.Len(t, SeedExpectedCategories, 5,
			"5 categories expected for stable UI palette layout")
		seenCat := map[string]bool{}
		for _, c := range SeedExpectedCategories {
			assert.False(t, seenCat[c], "duplicate category %q", c)
			seenCat[c] = true
		}
	})

	t.Run("Scenario_SlugsAreFilesystemAndURLSafe", func(t *testing.T) {
		// Given commands may be embedded in URLs (deep-link to /share?cmd=…)
		//       and in filesystem paths (export pages),
		// When we inspect each slug,
		// Then only safe characters appear (lowercase ASCII + digits +
		//      hyphen + underscore) — no spaces, no special chars.
		for _, slug := range SeedExpectedSlugs {
			for _, r := range slug {
				safe := (r >= 'a' && r <= 'z') ||
					(r >= '0' && r <= '9') ||
					r == '-' || r == '_'
				assert.True(t, safe,
					"slug %q has unsafe char %q (must be filesystem + URL safe)", slug, r)
			}
		}
	})

	t.Run("Scenario_CanonicalCountIsExplicitGuardAgainstAccidentalChanges", func(t *testing.T) {
		// Given seed migration changes are easy to make accidentally
		//       (someone deletes a row, comments out a block),
		// When the canonical count constant is checked,
		// Then it matches the documented 13 — a refactor that adds or
		//      removes commands MUST update both the migration and this
		//      list, ensuring intentional change.
		assert.Equal(t, 13, len(SeedExpectedSlugs),
			"canonical count is 13 — change requires updating both migration and list")
	})
}
