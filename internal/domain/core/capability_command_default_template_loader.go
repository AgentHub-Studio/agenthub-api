package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityCommandLoader loads capability-category slash commands from
// ah_core.command. These commands give tenants instant access to the capability
// layer introduced in migrations 000090 (skills/tools) and 000091 (agents)
// without any custom configuration. Seeded by migration 000092.
//
// Distinct from CoreCommandLoader which loads the full platform command set
// (category: session/discovery/analytics/collaboration/feedback). This loader
// targets only the 'capability' category which bridges the chat UX to the
// capability agents (core-researcher, core-analyst, core-planner).
type CoreCapabilityCommandLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityCommandLoader creates a CoreCapabilityCommandLoader.
func NewCoreCapabilityCommandLoader(pool *pgxpool.Pool) *CoreCapabilityCommandLoader {
	return &CoreCapabilityCommandLoader{pool: pool}
}

// LoadCapabilityCommands returns active commands in the 'capability' category,
// ordered by (sort_order, slug) for stable UI rendering.
// Returns nil, nil when ah_core.command is missing (non-fatal — supports
// fresh deployments where the seed migration has not run yet).
func (l *CoreCapabilityCommandLoader) LoadCapabilityCommands(ctx context.Context) ([]CoreCommand, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, name, slug, description,
		       COALESCE(argument_hint, '') AS argument_hint,
		       category, handler_type,
		       COALESCE(prompt_template, '') AS prompt_template,
		       COALESCE(tool_slug, '')       AS tool_slug,
		       requires_admin, disable_model_invocation,
		       is_active, sort_order
		  FROM ah_core.command
		 WHERE category = 'capability'
		   AND is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.command not accessible, capability commands unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability commands: %w", err)
	}
	defer rows.Close()

	var commands []CoreCommand
	for rows.Next() {
		var c CoreCommand
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Slug, &c.Description,
			&c.ArgumentHint, &c.Category, &c.HandlerType,
			&c.PromptTemplate, &c.ToolSlug,
			&c.RequiresAdmin, &c.DisableModelInvocation,
			&c.IsActive, &c.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability command: %w", err)
		}
		commands = append(commands, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.command not accessible (post-iter), capability commands unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability commands: %w", err)
	}
	return commands, nil
}

// FindCapabilityCommandBySlug returns a single capability command by slug.
// Returns (CoreCommand{}, false, nil) when not found.
func (l *CoreCapabilityCommandLoader) FindCapabilityCommandBySlug(ctx context.Context, slug string) (CoreCommand, bool, error) {
	all, err := l.LoadCapabilityCommands(ctx)
	if err != nil {
		return CoreCommand{}, false, err
	}
	for _, c := range all {
		if c.Slug == slug {
			return c, true, nil
		}
	}
	return CoreCommand{}, false, nil
}

// ============================================================
// Seed catalog constants — migration 000092 (2026-05-11).
// ============================================================

// SeedCapabilityCommandSlugs is the canonical closed set of capability
// slash command slugs seeded in migration 000092. Each command is adapted
// from Claude Code's built-in command surface for AgentHub's web UX and
// gives tenants immediate access to the capability layer agents.
var SeedCapabilityCommandSlugs = []string{
	"research",
	"analyze",
	"plan",
	"summarize-doc",
	"tasks",
}

// SeedCapabilityCommandCount is the expected row count after migration 000092.
const SeedCapabilityCommandCount = 5

// SeedCapabilityCommandCategory is the category tag for capability commands.
// This is intentionally distinct from the platform command categories in
// SeedExpectedCategories (session/discovery/analytics/collaboration/feedback).
const SeedCapabilityCommandCategory = "capability"

// SeedCapabilityCommandHandlerType is the handler_type for all capability
// commands. All five use 'prompt' — the runner expands the prompt_template
// with the user's arguments before invoking the LLM.
const SeedCapabilityCommandHandlerType = "prompt"

// SeedCapabilityResearchSlug is the slug for the /research command.
const SeedCapabilityResearchSlug = "research"

// SeedCapabilityAnalyzeSlug is the slug for the /analyze command.
const SeedCapabilityAnalyzeSlug = "analyze"

// SeedCapabilityPlanSlug is the slug for the /plan command.
const SeedCapabilityPlanSlug = "plan"
