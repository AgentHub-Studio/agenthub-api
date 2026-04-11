package skill

import (
	"time"

	"github.com/google/uuid"
)

// Skill represents an abstract AI capability that can be bound to one or more tools.
type Skill struct {
	ID           uuid.UUID `db:"id"`
	Name         string    `db:"name"`
	Slug         string    `db:"slug"`
	Description  string    `db:"description"`
	Instructions string    `db:"instructions"`
	Category     string    `db:"category"`
	// AllowedTools restricts which tools this skill can use when invoked.
	// Empty means all tools are allowed. Inspired by Claude Code's
	// BundledSkillDefinition.allowedTools for fine-grained security.
	AllowedTools []string  `db:"allowed_tools"`
	// DisableModelInvocation marks skills that should only be invoked by the user
	// (via slash commands or UI), not by the LLM. Their descriptions are excluded
	// from the system prompt to save context tokens.
	// Inspired by Claude Code's BundledSkillDefinition.disableModelInvocation.
	DisableModelInvocation bool `db:"disable_model_invocation"`
	// ContextMode determines how the skill executes:
	//   "inline" — runs in the current conversation context (default)
	//   "fork" — runs in a sub-agent with its own context
	// Fork mode prevents diagnostic skills from polluting the main conversation.
	// Inspired by Claude Code's BundledSkillDefinition.context ('inline' | 'fork').
	ContextMode string `db:"context_mode"`
	// WhenToUse provides guidance on WHEN the LLM should invoke this skill.
	// Separate from Description (which says WHAT it does). Injected into the
	// system prompt as a sub-bullet under the tool listing so the LLM can make
	// more precise selection decisions without bloating the tool description.
	// Inspired by Claude Code's BundledSkillDefinition.whenToUse.
	WhenToUse *string `db:"when_to_use"`
	// ArgumentHint is a short hint for slash command argument format shown in the UI.
	// Example: "<agent_id> [focus]".
	// Inspired by Claude Code's BundledSkillDefinition.argumentHint.
	ArgumentHint *string `db:"argument_hint"`
	// ShouldDefer controls skill-level progressive disclosure.
	// When true, the entire skill is withheld from the initial tool list and
	// requires a tool_search round-trip to load. This is the 3rd layer of
	// disclosure (after tool-level deferral and the DeferredToolThreshold).
	// Inspired by Claude Code's ShouldDefer flag applied at the skill level.
	ShouldDefer bool      `db:"should_defer"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}
