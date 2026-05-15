package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityToolPermission is a single per-agent tool permission declaration
// seeded by migration 000118. Each row specifies whether a capability agent is
// allowed to invoke a given tool autonomously (allow), never (deny), or only
// after the user explicitly approves (require_approval). The design is adapted
// from the Claude Code permission mode system.
type CoreCapabilityToolPermission struct {
	// ID is the auto-generated BIGSERIAL primary key.
	ID int64
	// AgentSlug identifies the capability agent that owns this permission
	// (e.g. "core-researcher", "core-analyst", "core-planner").
	AgentSlug string
	// ToolSlug identifies the tool being governed by this permission
	// (e.g. "core-web-search", "core-web-fetch", "core-doc-search",
	// "core-subagent-run").
	ToolSlug string
	// PermissionMode is the access mode for this agent/tool pair.
	// One of "allow", "deny", or "require_approval".
	PermissionMode string
	// Rationale is a human-readable explanation of why this permission mode
	// was chosen for this agent/tool pair.
	Rationale string
	// CreatedAt is the UTC timestamp when the row was created.
	CreatedAt time.Time
}

// CoreCapabilityToolPermissionLoader loads per-agent tool permission
// declarations from ah_core.capability_tool_permission. The table is seeded
// by migration 000118 with 12 rows (4 per capability agent). All methods are
// non-fatal when the table or schema is missing (supports fresh deployments
// before the migration runs).
type CoreCapabilityToolPermissionLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityToolPermissionLoader creates a
// CoreCapabilityToolPermissionLoader backed by pool.
func NewCoreCapabilityToolPermissionLoader(pool *pgxpool.Pool) *CoreCapabilityToolPermissionLoader {
	return &CoreCapabilityToolPermissionLoader{pool: pool}
}

// LoadCapabilityToolPermissions returns all tool permission rows from
// ah_core.capability_tool_permission ordered by agent_slug, tool_slug asc.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityToolPermissionLoader) LoadCapabilityToolPermissions(ctx context.Context) ([]CoreCapabilityToolPermission, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, tool_slug, permission_mode, rationale, created_at
		  FROM ah_core.capability_tool_permission
		 ORDER BY agent_slug, tool_slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_tool_permission not accessible, tool permissions unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability tool permissions: %w", err)
	}
	defer rows.Close()

	var permissions []CoreCapabilityToolPermission
	for rows.Next() {
		var p CoreCapabilityToolPermission
		if err := rows.Scan(
			&p.ID, &p.AgentSlug, &p.ToolSlug, &p.PermissionMode, &p.Rationale, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability tool permission: %w", err)
		}
		permissions = append(permissions, p)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_tool_permission not accessible (post-iter), tool permissions unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability tool permissions: %w", err)
	}
	return permissions, nil
}

// LoadToolPermissionsForAgent returns the tool permission rows from
// ah_core.capability_tool_permission WHERE agent_slug = $1, ordered by
// tool_slug asc. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityToolPermissionLoader) LoadToolPermissionsForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityToolPermission, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, tool_slug, permission_mode, rationale, created_at
		  FROM ah_core.capability_tool_permission
		 WHERE agent_slug = $1
		 ORDER BY tool_slug`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_tool_permission not accessible, tool permissions unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query tool permissions for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var permissions []CoreCapabilityToolPermission
	for rows.Next() {
		var p CoreCapabilityToolPermission
		if err := rows.Scan(
			&p.ID, &p.AgentSlug, &p.ToolSlug, &p.PermissionMode, &p.Rationale, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan tool permission for agent %q: %w", agentSlug, err)
		}
		permissions = append(permissions, p)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_tool_permission not accessible (post-iter), tool permissions unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate tool permissions for agent %q: %w", agentSlug, err)
	}
	return permissions, nil
}

// GetToolPermission returns the permission_mode for the given agentSlug/toolSlug
// pair. Returns ("", false, nil) when no row exists or the table is not
// accessible (non-fatal). Returns (mode, true, nil) when found.
func (l *CoreCapabilityToolPermissionLoader) GetToolPermission(ctx context.Context, agentSlug, toolSlug string) (string, bool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return "", false, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT permission_mode
		  FROM ah_core.capability_tool_permission
		 WHERE agent_slug = $1
		   AND tool_slug  = $2
		 LIMIT 1`

	var mode string
	err = conn.QueryRow(ctx, query, agentSlug, toolSlug).Scan(&mode)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_tool_permission not accessible, returning not-found",
				"agent_slug", agentSlug, "tool_slug", toolSlug, "err", err)
			return "", false, nil
		}
		// pgx returns "no rows in result set" when zero rows match.
		if err.Error() == "no rows in result set" {
			return "", false, nil
		}
		return "", false, fmt.Errorf("core: get tool permission for agent %q tool %q: %w", agentSlug, toolSlug, err)
	}
	return mode, true, nil
}

