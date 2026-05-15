-- Seed platform-managed KAIROS HEARTBEAT STRATEGY templates in ah_core.
-- Inspired by PDF arXiv:2604.14228v1 §11.6 (KAIROS proactive background agent,
-- tick-based heartbeats, SleepTool economic throttling, terminal focus awareness).
--
-- AgentHub-web adaptation: agents run server-side; "terminal focus awareness"
-- maps to HTTP session presence detection (last API call timestamp).
-- Tenants use these presets to configure how proactively their agents behave.

CREATE TABLE IF NOT EXISTS ah_core.kairos_heartbeat_strategy_template (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            VARCHAR(64)  NOT NULL UNIQUE,
    label           VARCHAR(128) NOT NULL,
    description     TEXT         NOT NULL,
    -- schedule_type classifies the trigger model.
    -- on-demand: only when user sends message.
    -- heartbeat: periodic tick during user absence.
    -- cron: fixed calendar schedule (daily/weekly).
    schedule_type   VARCHAR(32)  NOT NULL,
    -- tick_interval_seconds: period between proactive ticks (0 = on-demand only).
    tick_interval_seconds INTEGER NOT NULL DEFAULT 0,
    -- presence_window_seconds: how recent a user action must be to suppress ticking.
    presence_window_seconds INTEGER NOT NULL DEFAULT 30,
    -- max_act_ticks_before_sleep: consecutive proactive acts before forced pause.
    -- 0 = unlimited (only bounded by economic_budget_usd_per_session).
    max_act_ticks_before_sleep INTEGER NOT NULL DEFAULT 0,
    -- sleep_duration_seconds: length of mandatory pause after reaching max act ticks.
    sleep_duration_seconds INTEGER NOT NULL DEFAULT 0,
    -- economic_budget_usd_per_session: cumulative API cost cap per session.
    -- 0.0 = unlimited.
    economic_budget_usd_per_session NUMERIC(8,4) NOT NULL DEFAULT 0.0,
    -- is_proactive: true if this strategy can act without a user message.
    is_proactive    BOOLEAN NOT NULL DEFAULT FALSE,
    -- is_background: true if the agent runs outside the user's active session.
    is_background   BOOLEAN NOT NULL DEFAULT FALSE,
    -- recommended_for: JSONB array of agent categories this preset fits.
    recommended_for JSONB        NOT NULL DEFAULT '[]',
    -- source_kairos_pattern: maps to §11.6 KAIROS variant name.
    source_kairos_pattern VARCHAR(64) NOT NULL DEFAULT 'none',
    sort_order      INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_kairos_slug     ON ah_core.kairos_heartbeat_strategy_template (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_kairos_type     ON ah_core.kairos_heartbeat_strategy_template (schedule_type);
CREATE INDEX IF NOT EXISTS idx_ah_core_kairos_proactive ON ah_core.kairos_heartbeat_strategy_template (is_proactive);

-- ============================
-- 6 heartbeat strategy presets
-- ============================
INSERT INTO ah_core.kairos_heartbeat_strategy_template
    (slug, label, description, schedule_type,
     tick_interval_seconds, presence_window_seconds,
     max_act_ticks_before_sleep, sleep_duration_seconds,
     economic_budget_usd_per_session, is_proactive, is_background,
     recommended_for, source_kairos_pattern, sort_order) VALUES

('on-demand',
 'On-Demand Only',
 'Agent acts only when the user sends a message. No proactive ticking. Lowest cost, highest user control.',
 'on-demand',
 0, 30,
 0, 0,
 0.0, FALSE, FALSE,
 '["general-assistant","documentation-guide","read-researcher"]',
 'none', 10),

('heartbeat-5m',
 'Heartbeat 5-Minute',
 'Proactive tick every 5 minutes when user is absent. Minimum KAIROS economic interval — each wake-up costs a prompt-cache miss after 5 minutes of inactivity. Suitable for time-sensitive monitoring.',
 'heartbeat',
 300, 30,
 12, 900,
 0.05, TRUE, TRUE,
 '["validator","monitoring-agent","incident-responder"]',
 'kairos-standard', 20),

('heartbeat-15m',
 'Heartbeat 15-Minute',
 'Proactive tick every 15 minutes when user is absent. Balanced between responsiveness and cost. Recommended for most background agents.',
 'heartbeat',
 900, 30,
 8, 1800,
 0.02, TRUE, TRUE,
 '["general-assistant","planner","research-assistant"]',
 'kairos-relaxed', 30),

('heartbeat-hourly',
 'Heartbeat Hourly',
 'Proactive tick every hour. Low cost, suitable for agents that monitor slow-changing conditions or perform hourly summaries.',
 'heartbeat',
 3600, 30,
 4, 7200,
 0.01, TRUE, TRUE,
 '["documentation-guide","summary-agent","compliance-monitor"]',
 'kairos-slow', 40),

('daily-digest',
 'Daily Digest',
 'Agent fires once per day (cron-style). No tick-based presence detection — runs on schedule regardless of user activity. Suitable for daily reports and summaries.',
 'cron',
 86400, 0,
 1, 3600,
 0.005, TRUE, TRUE,
 '["documentation-guide","compliance-monitor","analytics-agent"]',
 'scheduled-daily', 50),

('weekly-report',
 'Weekly Report',
 'Agent fires once per week. Lowest cost scheduled strategy. Suitable for weekly summaries and governance reports.',
 'cron',
 604800, 0,
 1, 7200,
 0.002, TRUE, TRUE,
 '["documentation-guide","governance-reporter","analytics-agent"]',
 'scheduled-weekly', 60)

ON CONFLICT (slug) DO NOTHING;
