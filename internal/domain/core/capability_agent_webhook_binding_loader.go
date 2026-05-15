package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityAgentWebhookBindingLoader loads capability agent–webhook
// bindings from ah_core.capability_agent_webhook_binding. These 3 bindings
// (sort_order 1–3) link each capability agent (introduced in migration 000091)
// to its corresponding webhook notification template (introduced in migration
// 000097):
//
//   - core-researcher  →  capability-research-complete
//   - core-analyst     →  capability-analysis-done
//   - core-planner     →  capability-tasks-updated
//
// Seeded by migration 000099. The table ah_core.capability_agent_webhook_binding
// is also created by that migration.
//
// The loader is non-fatal when the ah_core schema or the
// capability_agent_webhook_binding table is missing — supports fresh deployments
// where migration 000099 has not yet run.
type CoreCapabilityAgentWebhookBindingLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityAgentWebhookBindingLoader creates a
// CoreCapabilityAgentWebhookBindingLoader backed by pool.
func NewCoreCapabilityAgentWebhookBindingLoader(pool *pgxpool.Pool) *CoreCapabilityAgentWebhookBindingLoader {
	return &CoreCapabilityAgentWebhookBindingLoader{pool: pool}
}

// CoreAgentWebhookBinding is a platform-managed binding that associates a
// capability agent with a webhook notification template it emits upon completing
// its primary task.
type CoreAgentWebhookBinding struct {
	AgentSlug   string
	WebhookSlug string
	IsActive    bool
	SortOrder   int
}

// LoadCapabilityAgentWebhookBindings returns all capability agent–webhook
// bindings from ah_core.capability_agent_webhook_binding, ordered by
// sort_order. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentWebhookBindingLoader) LoadCapabilityAgentWebhookBindings(ctx context.Context) ([]CoreAgentWebhookBinding, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT agent_slug, webhook_slug, is_active, sort_order
		  FROM ah_core.capability_agent_webhook_binding
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_webhook_binding not accessible, capability agent webhook bindings unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability agent webhook bindings: %w", err)
	}
	defer rows.Close()

	var bindings []CoreAgentWebhookBinding
	for rows.Next() {
		var b CoreAgentWebhookBinding
		if err := rows.Scan(&b.AgentSlug, &b.WebhookSlug, &b.IsActive, &b.SortOrder); err != nil {
			return nil, fmt.Errorf("core: scan capability agent webhook binding: %w", err)
		}
		bindings = append(bindings, b)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_webhook_binding not accessible (post-iter), capability agent webhook bindings unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability agent webhook bindings: %w", err)
	}
	return bindings, nil
}

// LoadWebhookBindingsForAgent returns all webhook bindings for the given
// agentSlug from ah_core.capability_agent_webhook_binding, ordered by
// sort_order. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentWebhookBindingLoader) LoadWebhookBindingsForAgent(ctx context.Context, agentSlug string) ([]CoreAgentWebhookBinding, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT agent_slug, webhook_slug, is_active, sort_order
		  FROM ah_core.capability_agent_webhook_binding
		 WHERE agent_slug = $1
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_webhook_binding not accessible, agent webhook bindings unavailable", "err", err, "agent_slug", agentSlug)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query webhook bindings for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var bindings []CoreAgentWebhookBinding
	for rows.Next() {
		var b CoreAgentWebhookBinding
		if err := rows.Scan(&b.AgentSlug, &b.WebhookSlug, &b.IsActive, &b.SortOrder); err != nil {
			return nil, fmt.Errorf("core: scan webhook binding for agent %q: %w", agentSlug, err)
		}
		bindings = append(bindings, b)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_webhook_binding not accessible (post-iter), agent webhook bindings unavailable", "err", err, "agent_slug", agentSlug)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate webhook bindings for agent %q: %w", agentSlug, err)
	}
	return bindings, nil
}

// ============================================================
// Seed catalog constants — migration 000099 (2026-05-11).
// ============================================================

// SeedCapabilityAgentWebhookBindingCount is the expected row count after
// migration 000099. One binding per capability agent.
const SeedCapabilityAgentWebhookBindingCount = 3

// SeedCapabilityAgentWebhookBindings is the canonical closed set of capability
// agent–webhook bindings seeded in migration 000099. Each entry pairs an agent
// slug with the webhook notification template slug it emits.
var SeedCapabilityAgentWebhookBindings = []struct {
	AgentSlug   string
	WebhookSlug string
}{
	{"core-researcher", "capability-research-complete"},
	{"core-analyst", "capability-analysis-done"},
	{"core-planner", "capability-tasks-updated"},
}

// SeedResearcherWebhookBinding is the human-readable label for the
// core-researcher → capability-research-complete binding.
// Used in logs and test descriptions.
const SeedResearcherWebhookBinding = "core-researcher→capability-research-complete"

// SeedAnalystWebhookBinding is the human-readable label for the
// core-analyst → capability-analysis-done binding.
// Used in logs and test descriptions.
const SeedAnalystWebhookBinding = "core-analyst→capability-analysis-done"

// SeedPlannerWebhookBinding is the human-readable label for the
// core-planner → capability-tasks-updated binding.
// Used in logs and test descriptions.
const SeedPlannerWebhookBinding = "core-planner→capability-tasks-updated"
