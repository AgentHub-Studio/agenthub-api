package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityAgentToolConfigLoader loads per-agent tool parameter overrides
// from ah_core.capability_agent_tool_config. These 7 rows (seeded by migration
// 000101) provide role-specific tool invocation parameters for the three
// capability agents introduced in migration 000091:
//
//   - core-researcher  (3 overrides: max_results, timeout_seconds, similarity_threshold)
//   - core-analyst     (2 overrides: max_results, max_tokens)
//   - core-planner     (2 overrides: max_items, include_completed)
//
// Non-fatal when the ah_core schema or the capability_agent_tool_config table
// is missing — supports fresh deployments where migration 000101 has not yet run.
type CoreCapabilityAgentToolConfigLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityAgentToolConfigLoader creates a
// CoreCapabilityAgentToolConfigLoader backed by pool.
func NewCoreCapabilityAgentToolConfigLoader(pool *pgxpool.Pool) *CoreCapabilityAgentToolConfigLoader {
	return &CoreCapabilityAgentToolConfigLoader{pool: pool}
}

// CoreAgentToolConfig is a single per-agent tool parameter override. Captures
// the agent, tool, parameter key, and the override value, plus a sort position
// within the agent's tool config list.
type CoreAgentToolConfig struct {
	AgentSlug  string
	ToolSlug   string
	ParamKey   string
	ParamValue string
	SortOrder  int
}

// LoadCapabilityAgentToolConfigs returns all capability agent tool config rows
// from ah_core.capability_agent_tool_config WHERE agent_slug = ANY($1), ordered
// by agent_slug, sort_order. Returns nil, nil when the table is not accessible
// (non-fatal).
func (l *CoreCapabilityAgentToolConfigLoader) LoadCapabilityAgentToolConfigs(ctx context.Context) ([]CoreAgentToolConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT agent_slug, tool_slug, param_key, param_value, sort_order
		  FROM ah_core.capability_agent_tool_config
		 WHERE agent_slug = ANY($1)
		 ORDER BY agent_slug, sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityAgentToolConfigAgentSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_tool_config not accessible, capability agent tool configs unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability agent tool configs: %w", err)
	}
	defer rows.Close()

	var configs []CoreAgentToolConfig
	for rows.Next() {
		var c CoreAgentToolConfig
		if err := rows.Scan(
			&c.AgentSlug, &c.ToolSlug, &c.ParamKey, &c.ParamValue, &c.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability agent tool config: %w", err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_tool_config not accessible (post-iter), capability agent tool configs unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability agent tool configs: %w", err)
	}
	return configs, nil
}

// LoadToolConfigsForAgent returns all capability agent tool config rows for the
// given agentSlug from ah_core.capability_agent_tool_config, ordered by
// sort_order. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentToolConfigLoader) LoadToolConfigsForAgent(ctx context.Context, agentSlug string) ([]CoreAgentToolConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT agent_slug, tool_slug, param_key, param_value, sort_order
		  FROM ah_core.capability_agent_tool_config
		 WHERE agent_slug = $1
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_tool_config not accessible, tool configs for agent unavailable", "err", err, "agent_slug", agentSlug)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query tool configs for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var configs []CoreAgentToolConfig
	for rows.Next() {
		var c CoreAgentToolConfig
		if err := rows.Scan(
			&c.AgentSlug, &c.ToolSlug, &c.ParamKey, &c.ParamValue, &c.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan tool config for agent %q: %w", agentSlug, err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_agent_tool_config not accessible (post-iter), tool configs for agent unavailable", "err", err, "agent_slug", agentSlug)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate tool configs for agent %q: %w", agentSlug, err)
	}
	return configs, nil
}

// ============================================================
// Seed catalog constants — migration 000101 (2026-05-11).
// ============================================================

// SeedCapabilityAgentToolConfigCount is the expected total row count after
// migration 000101. Seven overrides across three capability agents.
const SeedCapabilityAgentToolConfigCount = 7

// SeedCapabilityAgentToolConfigAgentSlugs is the canonical closed set of
// capability agent slugs that have tool config overrides seeded in migration 000101.
var SeedCapabilityAgentToolConfigAgentSlugs = []string{
	"core-researcher",
	"core-analyst",
	"core-planner",
}

// SeedResearcherToolConfigCount is the number of tool config override rows
// seeded for the core-researcher agent in migration 000101.
const SeedResearcherToolConfigCount = 3

// SeedAnalystToolConfigCount is the number of tool config override rows
// seeded for the core-analyst agent in migration 000101.
const SeedAnalystToolConfigCount = 2

// SeedPlannerToolConfigCount is the number of tool config override rows
// seeded for the core-planner agent in migration 000101.
const SeedPlannerToolConfigCount = 2

// SeedToolConfigMaxResults is the parameter key for maximum result count overrides.
const SeedToolConfigMaxResults = "max_results"

// SeedToolConfigTimeoutSeconds is the parameter key for timeout duration overrides.
const SeedToolConfigTimeoutSeconds = "timeout_seconds"

// SeedToolConfigSimilarityThreshold is the parameter key for vector similarity
// threshold overrides.
const SeedToolConfigSimilarityThreshold = "similarity_threshold"

// SeedToolConfigMaxTokens is the parameter key for maximum token budget overrides.
const SeedToolConfigMaxTokens = "max_tokens"

// SeedToolConfigMaxItems is the parameter key for maximum item count overrides.
const SeedToolConfigMaxItems = "max_items"

// SeedToolConfigIncludeCompleted is the parameter key for include-completed-items
// flag overrides.
const SeedToolConfigIncludeCompleted = "include_completed"
