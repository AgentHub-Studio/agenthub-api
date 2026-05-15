package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreHook represents a platform-managed default hook from ah_core.hook.
// Default hooks are baseline behaviours every tenant inherits — adapted
// from Claude Code's 27 lifecycle hooks (PDF Section 6.1) for AgentHub's
// web reality. Only PROMPT-TYPE hooks are seeded (zero side effects); HTTP
// hooks are never seeded by the platform to avoid surprising deployments.
//
// Inspired by PDF arXiv:2604.14228v1 Section 6.1 + Section 5.3.
type CoreHook struct {
	ID                     uuid.UUID
	Name                   string
	Slug                   string
	Description            string
	Event                  string // ExtendedHookEvent name (PreToolUse, PostToolUse, ...)
	HookType               string // "prompt" | "http" — seeds only "prompt"
	InjectText             string
	Matcher                string // "" = applies to all invocations of the event
	Priority               int
	RequiresAdminToDisable bool
	IsActive               bool
	SortOrder              int
}

// CoreHookLoader loads platform-managed default hooks from the global
// ah_core schema. Like other core loaders, non-fatal when schema missing.
type CoreHookLoader struct {
	pool *pgxpool.Pool
}

// NewCoreHookLoader creates a CoreHookLoader backed by the given pool.
func NewCoreHookLoader(pool *pgxpool.Pool) *CoreHookLoader {
	return &CoreHookLoader{pool: pool}
}

// LoadAll returns all active platform hooks from ah_core.hook, ordered by
// (priority DESC, sort_order, slug) so the highest-priority hooks fire
// first within an event.
func (l *CoreHookLoader) LoadAll(ctx context.Context) ([]CoreHook, error) {
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
		 WHERE is_active = true
		 ORDER BY priority DESC, sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.hook not accessible, core hooks unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query hooks: %w", err)
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
			return nil, fmt.Errorf("core: scan hook: %w", err)
		}
		hooks = append(hooks, h)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.hook not accessible (post-iter), core hooks unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate hooks: %w", err)
	}
	return hooks, nil
}

// LoadByEvent returns active hooks for a specific lifecycle event,
// preserving the priority ordering. Used by the runner to find hooks
// to dispatch when a given event fires.
func (l *CoreHookLoader) LoadByEvent(ctx context.Context, event string) ([]CoreHook, error) {
	all, err := l.LoadAll(ctx)
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

// FindBySlug returns one hook by slug.
func (l *CoreHookLoader) FindBySlug(ctx context.Context, slug string) (CoreHook, bool, error) {
	all, err := l.LoadAll(ctx)
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

// SeedExpectedHookSlugs is the canonical list of slugs the seed migration
// 000010_seed_hooks installs. Tests assert this list matches actual rows.
var SeedExpectedHookSlugs = []string{
	// safety (4 — admin-only disable)
	"safety-pretooluse-confirm-destructive",
	"safety-pretooluse-redact-secrets",
	"safety-permissiondenied-explain",
	"safety-posttooluseFailure-acknowledge",
	// lifecycle (3)
	"lifecycle-sessionstart-greeting",
	"lifecycle-userpromptsubmit-clarify-if-vague",
	"lifecycle-stop-summarize-if-long",
	// context (2)
	"context-precompact-preserve-decisions",
	"context-postcompact-acknowledge",
	// coordination (2)
	"coord-subagentstop-summarize-result",
	"coord-taskcompleted-acknowledge",
}

// SeedExpectedHookEvents is the closed set of lifecycle events the seed
// targets. Must each match an ExtendedHookEvent constant in agentic.
var SeedExpectedHookEvents = []string{
	"PreToolUse",
	"PostToolUseFailure",
	"PermissionDenied",
	"SessionStart",
	"UserPromptSubmit",
	"Stop",
	"PreCompact",
	"PostCompact",
	"SubagentStop",
	"TaskCompleted",
}

// SeedAdminOnlyDisableHookSlugs lists the safety hooks tenants cannot
// disable without admin role.
var SeedAdminOnlyDisableHookSlugs = []string{
	"safety-pretooluse-confirm-destructive",
	"safety-pretooluse-redact-secrets",
	"safety-permissiondenied-explain",
	"safety-posttooluseFailure-acknowledge",
}
