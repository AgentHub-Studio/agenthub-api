-- Seed capability session config rows in ah_core
-- (migration 000111). These 9 rows encode per-agent session configuration
-- defaults that shape the initial chat session experience when a capability
-- agent is first used. Three configs per agent (max_turns, idle_timeout_seconds,
-- welcome_message):
--
-- Researcher (3 configs):
--   session-researcher-max-turns       — max_turns:              50
--   session-researcher-idle-timeout    — idle_timeout_seconds:   1800 (30 min)
--   session-researcher-welcome         — welcome_message:        research specialist intro
--
-- Analyst (3 configs):
--   session-analyst-max-turns          — max_turns:              30
--   session-analyst-idle-timeout       — idle_timeout_seconds:   3600 (60 min)
--   session-analyst-welcome            — welcome_message:        analysis specialist intro
--
-- Planner (3 configs):
--   session-planner-max-turns          — max_turns:              100 (most turns)
--   session-planner-idle-timeout       — idle_timeout_seconds:   7200 (120 min — longest)
--   session-planner-welcome            — welcome_message:        planning specialist intro
--
-- Design notes:
--   - PRIMARY KEY (slug VARCHAR(120)) — human-readable key; no UUID needed.
--   - agent_slug VARCHAR(120): the capability agent this config belongs to.
--   - config_key VARCHAR(120): configuration dimension (max_turns / idle_timeout_seconds / welcome_message).
--   - config_value TEXT: the configuration value (numeric string or free text).
--   - description TEXT: human-readable explanation of what this config controls.
--   - sort_order INTEGER: display order within each agent's config list.
--   - UNIQUE (agent_slug, config_key): prevents duplicate config keys per agent.
--   - ON CONFLICT (slug) DO NOTHING — idempotent seeds.
--   - Table is created here in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_session_config (
    slug         VARCHAR(120) PRIMARY KEY,
    agent_slug   VARCHAR(120) NOT NULL,
    config_key   VARCHAR(120) NOT NULL,
    config_value TEXT         NOT NULL,
    description  TEXT,
    sort_order   INTEGER      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (agent_slug, config_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_session_config_agent_slug
    ON ah_core.capability_session_config (agent_slug);

-- ==============================
-- 9 capability session config rows
-- ==============================
INSERT INTO ah_core.capability_session_config
    (slug, agent_slug, config_key, config_value, description, sort_order)
VALUES
    -- Researcher configs
    ('session-researcher-max-turns',
     'core-researcher', 'max_turns', '50',
     'Max conversation turns per session — researcher balances depth with brevity', 1),
    ('session-researcher-idle-timeout',
     'core-researcher', 'idle_timeout_seconds', '1800',
     '30 minutes idle timeout — research sessions are focused and relatively short', 2),
    ('session-researcher-welcome',
     'core-researcher', 'welcome_message',
     'I''m your research specialist. I can search the web, find sources, and synthesize information on any topic. What would you like to research?',
     'Shown at session start for core-researcher', 3),

    -- Analyst configs
    ('session-analyst-max-turns',
     'core-analyst', 'max_turns', '30',
     'Max conversation turns per session — analysts focus on specific documents, fewer turns needed', 1),
    ('session-analyst-idle-timeout',
     'core-analyst', 'idle_timeout_seconds', '3600',
     '60 minutes idle timeout — analysis takes longer than research, users need thinking time', 2),
    ('session-analyst-welcome',
     'core-analyst', 'welcome_message',
     'I''m your analysis specialist. I can analyze documents, extract patterns, and provide evidence-based insights. Upload a document or describe what you need analyzed.',
     'Shown at session start for core-analyst', 3),

    -- Planner configs
    ('session-planner-max-turns',
     'core-planner', 'max_turns', '100',
     'Max conversation turns per session — planners have longer conversations with many task iterations', 1),
    ('session-planner-idle-timeout',
     'core-planner', 'idle_timeout_seconds', '7200',
     '2 hours idle timeout — planning sessions can be long; users step away to think between turns', 2),
    ('session-planner-welcome',
     'core-planner', 'welcome_message',
     'I''m your planning specialist. I can break down complex goals into actionable tasks, manage task lists, and help track progress. What would you like to plan?',
     'Shown at session start for core-planner', 3)

ON CONFLICT (slug) DO NOTHING;
