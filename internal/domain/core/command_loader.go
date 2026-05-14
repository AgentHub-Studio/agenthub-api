package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgUndefinedTable is SQLSTATE 42P01 — the relation does not exist.
// Treated as non-fatal in CoreCommandLoader to support fresh deployments
// where the seed migration has not run yet (or test scenarios that drop
// the table to verify the down migration).
const pgUndefinedTable = "42P01"

// isUndefinedRelation returns true when err indicates the queried table
// does not exist. Both pgx Query-time and Scan/rows.Err()-time can
// surface 42P01 depending on the driver path.
func isUndefinedRelation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUndefinedTable {
		return true
	}
	return false
}

// CoreCommand represents a platform-managed slash command from ah_core.command.
// These are WEB-friendly commands available to every tenant — adapted from the
// Claude Code commands surface but stripped of CLI/IDE-only items
// (/init, /add-dir, /mcp, /ide, /sandbox, /vim, /editor, /shell).
//
// Inspired by PDF arXiv:2604.14228v1 Section 6.1 (Plugin manifest component
// types: commands).
type CoreCommand struct {
	ID                     uuid.UUID
	Name                   string
	Slug                   string
	Description            string
	ArgumentHint           string
	Category               string
	HandlerType            string // "builtin" | "prompt" | "tool"
	PromptTemplate         string
	ToolSlug               string
	RequiresAdmin          bool
	DisableModelInvocation bool
	IsActive               bool
	SortOrder              int
}

// CoreCommandLoader loads slash commands from the global ah_core schema.
// Like CoreToolLoader, it is non-fatal: if the schema does not exist
// (fresh deployments, test environments without the migration applied),
// it returns an empty slice with a warning log.
type CoreCommandLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCommandLoader creates a CoreCommandLoader backed by the given pool.
func NewCoreCommandLoader(pool *pgxpool.Pool) *CoreCommandLoader {
	return &CoreCommandLoader{pool: pool}
}

// LoadAll returns all active commands from ah_core.command, ordered by
// (category, sort_order, slug) for stable UI rendering.
// Returns an empty slice (not an error) when the ah_core.command table is
// missing — supports running the API before the seed migration has applied.
func (l *CoreCommandLoader) LoadAll(ctx context.Context) ([]CoreCommand, error) {
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
		 WHERE is_active = true
		 ORDER BY category, sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.command not accessible, core commands unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query commands: %w", err)
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
			return nil, fmt.Errorf("core: scan command: %w", err)
		}
		commands = append(commands, c)
	}
	if err := rows.Err(); err != nil {
		// pgx surfaces some errors (including missing-relation in certain
		// driver paths) only at iteration time. Honour the non-fatal
		// contract here too.
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.command not accessible (post-iter), core commands unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate commands: %w", err)
	}
	return commands, nil
}

// FindBySlug returns one command by slug. Useful for the runner when the
// user types "/<slug>" and we need to dispatch the handler.
// Returns (CoreCommand{}, false, nil) if not found.
func (l *CoreCommandLoader) FindBySlug(ctx context.Context, slug string) (CoreCommand, bool, error) {
	all, err := l.LoadAll(ctx)
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

// SeedExpectedSlugs is the canonical list of slugs the seed migration
// 000008_seed_commands installs. Tests assert this list matches the actual
// rows so a missing/typo'd seed entry is caught at test time.
//
// Keep this list in sync with migrations/ah_core/000008_seed_commands.up.sql.
var SeedExpectedSlugs = []string{
	// session
	"help", "clear", "reset", "summarize", "export", "resume", "branch",
	// discovery
	"skills", "agents", "memory",
	// analytics
	"cost",
	// collaboration
	"share",
	// feedback
	"feedback",
}

// SeedExpectedCategories is the closed set of categories used by the seed.
// New categories require updating the UI palette — this list keeps the
// seed and the front-end in sync.
var SeedExpectedCategories = []string{
	"session", "discovery", "analytics", "collaboration", "feedback",
}