// ============================================================
// Seed catalog constants — migration 000118 (2026-05-11).
// ============================================================

// SeedToolPermissionCount is the expected total row count after migration
// 000118. Twelve tool permission rows — four per capability agent (researcher,
// analyst, planner) — covering the four core tools: web-search, web-fetch,
// doc-search, and subagent-run.
const SeedToolPermissionCount = 12

// SeedToolPermissionAgentCount is the number of capability agents that have
// seeded tool permission rows in migration 000118 (researcher, analyst, planner).
const SeedToolPermissionAgentCount = 3

// Permission mode constants — migration 000118.

// SeedPermModeAllow is the permission_mode value that grants the agent
// autonomous invocation rights for the tool ("allow"). The agent may call this
// tool at any time without user confirmation.
const SeedPermModeAllow = "allow"

// SeedPermModeDeny is the permission_mode value that prohibits the agent from
// invoking the tool entirely ("deny"). The tool is not available to the agent
// under any circumstances.
const SeedPermModeDeny = "deny"

// SeedPermModeRequireApproval is the permission_mode value that requires
// explicit user approval before the agent may invoke the tool
// ("require_approval"). The agent pauses and presents the pending tool call to
// the user; execution continues only after the user accepts.
const SeedPermModeRequireApproval = "require_approval"

// Tool slug constants — migration 000118.

// SeedToolSlugWebSearch is the slug of the web search tool that capability
// agents may be granted permission to invoke ("core-web-search").
const SeedToolSlugWebSearch = "core-web-search"

// SeedToolSlugWebFetch is the slug of the web fetch tool that capability
// agents may be granted permission to invoke ("core-web-fetch").
const SeedToolSlugWebFetch = "core-web-fetch"

// SeedToolSlugDocSearch is the slug of the document search tool that capability
// agents may be granted permission to invoke ("core-doc-search").
const SeedToolSlugDocSearch = "core-doc-search"

// SeedToolSlugSubagentRun is the slug of the subagent delegation tool that
// capability agents may be granted permission to invoke ("core-subagent-run").
const SeedToolSlugSubagentRun = "core-subagent-run"

// Semantic subagent permission constants — migration 000118.
// These alias the per-agent subagent-run permission mode to make intent
// explicit at call sites.

// SeedResearcherSubagentPermission is the permission mode assigned to
// core-researcher for the core-subagent-run tool. The researcher may delegate
// to subagents only after the user explicitly approves each delegation.
const SeedResearcherSubagentPermission = SeedPermModeRequireApproval

// SeedAnalystSubagentPermission is the permission mode assigned to
// core-analyst for the core-subagent-run tool. The analyst never delegates;
// it maintains a single-thread analysis focus to ensure coherent results.
const SeedAnalystSubagentPermission = SeedPermModeDeny

// SeedPlannerSubagentPermission is the permission mode assigned to
// core-planner for the core-subagent-run tool. The planner may freely delegate
// subtasks to specialised agents — subagent orchestration is its primary role.
const SeedPlannerSubagentPermission = SeedPermModeAllow
