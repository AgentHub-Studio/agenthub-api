package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityUIHintLoader loads contextual UI hints from
// ah_core.capability_ui_hint. These 6 rows (seeded by migration 000107)
// define hints shown to new users in the AgentHub web UI when interacting
// with capability agents. Hints are adapted from Claude Code's in-app
// guidance patterns:
//
//   - hint-researcher-start-tip      (agent_chat_start,  tip,  core-researcher)
//   - hint-researcher-citation-tip   (post_tool_result,  info, core-researcher)
//   - hint-analyst-doc-upload-tip    (agent_chat_start,  tip,  core-analyst)
//   - hint-analyst-confidence-tip    (post_tool_result,  tip,  core-analyst)
//   - hint-planner-task-tip          (agent_chat_start,  info, core-planner)
//   - hint-planner-breakdown-tip     (agent_chat_start,  tip,  core-planner)
//
// Non-fatal when the ah_core schema or the capability_ui_hint table is
// missing — supports fresh deployments where migration 000107 has not yet run.
type CoreCapabilityUIHintLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityUIHintLoader creates a CoreCapabilityUIHintLoader backed
// by pool.
func NewCoreCapabilityUIHintLoader(pool *pgxpool.Pool) *CoreCapabilityUIHintLoader {
	return &CoreCapabilityUIHintLoader{pool: pool}
}

// CoreCapabilityUIHint is a single UI hint definition. Captures the slug,
// display title, hint body text, the trigger context (when to show it), the
// target agent slug (nil means global), the hint type ('tip' or 'info'),
// whether the user can dismiss it, and the display order.
type CoreCapabilityUIHint struct {
	Slug           string
	Title          string
	Body           string
	TriggerContext string
	AgentSlug      string
	HintType       string
	IsDismissable  bool
	SortOrder      int
}

// LoadCapabilityUIHints returns all UI hint rows from
// ah_core.capability_ui_hint WHERE slug = ANY($1), ordered by sort_order.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityUIHintLoader) LoadCapabilityUIHints(ctx context.Context) ([]CoreCapabilityUIHint, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, title, body, trigger_context,
		       COALESCE(agent_slug, ''), hint_type, is_dismissable, sort_order
		  FROM ah_core.capability_ui_hint
		 WHERE slug = ANY($1)
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityUIHintSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_ui_hint not accessible, UI hints unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability UI hints: %w", err)
	}
	defer rows.Close()

	var hints []CoreCapabilityUIHint
	for rows.Next() {
		var h CoreCapabilityUIHint
		if err := rows.Scan(
			&h.Slug, &h.Title, &h.Body, &h.TriggerContext,
			&h.AgentSlug, &h.HintType, &h.IsDismissable, &h.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability UI hint: %w", err)
		}
		hints = append(hints, h)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_ui_hint not accessible (post-iter), UI hints unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability UI hints: %w", err)
	}
	return hints, nil
}

// LoadUIHintsForAgent returns all UI hint rows from
// ah_core.capability_ui_hint WHERE agent_slug = $1, ordered by sort_order.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityUIHintLoader) LoadUIHintsForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityUIHint, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, title, body, trigger_context,
		       COALESCE(agent_slug, ''), hint_type, is_dismissable, sort_order
		  FROM ah_core.capability_ui_hint
		 WHERE agent_slug = $1
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_ui_hint not accessible, agent UI hints unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query UI hints for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var hints []CoreCapabilityUIHint
	for rows.Next() {
		var h CoreCapabilityUIHint
		if err := rows.Scan(
			&h.Slug, &h.Title, &h.Body, &h.TriggerContext,
			&h.AgentSlug, &h.HintType, &h.IsDismissable, &h.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan agent UI hint: %w", err)
		}
		hints = append(hints, h)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_ui_hint not accessible (post-iter), agent UI hints unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate agent UI hints: %w", err)
	}
	return hints, nil
}

// LoadUIHintsByTrigger returns all UI hint rows from
// ah_core.capability_ui_hint WHERE trigger_context = $1, ordered by sort_order.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityUIHintLoader) LoadUIHintsByTrigger(ctx context.Context, triggerContext string) ([]CoreCapabilityUIHint, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, title, body, trigger_context,
		       COALESCE(agent_slug, ''), hint_type, is_dismissable, sort_order
		  FROM ah_core.capability_ui_hint
		 WHERE trigger_context = $1
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, triggerContext)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_ui_hint not accessible, trigger UI hints unavailable",
				"trigger_context", triggerContext, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query UI hints by trigger %q: %w", triggerContext, err)
	}
	defer rows.Close()

	var hints []CoreCapabilityUIHint
	for rows.Next() {
		var h CoreCapabilityUIHint
		if err := rows.Scan(
			&h.Slug, &h.Title, &h.Body, &h.TriggerContext,
			&h.AgentSlug, &h.HintType, &h.IsDismissable, &h.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan trigger UI hint: %w", err)
		}
		hints = append(hints, h)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_ui_hint not accessible (post-iter), trigger UI hints unavailable",
				"trigger_context", triggerContext, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate trigger UI hints: %w", err)
	}
	return hints, nil
}

// ============================================================
// Seed catalog constants — migration 000107 (2026-05-11).
// ============================================================

// SeedCapabilityUIHintCount is the expected total row count after migration
// 000107. Six UI hint rows — two per capability agent (researcher, analyst,
// planner), covering the most common guidance moments in a new user session.
const SeedCapabilityUIHintCount = 6

// SeedCapabilityUIHintSlugs is the canonical closed set of slugs seeded by
// migration 000107, in sort_order ascending.
var SeedCapabilityUIHintSlugs = []string{
	"hint-researcher-start-tip",
	"hint-researcher-citation-tip",
	"hint-analyst-doc-upload-tip",
	"hint-analyst-confidence-tip",
	"hint-planner-task-tip",
	"hint-planner-breakdown-tip",
}

// Per-agent UI hint counts.

// SeedResearcherUIHintCount is the number of UI hints for core-researcher.
const SeedResearcherUIHintCount = 2

// SeedAnalystUIHintCount is the number of UI hints for core-analyst.
const SeedAnalystUIHintCount = 2

// SeedPlannerUIHintCount is the number of UI hints for core-planner.
const SeedPlannerUIHintCount = 2

// Hint type constants.

// SeedUIHintTypeTip is the hint_type value for actionable guidance hints.
const SeedUIHintTypeTip = "tip"

// SeedUIHintTypeInfo is the hint_type value for informational (non-actionable) hints.
const SeedUIHintTypeInfo = "info"

// Trigger context constants.

// SeedUIHintTriggerChatStart is the trigger_context shown at the start of an
// agent chat session, before the first user message is sent.
const SeedUIHintTriggerChatStart = "agent_chat_start"

// SeedUIHintTriggerPostToolResult is the trigger_context shown after a tool
// result is returned during an agent run, surfacing follow-up guidance.
const SeedUIHintTriggerPostToolResult = "post_tool_result"

// Type counts.

// SeedUIHintTipCount is the number of hints with hint_type='tip' (4 of 6).
const SeedUIHintTipCount = 4

// SeedUIHintInfoCount is the number of hints with hint_type='info' (2 of 6).
const SeedUIHintInfoCount = 2
