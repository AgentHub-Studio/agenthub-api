-- Seed capability agent memory configs in ah_core
-- (migration 000102). These 9 rows provide per-agent memory settings
-- for the three capability agents introduced in migration 000091,
-- adapted from §9.1 session persistence channels (session_transcript /
-- global_prompt_history / subagent_sidechain):
--
--   core-researcher  (3 rows: max_context_tokens, summary_strategy, persistence_scope)
--   core-analyst     (3 rows: max_context_tokens, summary_strategy, persistence_scope)
--   core-planner     (3 rows: max_context_tokens, summary_strategy, persistence_scope)
--
-- Each row encodes a single memory configuration setting that the runtime
-- layer reads when building context windows and summarisation policies for
-- that agent. Defaults are intentionally role-differentiated:
--
--   core-researcher: 100k context, progressive summarisation, session scope
--   core-analyst:    150k context, snapshot summarisation, global scope
--   core-planner:    50k context,  append-only log, session scope
--
-- Design notes:
--   - PRIMARY KEY (id UUID) — opaque row identifier.
--   - UNIQUE (agent_slug, config_key) — one value per (agent, config key).
--   - ON CONFLICT (agent_slug, config_key) DO NOTHING — idempotent.
--   - sort_order — controls processing order within a given agent's config list.
--   - config_value is VARCHAR(500); caller is responsible for type coercion.
--
-- The capability_agent_memory_config table is created here in ah_core
-- (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_agent_memory_config (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_slug   VARCHAR(120) NOT NULL,
    config_key   VARCHAR(120) NOT NULL,
    config_value VARCHAR(500) NOT NULL,
    description  TEXT,
    sort_order   INTEGER      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (agent_slug, config_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_camc_agent
    ON ah_core.capability_agent_memory_config (agent_slug);

CREATE INDEX IF NOT EXISTS idx_ah_core_camc_agent_sort
    ON ah_core.capability_agent_memory_config (agent_slug, sort_order);

-- ============================
-- 9 capability agent memory config rows
-- ============================
INSERT INTO ah_core.capability_agent_memory_config
    (agent_slug, config_key, config_value, description, sort_order)
VALUES
    -- core-researcher (3 rows, sort_order 1–3)
    (
        'core-researcher',
        'max_context_tokens',
        '100000',
        'Max tokens in active context window',
        1
    ),
    (
        'core-researcher',
        'summary_strategy',
        'progressive',
        'Summarize progressively as context grows',
        2
    ),
    (
        'core-researcher',
        'persistence_scope',
        'session',
        'Persist findings within session',
        3
    ),
    -- core-analyst (3 rows, sort_order 1–3)
    (
        'core-analyst',
        'max_context_tokens',
        '150000',
        'Larger window for document analysis',
        1
    ),
    (
        'core-analyst',
        'summary_strategy',
        'snapshot',
        'Snapshot summaries at key checkpoints',
        2
    ),
    (
        'core-analyst',
        'persistence_scope',
        'global',
        'Persist analysis globally across sessions',
        3
    ),
    -- core-planner (3 rows, sort_order 1–3)
    (
        'core-planner',
        'max_context_tokens',
        '50000',
        'Smaller window, task list is primary memory',
        1
    ),
    (
        'core-planner',
        'summary_strategy',
        'append_only',
        'Append-only task log, never rewrite',
        2
    ),
    (
        'core-planner',
        'persistence_scope',
        'session',
        'Task context scoped to session',
        3
    )

ON CONFLICT (agent_slug, config_key) DO NOTHING;
