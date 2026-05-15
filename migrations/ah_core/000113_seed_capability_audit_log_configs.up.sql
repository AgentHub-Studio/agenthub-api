-- Seed capability audit log config rows in ah_core
-- (migration 000113). These 9 rows encode per-agent audit log configuration that
-- defines which events capability agents emit to the audit log, how long those
-- logs are retained, and at what verbosity level. Three config rows per agent
-- (researcher, analyst, planner):
--
-- Researcher (3 rows):
--   audit-researcher-events    — log_events    tool_call,tool_result,web_search,doc_index
--   audit-researcher-retention — retention_days 90
--   audit-researcher-level     — log_level      info
--
-- Analyst (3 rows):
--   audit-analyst-events       — log_events    tool_call,tool_result,doc_read,doc_search
--   audit-analyst-retention    — retention_days 180 (longer for compliance needs)
--   audit-analyst-level        — log_level      info
--
-- Planner (3 rows):
--   audit-planner-events       — log_events    tool_call,tool_result,task_create,task_update
--   audit-planner-retention    — retention_days 365 (full year for project audit trails)
--   audit-planner-level        — log_level      debug (planner logs more detail for task tracking)
--
-- Design notes:
--   - PRIMARY KEY (slug VARCHAR(120)) — human-readable key; no UUID needed.
--   - agent_slug VARCHAR(120): the capability agent this config applies to.
--   - config_key VARCHAR(120): machine-readable config key (log_events, retention_days, log_level).
--   - config_value TEXT: config value (event list, numeric string, or log level string).
--   - description TEXT: human-readable explanation of the config row.
--   - sort_order INTEGER: display order within each agent's config set.
--   - UNIQUE (agent_slug, config_key): prevents duplicate config keys per agent.
--   - ON CONFLICT (slug) DO NOTHING — idempotent seeds.
--   - Table is created here in ah_core (schema created by migration 000001).
--   - Planner has the longest retention (365d) because project audit trails must
--     span full annual planning cycles.
--   - Analyst has medium retention (180d) to satisfy typical compliance windows
--     (SOX, GDPR 6-month audit requirement).
--   - Researcher has the shortest retention (90d) — research outputs are
--     transient and are superseded quickly.
--   - Planner is the only agent with debug log level — task decomposition steps
--     are complex enough to require detailed logging for troubleshooting.

CREATE TABLE IF NOT EXISTS ah_core.capability_audit_log_config (
    slug         VARCHAR(120) PRIMARY KEY,
    agent_slug   VARCHAR(120) NOT NULL,
    config_key   VARCHAR(120) NOT NULL,
    config_value TEXT         NOT NULL,
    description  TEXT,
    sort_order   INTEGER      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (agent_slug, config_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_audit_log_config_agent_slug
    ON ah_core.capability_audit_log_config (agent_slug);

-- ==============================
-- 9 capability audit log config rows
-- ==============================
INSERT INTO ah_core.capability_audit_log_config
    (slug, agent_slug, config_key, config_value, description, sort_order)
VALUES
    -- Researcher audit log config
    ('audit-researcher-events',
     'core-researcher', 'log_events',
     'tool_call,tool_result,web_search,doc_index',
     'Events emitted to the audit log by the researcher agent', 1),
    ('audit-researcher-retention',
     'core-researcher', 'retention_days',
     '90',
     'Audit log retention in days for the researcher agent', 2),
    ('audit-researcher-level',
     'core-researcher', 'log_level',
     'info',
     'Audit log verbosity level for the researcher agent', 3),

    -- Analyst audit log config
    ('audit-analyst-events',
     'core-analyst', 'log_events',
     'tool_call,tool_result,doc_read,doc_search',
     'Events emitted to the audit log by the analyst agent', 1),
    ('audit-analyst-retention',
     'core-analyst', 'retention_days',
     '180',
     'Audit log retention in days for the analyst agent — longer for compliance needs', 2),
    ('audit-analyst-level',
     'core-analyst', 'log_level',
     'info',
     'Audit log verbosity level for the analyst agent', 3),

    -- Planner audit log config
    ('audit-planner-events',
     'core-planner', 'log_events',
     'tool_call,tool_result,task_create,task_update',
     'Events emitted to the audit log by the planner agent', 1),
    ('audit-planner-retention',
     'core-planner', 'retention_days',
     '365',
     'Audit log retention in days for the planner agent — full year for project audit trails', 2),
    ('audit-planner-level',
     'core-planner', 'log_level',
     'debug',
     'Audit log verbosity level for the planner agent — debug for detailed task tracking', 3)

ON CONFLICT (slug) DO NOTHING;
