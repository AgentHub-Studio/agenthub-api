package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityAgentMemoryConfigLoader loads per-agent memory settings
// from ah_core.capability_agent_memory_config. These 9 rows (seeded by migration
// 000102) provide role-specific memory configuration for the three capability
// agents introduced in migration 000091, adapted from §9.1 session persistence
// channels (session_transcript / global_prompt_history / subagent_sidechain):
//
//   - core-researcher  (3 configs: max_context_tokens, summary_strategy, persistence_scope)
//   - core-analyst     (3 configs: max_context_tokens, summary_strategy, persistence_scope)
//   - core-planner     (3 configs: max_context_tokens, summary_strategy, persistence_scope)
//
// Non-fatal when the ah_core schema or the capability_agent_memory_config table
// is missing — supports fresh deployments where migration 000102 has not yet run.
type CoreCapabilityAgentMemoryConfigLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityAgentMemoryConfigLoader creates a
// CoreCapabilityAgentMemoryConfigLoader backed by pool.
func NewCoreCapabilityAgentMemoryConfigLoader(pool *pgxpool.Pool) *CoreCapabilityAgentMemoryConfigLoader {
	return &CoreCapabilityAgentMemoryConfigLoader{pool: pool}
}

// CoreAgentMemoryConfig is a single per-agent memory configuration setting.
// Captures the agent, config key, value, optional description, and sort position
// within the agent's memory config list.
type CoreAgentMemoryConfig struct {
	AgentSlug   string
	ConfigKey   string
	ConfigValue string
	Description string
	SortOrder   int
}

// LoadCapabilityAgentMemoryConfigs returns all capability agent memory config rows
// from ah_core.capability_agent_memory_config WHERE agent_slug = ANY($1), ordered
// by agent_slug, sort_order. Returns nil, nil when the table is not accessible
// (non-fatal).
func (l *CoreCapabilityAgentMemoryConfigLoader) LoadCapabilityAgentMemoryConfigs(ctx context.Context) ([]CoreAgentMemoryConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT agent_slug, config_key, config_value, COALESCE(description, ''), sort_order
		  FROM ah_core.capability_agent_memory_config
		 WHERE agent_slug = ANY($1)
		 ORDER BY agent_slug, sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityAgentMemoryConfigAgentSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_memory_config not accessible, capability agent memory configs unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability agent memory configs: %w", err)
	}
	defer rows.Close()

	var configs []CoreAgentMemoryConfig
	for rows.Next() {
		var c CoreAgentMemoryConfig
		if err := rows.Scan(
			&c.AgentSlug, &c.ConfigKey, &c.ConfigValue, &c.Description, &c.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability agent memory config: %w", err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_memory_config not accessible (post-iter), capability agent memory configs unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability agent memory configs: %w", err)
	}
	return configs, nil
}

// LoadMemoryConfigsForAgent returns all capability agent memory config rows for the
// given agentSlug from ah_core.capability_agent_memory_config, ordered by
// sort_order. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentMemoryConfigLoader) LoadMemoryConfigsForAgent(ctx context.Context, agentSlug string) ([]CoreAgentMemoryConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT agent_slug, config_key, config_value, COALESCE(description, ''), sort_order
		  FROM ah_core.capability_agent_memory_config
		 WHERE agent_slug = $1
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_memory_config not accessible, memory configs for agent unavailable", "err", err, "agent_slug", agentSlug)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query memory configs for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var configs []CoreAgentMemoryConfig
	for rows.Next() {
		var c CoreAgentMemoryConfig
		if err := rows.Scan(
			&c.AgentSlug, &c.ConfigKey, &c.ConfigValue, &c.Description, &c.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan memory config for agent %q: %w", agentSlug, err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_memory_config not accessible (post-iter), memory configs for agent unavailable", "err", err, "agent_slug", agentSlug)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate memory configs for agent %q: %w", agentSlug, err)
	}
	return configs, nil
}

// ============================================================
// Seed catalog constants — migration 000102 (2026-05-11).
// ============================================================

// SeedCapabilityAgentMemoryConfigCount is the expected total row count after
// migration 000102. Nine memory config rows across three capability agents
// (3 configs each: max_context_tokens, summary_strategy, persistence_scope).
const SeedCapabilityAgentMemoryConfigCount = 9

// SeedCapabilityAgentMemoryConfigAgentSlugs is the canonical closed set of
// capability agent slugs that have memory config rows seeded in migration 000102.
var SeedCapabilityAgentMemoryConfigAgentSlugs = []string{
	"core-researcher",
	"core-analyst",
	"core-planner",
}

// SeedResearcherMemoryConfigCount is the number of memory config rows seeded
// for the core-researcher agent in migration 000102.
const SeedResearcherMemoryConfigCount = 3

// SeedAnalystMemoryConfigCount is the number of memory config rows seeded
// for the core-analyst agent in migration 000102.
const SeedAnalystMemoryConfigCount = 3

// SeedPlannerMemoryConfigCount is the number of memory config rows seeded
// for the core-planner agent in migration 000102.
const SeedPlannerMemoryConfigCount = 3

// SeedMemoryConfigMaxContextTokens is the config key for the maximum token
// count in the active context window.
const SeedMemoryConfigMaxContextTokens = "max_context_tokens"

// SeedMemoryConfigSummaryStrategy is the config key for the summarisation
// strategy applied as the context window approaches its limit.
const SeedMemoryConfigSummaryStrategy = "summary_strategy"

// SeedMemoryConfigPersistenceScope is the config key for the persistence scope
// that controls how long memory is retained (session vs. global).
const SeedMemoryConfigPersistenceScope = "persistence_scope"

// SeedSummaryStrategyProgressive is the summary strategy for core-researcher:
// summarise progressively as the context grows.
const SeedSummaryStrategyProgressive = "progressive"

// SeedSummaryStrategySnapshot is the summary strategy for core-analyst:
// take snapshot summaries at key analysis checkpoints.
const SeedSummaryStrategySnapshot = "snapshot"

// SeedSummaryStrategyAppendOnly is the summary strategy for core-planner:
// append-only task log that is never rewritten.
const SeedSummaryStrategyAppendOnly = "append_only"

// SeedPersistenceScopeSession is the persistence scope for core-researcher and
// core-planner: memory is retained only for the current session.
const SeedPersistenceScopeSession = "session"

// SeedPersistenceScopeGlobal is the persistence scope for core-analyst: memory
// persists globally across sessions to support longitudinal analysis.
const SeedPersistenceScopeGlobal = "global"
