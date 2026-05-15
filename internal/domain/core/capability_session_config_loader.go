package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilitySessionConfigLoader loads per-agent session configuration
// defaults from ah_core.capability_session_config. These 9 rows (seeded by
// migration 000111) encode the session experience defaults for each capability
// agent — controlling max_turns, idle_timeout_seconds, and welcome_message:
//
//   - session-researcher-max-turns       (core-researcher, max_turns:            50)
//   - session-researcher-idle-timeout    (core-researcher, idle_timeout_seconds: 1800)
//   - session-researcher-welcome         (core-researcher, welcome_message:      researcher intro)
//   - session-analyst-max-turns          (core-analyst,    max_turns:            30)
//   - session-analyst-idle-timeout       (core-analyst,    idle_timeout_seconds: 3600)
//   - session-analyst-welcome            (core-analyst,    welcome_message:      analyst intro)
//   - session-planner-max-turns          (core-planner,    max_turns:            100)
//   - session-planner-idle-timeout       (core-planner,    idle_timeout_seconds: 7200)
//   - session-planner-welcome            (core-planner,    welcome_message:      planner intro)
//
// Non-fatal when the ah_core schema or the capability_session_config table is
// missing — supports fresh deployments where migration 000111 has not yet run.
type CoreCapabilitySessionConfigLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilitySessionConfigLoader creates a
// CoreCapabilitySessionConfigLoader backed by pool.
func NewCoreCapabilitySessionConfigLoader(pool *pgxpool.Pool) *CoreCapabilitySessionConfigLoader {
	return &CoreCapabilitySessionConfigLoader{pool: pool}
}

// CoreCapabilitySessionConfig is a single per-agent session configuration entry.
// Captures the slug, the target agent slug, the config key, the config value,
// an optional description, and the display sort order.
type CoreCapabilitySessionConfig struct {
	Slug        string
	AgentSlug   string
	ConfigKey   string
	ConfigValue string
	Description string
	SortOrder   int
}

// LoadCapabilitySessionConfigs returns all session config rows from
// ah_core.capability_session_config WHERE agent_slug = ANY($1), ordered by
// agent_slug, sort_order. Returns nil, nil when the table is not accessible
// (non-fatal).
func (l *CoreCapabilitySessionConfigLoader) LoadCapabilitySessionConfigs(ctx context.Context) ([]CoreCapabilitySessionConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, config_key, config_value,
		       COALESCE(description, ''), sort_order
		  FROM ah_core.capability_session_config
		 WHERE agent_slug = ANY($1)
		 ORDER BY agent_slug, sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilitySessionConfigAgentSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_session_config not accessible, session configs unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability session configs: %w", err)
	}
	defer rows.Close()

	var configs []CoreCapabilitySessionConfig
	for rows.Next() {
		var c CoreCapabilitySessionConfig
		if err := rows.Scan(
			&c.Slug, &c.AgentSlug, &c.ConfigKey, &c.ConfigValue,
			&c.Description, &c.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability session config: %w", err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_session_config not accessible (post-iter), session configs unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability session configs: %w", err)
	}
	return configs, nil
}

// LoadSessionConfigsForAgent returns the session config rows from
// ah_core.capability_session_config WHERE agent_slug = $1, ordered by
// sort_order. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilitySessionConfigLoader) LoadSessionConfigsForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilitySessionConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, config_key, config_value,
		       COALESCE(description, ''), sort_order
		  FROM ah_core.capability_session_config
		 WHERE agent_slug = $1
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_session_config not accessible, session configs unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query session configs for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var configs []CoreCapabilitySessionConfig
	for rows.Next() {
		var c CoreCapabilitySessionConfig
		if err := rows.Scan(
			&c.Slug, &c.AgentSlug, &c.ConfigKey, &c.ConfigValue,
			&c.Description, &c.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan session config for agent %q: %w", agentSlug, err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_session_config not accessible (post-iter), session configs unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate session configs for agent %q: %w", agentSlug, err)
	}
	return configs, nil
}

// ============================================================
// Seed catalog constants — migration 000111 (2026-05-11).
// ============================================================

// SeedCapabilitySessionConfigCount is the expected total row count after
// migration 000111. Nine session config rows — three per capability agent
// (researcher, analyst, planner), each agent having max_turns,
// idle_timeout_seconds, and welcome_message configs.
const SeedCapabilitySessionConfigCount = 9

// SeedCapabilitySessionConfigAgentSlugs is the canonical list of capability
// agent slugs that have seeded session configs in migration 000111.
var SeedCapabilitySessionConfigAgentSlugs = []string{
	"core-researcher",
	"core-analyst",
	"core-planner",
}

// SeedResearcherSessionConfigCount is the number of session configs seeded for
// core-researcher (3: max_turns + idle_timeout_seconds + welcome_message).
const SeedResearcherSessionConfigCount = 3

// SeedAnalystSessionConfigCount is the number of session configs seeded for
// core-analyst (3: max_turns + idle_timeout_seconds + welcome_message).
const SeedAnalystSessionConfigCount = 3

// SeedPlannerSessionConfigCount is the number of session configs seeded for
// core-planner (3: max_turns + idle_timeout_seconds + welcome_message).
const SeedPlannerSessionConfigCount = 3

// Config key constants — migration 000111.

// SeedSessionConfigMaxTurns is the config_key value for the maximum
// conversation turns per session configuration.
const SeedSessionConfigMaxTurns = "max_turns"

// SeedSessionConfigIdleTimeout is the config_key value for the idle session
// timeout in seconds configuration.
const SeedSessionConfigIdleTimeout = "idle_timeout_seconds"

// SeedSessionConfigWelcomeMessage is the config_key value for the message
// displayed at the start of a new session.
const SeedSessionConfigWelcomeMessage = "welcome_message"

// Notable config values — migration 000111.

// SeedResearcherMaxTurns is the max_turns value seeded for core-researcher (50).
const SeedResearcherMaxTurns = "50"

// SeedAnalystMaxTurns is the max_turns value seeded for core-analyst (30).
// Analysts focus on specific documents and need fewer turns than researchers or planners.
const SeedAnalystMaxTurns = "30"

// SeedPlannerMaxTurns is the max_turns value seeded for core-planner (100).
// Planners have the most turns because planning conversations iterate extensively.
const SeedPlannerMaxTurns = "100"

// SeedResearcherIdleTimeout is the idle_timeout_seconds value for core-researcher
// (1800 — 30 minutes). Research sessions are focused and relatively short.
const SeedResearcherIdleTimeout = "1800"

// SeedAnalystIdleTimeout is the idle_timeout_seconds value for core-analyst
// (3600 — 60 minutes). Analysis takes longer; users need thinking time between turns.
const SeedAnalystIdleTimeout = "3600"

// SeedPlannerIdleTimeout is the idle_timeout_seconds value for core-planner
// (7200 — 120 minutes). Planning sessions can be long; users step away to think.
// This is the longest timeout among the three capability agents.
const SeedPlannerIdleTimeout = "7200"
