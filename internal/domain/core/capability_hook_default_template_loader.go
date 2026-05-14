package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityHookLoader loads capability-layer lifecycle hooks from
// ah_core.hook. These hooks instrument the capability agent tier introduced in
// migrations 000090 (skills/tools), 000091 (agents), and 000092 (commands).
// Seeded by migration 000093.
//
// Distinct from CoreHookLoader (which loads the full platform baseline of
// 11 safety/lifecycle/context/coordination hooks). This loader targets only
// the 4 capability hooks that enforce citation discipline, query validation,
// and task-context awareness for the core-researcher / core-analyst /
// core-planner capability agents.
//
// Non-fatal when the ah_core schema or hook table is missing — supports
// fresh deployments where 000010 has not yet run.
type CoreCapabilityHookLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityHookLoader creates a CoreCapabilityHookLoader backed by pool.
func NewCoreCapabilityHookLoader(pool *pgxpool.Pool) *CoreCapabilityHookLoader {
	return &CoreCapabilityHookLoader{pool: pool}
}

// LoadCapabilityHooks returns all active capability-layer hooks from
// ah_core.hook, identified by their canonical slug set.
// Ordered by (priority DESC, sort_order, slug) to match the platform hook
// ordering contract used by the runner.
// Returns nil, nil when the ah_core.hook table is not accessible.
func (l *CoreCapabilityHookLoader) LoadCapabilityHooks(ctx context.Context) ([]CoreHook, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, name, slug,
		       COALESCE(description, '') AS description,
		       event, hook_type,
		       COALESCE(inject_text, '') AS inject_text,
		       matcher, priority,
		       requires_admin_to_disable, is_active, sort_order
		  FROM ah_core.hook
		 WHERE slug = ANY($1)
		   AND is_active = true
		 ORDER BY priority DESC, sort_order, slug`

	rows, err := conn.Query(ctx, query, SeedCapabilityHookSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.hook not accessible, capability hooks unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability hooks: %w", err)
	}
	defer rows.Close()

	var hooks []CoreHook
	for rows.Next() {
		var h CoreHook
		if err := rows.Scan(
			&h.ID, &h.Name, &h.Slug, &h.Description,
			&h.Event, &h.HookType, &h.InjectText,
			&h.Matcher, &h.Priority,
			&h.RequiresAdminToDisable, &h.IsActive, &h.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability hook: %w", err)
		}
		hooks = append(hooks, h)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.hook not accessible (post-iter), capability hooks unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability hooks: %w", err)
	}
	return hooks, nil
}

// LoadCapabilityHooksByEvent returns active capability hooks for a specific
// lifecycle event. Preserves priority ordering from LoadCapabilityHooks.
func (l *CoreCapabilityHookLoader) LoadCapabilityHooksByEvent(ctx context.Context, event string) ([]CoreHook, error) {
	all, err := l.LoadCapabilityHooks(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreHook
	for _, h := range all {
		if h.Event == event {
			matched = append(matched, h)
		}
	}
	return matched, nil
}

// FindCapabilityHookBySlug returns a single capability hook by slug.
// Returns (CoreHook{}, false, nil) when not found.
func (l *CoreCapabilityHookLoader) FindCapabilityHookBySlug(ctx context.Context, slug string) (CoreHook, bool, error) {
	all, err := l.LoadCapabilityHooks(ctx)
	if err != nil {
		return CoreHook{}, false, err
	}
	for _, h := range all {
		if h.Slug == slug {
			return h, true, nil
		}
	}
	return CoreHook{}, false, nil
}

// ============================================================
// Seed catalog constants — migration 000093 (2026-05-11).
// ============================================================

// SeedCapabilityHookSlugs is the canonical closed set of capability hook slugs
// seeded in migration 000093. Each hook instruments a distinct phase of a
// capability agent invocation: PreToolUse (validate query), PostToolUse
// (cite web sources, index doc citations), SessionStart (load task context).
var SeedCapabilityHookSlugs = []string{
	"capability-posttooluse-cite-web-sources",
	"capability-posttooluse-index-doc-citations",
	"capability-pretooluse-validate-search-query",
	"capability-sessionstart-load-task-context",
}

// SeedCapabilityHookCount is the expected row count after migration 000093.
const SeedCapabilityHookCount = 4

// SeedCapabilityHookEvents is the closed set of lifecycle events used by the
// 4 capability hooks. Must each match an ExtendedHookEvent constant in agentic.
var SeedCapabilityHookEvents = []string{
	"PostToolUse",
	"PreToolUse",
	"SessionStart",
}

// SeedCapabilityWebResearchHookSlugs are the two hooks that apply specifically
// to web research operations (cite-web-sources uses matcher core-web-search,
// core-web-fetch; validate-search-query uses matcher core-web-search).
var SeedCapabilityWebResearchHookSlugs = []string{
	"capability-posttooluse-cite-web-sources",
	"capability-pretooluse-validate-search-query",
}

// SeedCapabilityDocAnalysisHookSlugs contains the single hook that applies to
// document analysis operations (matcher: core-doc-search).
var SeedCapabilityDocAnalysisHookSlugs = []string{
	"capability-posttooluse-index-doc-citations",
}

// SeedCapabilitySessionHookSlugs contains the single SessionStart hook that
// loads existing task context when a capability session begins.
var SeedCapabilitySessionHookSlugs = []string{
	"capability-sessionstart-load-task-context",
}
