package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityHandoffConfig is a single per-agent handoff configuration row
// loaded from ah_core.capability_handoff_config. Each row encodes one handoff
// rule for a core capability agent: when to hand off, which agent (or human)
// to hand off to, the message to send, and whether the handoff is automatic.
type CoreCapabilityHandoffConfig struct {
	ID               int64
	AgentSlug        string
	HandoffKey       string
	TriggerCondition string
	TargetAgentSlug  string
	HandoffMessage   string
	IsAutomatic      bool
	CreatedAt        time.Time
}

// ============================================================
// Seed catalog constants — migration 000123 (2026-05-11).
// ============================================================

// SeedHandoffConfigCount is the total number of capability_handoff_config rows
// seeded by migration 000123: 9 rows (3 handoff rules × 3 agents).
const SeedHandoffConfigCount = 9

// SeedHandoffConfigAgentCount is the number of capability agents that have
// seeded handoff config rows in migration 000123 (researcher, analyst, planner).
const SeedHandoffConfigAgentCount = 3

// SeedHumanEscalationCount is the number of human escalation rows seeded by
// migration 000123: one per agent, where target_agent_slug is empty string.
const SeedHumanEscalationCount = 3

// SeedAutomaticHandoffCount is the number of handoff config rows with
// is_automatic=true seeded by migration 000123. All human escalation rows are
// automatic (one per agent).
const SeedAutomaticHandoffCount = 3

// Handoff key constants for well-known capability_handoff_config.handoff_key values.
const (
	// SeedHandoffKeyNeedsResearch is the handoff_key used when an agent needs
	// more information gathered before it can continue (routes to core-researcher).
	SeedHandoffKeyNeedsResearch = "needs_research"

	// SeedHandoffKeyNeedsAnalysis is the handoff_key used when an agent encounters
	// a task requiring data analysis (routes to core-analyst).
	SeedHandoffKeyNeedsAnalysis = "needs_analysis"

	// SeedHandoffKeyNeedsPlanning is the handoff_key used when an agent encounters
	// a task requiring project planning (routes to core-planner).
	SeedHandoffKeyNeedsPlanning = "needs_planning"

	// SeedHandoffKeyHumanEscalation is the handoff_key used when no agent can
	// handle the request and a human must intervene (target_agent_slug is empty).
	SeedHandoffKeyHumanEscalation = "human_escalation"
)

// CoreCapabilityHandoffConfigLoader loads per-agent handoff configuration rows
// from ah_core.capability_handoff_config (seeded by migration 000123).
// All methods are non-fatal when the ah_core schema or the
// capability_handoff_config table is not yet accessible — supports fresh
// deployments where migration 000123 has not yet run.
type CoreCapabilityHandoffConfigLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityHandoffConfigLoader creates a
// CoreCapabilityHandoffConfigLoader backed by pool.
func NewCoreCapabilityHandoffConfigLoader(pool *pgxpool.Pool) *CoreCapabilityHandoffConfigLoader {
	return &CoreCapabilityHandoffConfigLoader{pool: pool}
}

// LoadCapabilityHandoffConfigs returns all rows from
// ah_core.capability_handoff_config ordered by agent_slug, handoff_key.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityHandoffConfigLoader) LoadCapabilityHandoffConfigs(ctx context.Context) ([]CoreCapabilityHandoffConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, handoff_key, trigger_condition, target_agent_slug,
		       handoff_message, is_automatic, created_at
		  FROM ah_core.capability_handoff_config
		 ORDER BY agent_slug, handoff_key`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_handoff_config not accessible, handoff configs unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_handoff_config: %w", err)
	}
	defer rows.Close()

	var configs []CoreCapabilityHandoffConfig
	for rows.Next() {
		var c CoreCapabilityHandoffConfig
		if err := rows.Scan(
			&c.ID, &c.AgentSlug, &c.HandoffKey, &c.TriggerCondition,
			&c.TargetAgentSlug, &c.HandoffMessage, &c.IsAutomatic, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_handoff_config: %w", err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_handoff_config not accessible (post-iter), handoff configs unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability_handoff_config: %w", err)
	}
	return configs, nil
}

// LoadHandoffConfigsForAgent returns all handoff config rows for the given
// agentSlug from ah_core.capability_handoff_config, ordered by handoff_key.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityHandoffConfigLoader) LoadHandoffConfigsForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityHandoffConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, handoff_key, trigger_condition, target_agent_slug,
		       handoff_message, is_automatic, created_at
		  FROM ah_core.capability_handoff_config
		 WHERE agent_slug = $1
		 ORDER BY handoff_key`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_handoff_config not accessible, handoff configs unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_handoff_config for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var configs []CoreCapabilityHandoffConfig
	for rows.Next() {
		var c CoreCapabilityHandoffConfig
		if err := rows.Scan(
			&c.ID, &c.AgentSlug, &c.HandoffKey, &c.TriggerCondition,
			&c.TargetAgentSlug, &c.HandoffMessage, &c.IsAutomatic, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_handoff_config for agent %q: %w", agentSlug, err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_handoff_config not accessible (post-iter), handoff configs unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability_handoff_config for agent %q: %w", agentSlug, err)
	}
	return configs, nil
}

// LoadHumanEscalations returns all handoff config rows where target_agent_slug
// is empty (human escalation — no target agent) from
// ah_core.capability_handoff_config, ordered by agent_slug.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityHandoffConfigLoader) LoadHumanEscalations(ctx context.Context) ([]CoreCapabilityHandoffConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, handoff_key, trigger_condition, target_agent_slug,
		       handoff_message, is_automatic, created_at
		  FROM ah_core.capability_handoff_config
		 WHERE target_agent_slug = ''
		 ORDER BY agent_slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_handoff_config not accessible, human escalations unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_handoff_config (human escalations): %w", err)
	}
	defer rows.Close()

	var configs []CoreCapabilityHandoffConfig
	for rows.Next() {
		var c CoreCapabilityHandoffConfig
		if err := rows.Scan(
			&c.ID, &c.AgentSlug, &c.HandoffKey, &c.TriggerCondition,
			&c.TargetAgentSlug, &c.HandoffMessage, &c.IsAutomatic, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_handoff_config (human escalation): %w", err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_handoff_config not accessible (post-iter), human escalations unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability_handoff_config (human escalations): %w", err)
	}
	return configs, nil
}
