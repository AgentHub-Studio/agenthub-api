package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityAuditLogConfigLoader loads per-agent audit log configuration
// from ah_core.capability_audit_log_config. These 9 rows (seeded by migration
// 000113) define which events capability agents emit to the audit log, how long
// those logs are retained, and at what verbosity level. Three config rows per
// agent:
//
//   - audit-researcher-events     (core-researcher, log_events,     tool_call,tool_result,web_search,doc_index)
//   - audit-researcher-retention  (core-researcher, retention_days, 90)
//   - audit-researcher-level      (core-researcher, log_level,      info)
//   - audit-analyst-events        (core-analyst,    log_events,     tool_call,tool_result,doc_read,doc_search)
//   - audit-analyst-retention     (core-analyst,    retention_days, 180)
//   - audit-analyst-level         (core-analyst,    log_level,      info)
//   - audit-planner-events        (core-planner,    log_events,     tool_call,tool_result,task_create,task_update)
//   - audit-planner-retention     (core-planner,    retention_days, 365)
//   - audit-planner-level         (core-planner,    log_level,      debug)
//
// Non-fatal when the ah_core schema or the capability_audit_log_config table is
// missing — supports fresh deployments where migration 000113 has not yet run.
type CoreCapabilityAuditLogConfigLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityAuditLogConfigLoader creates a
// CoreCapabilityAuditLogConfigLoader backed by pool.
func NewCoreCapabilityAuditLogConfigLoader(pool *pgxpool.Pool) *CoreCapabilityAuditLogConfigLoader {
	return &CoreCapabilityAuditLogConfigLoader{pool: pool}
}

// CoreCapabilityAuditLogConfig is a single per-agent audit log configuration
// entry. Captures the slug, the target agent slug, the config key, the config
// value, an optional description, and the display sort order.
type CoreCapabilityAuditLogConfig struct {
	Slug        string
	AgentSlug   string
	ConfigKey   string
	ConfigValue string
	Description string
	SortOrder   int
}

// LoadCapabilityAuditLogConfigs returns all audit log config rows from
// ah_core.capability_audit_log_config WHERE agent_slug = ANY($1), ordered by
// agent_slug, sort_order. Returns nil, nil when the table is not accessible
// (non-fatal).
func (l *CoreCapabilityAuditLogConfigLoader) LoadCapabilityAuditLogConfigs(ctx context.Context) ([]CoreCapabilityAuditLogConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, config_key, config_value, COALESCE(description, ''), sort_order
		  FROM ah_core.capability_audit_log_config
		 WHERE agent_slug = ANY($1)
		 ORDER BY agent_slug, sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityAuditLogConfigAgentSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_audit_log_config not accessible, audit log configs unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability audit log configs: %w", err)
	}
	defer rows.Close()

	var configs []CoreCapabilityAuditLogConfig
	for rows.Next() {
		var c CoreCapabilityAuditLogConfig
		if err := rows.Scan(
			&c.Slug, &c.AgentSlug, &c.ConfigKey, &c.ConfigValue,
			&c.Description, &c.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability audit log config: %w", err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_audit_log_config not accessible (post-iter), audit log configs unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability audit log configs: %w", err)
	}
	return configs, nil
}

// LoadAuditLogConfigsForAgent returns the audit log config rows from
// ah_core.capability_audit_log_config WHERE agent_slug = $1, ordered by
// sort_order. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAuditLogConfigLoader) LoadAuditLogConfigsForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityAuditLogConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, config_key, config_value, COALESCE(description, ''), sort_order
		  FROM ah_core.capability_audit_log_config
		 WHERE agent_slug = $1
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_audit_log_config not accessible, audit log configs unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query audit log configs for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var configs []CoreCapabilityAuditLogConfig
	for rows.Next() {
		var c CoreCapabilityAuditLogConfig
		if err := rows.Scan(
			&c.Slug, &c.AgentSlug, &c.ConfigKey, &c.ConfigValue,
			&c.Description, &c.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan audit log config for agent %q: %w", agentSlug, err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_audit_log_config not accessible (post-iter), audit log configs unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate audit log configs for agent %q: %w", agentSlug, err)
	}
	return configs, nil
}

// ============================================================
// Seed catalog constants — migration 000113 (2026-05-11).
// ============================================================

// SeedCapabilityAuditLogConfigCount is the expected total row count after
// migration 000113. Nine audit log config rows — three per capability agent
// (researcher, analyst, planner), each agent having config rows for
// log_events, retention_days, and log_level.
const SeedCapabilityAuditLogConfigCount = 9

// SeedCapabilityAuditLogConfigAgentSlugs is the canonical list of capability
// agent slugs that have seeded audit log config rows in migration 000113.
var SeedCapabilityAuditLogConfigAgentSlugs = []string{
	"core-researcher",
	"core-analyst",
	"core-planner",
}

// SeedResearcherAuditLogConfigCount is the number of audit log config rows
// seeded for core-researcher (3: log_events + retention_days + log_level).
const SeedResearcherAuditLogConfigCount = 3

// SeedAnalystAuditLogConfigCount is the number of audit log config rows
// seeded for core-analyst (3: log_events + retention_days + log_level).
const SeedAnalystAuditLogConfigCount = 3

// SeedPlannerAuditLogConfigCount is the number of audit log config rows
// seeded for core-planner (3: log_events + retention_days + log_level).
const SeedPlannerAuditLogConfigCount = 3

// Config key constants — migration 000113.

// SeedAuditLogConfigKeyEvents is the config_key value for the comma-separated
// list of events that should be emitted to the audit log for an agent.
const SeedAuditLogConfigKeyEvents = "log_events"

// SeedAuditLogConfigKeyRetention is the config_key value for the number of
// days audit log entries should be retained for an agent.
const SeedAuditLogConfigKeyRetention = "retention_days"

// SeedAuditLogConfigKeyLevel is the config_key value for the verbosity level
// of audit log entries emitted by an agent.
const SeedAuditLogConfigKeyLevel = "log_level"

// Log level constants — migration 000113.

// SeedAuditLogLevelInfo is the log_level config_value for agents that emit
// standard informational audit log entries. Used by researcher and analyst.
const SeedAuditLogLevelInfo = "info"

// SeedAuditLogLevelDebug is the log_level config_value for agents that emit
// verbose debug-level audit log entries. Used exclusively by the planner
// because task decomposition steps are complex enough to require detailed
// logging for troubleshooting.
const SeedAuditLogLevelDebug = "debug"

// Retention value constants — migration 000113. All values are stored as TEXT
// in config_value for schema flexibility.

// SeedResearcherRetentionDays is the retention_days config_value for
// core-researcher ("90"). Research outputs are transient and superseded quickly,
// so a 90-day window is sufficient.
const SeedResearcherRetentionDays = "90"

// SeedAnalystRetentionDays is the retention_days config_value for
// core-analyst ("180"). A 180-day retention window satisfies typical compliance
// requirements (SOX, GDPR 6-month audit windows).
const SeedAnalystRetentionDays = "180"

// SeedPlannerRetentionDays is the retention_days config_value for
// core-planner ("365"). Full-year retention is required because project audit
// trails must span complete annual planning cycles.
const SeedPlannerRetentionDays = "365"

// SeedPlannerAuditLogLevel is the log_level config_value for core-planner.
// The planner is the only capability agent configured for debug-level audit
// logging — task decomposition and dependency tracking generate complex state
// that requires verbose logging to diagnose planning failures.
const SeedPlannerAuditLogLevel = SeedAuditLogLevelDebug
