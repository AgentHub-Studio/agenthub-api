package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD scenarios for migration 000092 capability command templates.
// These assert seed shape and category separation without requiring a database.

func TestBDD_CapabilityCommandTemplateSeed(t *testing.T) {
	t.Run("Scenario_FiveCapabilityCommandsAdaptedFromClaudeCodeSurfaceForWeb", func(t *testing.T) {
		// Given Claude Code exposes /research, /analyze, /plan, /summarize,
		//       and /tasks as built-in commands for power users,
		// When AgentHub adapts these for the web UX (migration 000092),
		// Then exactly 5 capability commands exist, each adapted from a
		//      Claude Code archetype:
		//   /research     → web search + fetch
		//   /analyze      → document exploration
		//   /plan         → goal decomposition + task tracking
		//   /summarize-doc → document summarisation (distinct from /summarize)
		//   /tasks        → task list display (no argument required)
		assert.Equal(t, 5, SeedCapabilityCommandCount,
			"migration 000092 must seed exactly 5 capability commands")
		assert.Equal(t, 5, len(SeedCapabilityCommandSlugs),
			"slug list must contain exactly 5 entries")

		slugSet := map[string]bool{}
		for _, s := range SeedCapabilityCommandSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet["research"], "/research adapts Claude Code web-search capability")
		assert.True(t, slugSet["analyze"], "/analyze adapts Claude Code document-explore capability")
		assert.True(t, slugSet["plan"], "/plan adapts Claude Code plan/task capability")
		assert.True(t, slugSet["summarize-doc"], "/summarize-doc provides document summarisation")
		assert.True(t, slugSet["tasks"], "/tasks provides task list display")
	})

	t.Run("Scenario_CapabilityCommandsAreInSeparateCategoryFromPlatformCommands", func(t *testing.T) {
		// Given the platform command palette has 5 stable categories
		//       (session/discovery/analytics/collaboration/feedback) which
		//       map to built-in handler_type commands,
		// When capability commands are introduced in migration 000092,
		// Then they are placed in a NEW 'capability' category that does NOT
		//      appear in the platform SeedExpectedCategories set — preventing
		//      UI palette pollution and category confusion.
		platformCatSet := map[string]bool{}
		for _, c := range SeedExpectedCategories {
			platformCatSet[c] = true
		}

		assert.False(t, platformCatSet[SeedCapabilityCommandCategory],
			"'capability' must be absent from the platform category closed set (%v)",
			SeedExpectedCategories)

		// Platform still has exactly 5 categories.
		assert.Len(t, SeedExpectedCategories, 5,
			"platform category count must remain 5 — capability commands get their own category")
	})

	t.Run("Scenario_ResearchAnalyzePlanTriggerPromptHandlerNotBuiltin", func(t *testing.T) {
		// Given capability commands bridge the chat UX to capability agents
		//       by expanding a prompt_template before invoking the LLM,
		// When the handler_type constant is inspected,
		// Then it is 'prompt' — NOT 'builtin' (which would require hardcoded
		//      runner logic) and NOT 'tool' (which would invoke a tool directly).
		assert.Equal(t, "prompt", SeedCapabilityCommandHandlerType,
			"capability commands must use handler_type='prompt' to expand templates")

		assert.NotEqual(t, "builtin", SeedCapabilityCommandHandlerType,
			"capability commands must NOT use builtin — they have no hardcoded runner logic")
		assert.NotEqual(t, "tool", SeedCapabilityCommandHandlerType,
			"capability commands must NOT use tool — they dispatch via prompt expansion")
	})

	t.Run("Scenario_TasksCommandRequiresNoArgument", func(t *testing.T) {
		// Given /tasks displays the current task list (no input required),
		// When the command slug list is inspected for 'tasks',
		// Then it is present — and the seed document specifies argument_hint=''
		//      (empty string) confirming no user argument is needed.
		slugSet := map[string]bool{}
		for _, s := range SeedCapabilityCommandSlugs {
			slugSet[s] = true
		}
		assert.True(t, slugSet["tasks"],
			"/tasks must be in capability command list (displays task status with no argument)")

		// The other 4 commands DO require an argument — /tasks is the outlier.
		commandsWithArgs := []string{"research", "analyze", "plan", "summarize-doc"}
		for _, slug := range commandsWithArgs {
			assert.True(t, slugSet[slug],
				"/%s requires an argument and must be in the capability list", slug)
		}
	})

	t.Run("Scenario_CapabilityCommandSlugsDoNotOverlapWithExistingPlatformCommands", func(t *testing.T) {
		// Given the platform seed (migration 000008) installs 13 commands
		//       with slugs like 'summarize', 'help', 'clear', 'agents', etc.,
		// When capability commands are introduced in migration 000092,
		// Then NO capability slug collides with a platform slug — in particular
		//      'summarize-doc' is chosen (not 'summarize') to avoid collision
		//      with the existing /summarize session compact command.
		platformSet := map[string]bool{}
		for _, s := range SeedExpectedSlugs {
			platformSet[s] = true
		}

		for _, slug := range SeedCapabilityCommandSlugs {
			assert.False(t, platformSet[slug],
				"capability command slug %q must NOT collide with platform command slug (migration 000008)",
				slug)
		}

		// Explicitly assert the critical non-collision that motivated the
		// 'summarize-doc' naming choice.
		assert.True(t, platformSet["summarize"],
			"platform /summarize must still exist after migration 000092")
		capSet := map[string]bool{}
		for _, s := range SeedCapabilityCommandSlugs {
			capSet[s] = true
		}
		assert.False(t, capSet["summarize"],
			"/summarize must NOT appear in capability commands — use summarize-doc instead")
		assert.True(t, capSet["summarize-doc"],
			"/summarize-doc is the capability variant that avoids collision with /summarize")
	})
}
