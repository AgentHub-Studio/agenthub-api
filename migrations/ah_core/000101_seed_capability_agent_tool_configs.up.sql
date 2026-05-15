-- Seed capability agent tool config overrides in ah_core
-- (migration 000101). These 7 rows provide per-agent tool parameter overrides
-- for the three capability agents introduced in migration 000091:
--
--   core-researcher  (3 overrides: max_results, timeout_seconds, similarity_threshold)
--   core-analyst     (2 overrides: max_results, max_tokens)
--   core-planner     (2 overrides: max_items, include_completed)
--
-- Each row encodes a single parameter override that the runtime layer reads
-- when building tool invocation contexts for that agent. Defaults come from
-- the tool definition itself; these rows narrow them to the capability role.
--
-- Design notes:
--   - PRIMARY KEY (id UUID) — opaque row identifier.
--   - UNIQUE (agent_slug, tool_slug, param_key) — one value per (agent, tool, param).
--   - ON CONFLICT (agent_slug, tool_slug, param_key) DO NOTHING — idempotent.
--   - sort_order — controls processing order within a given agent's tool list.
--   - param_value is VARCHAR(500); caller is responsible for type coercion.
--
-- The capability_agent_tool_config table is created here in ah_core
-- (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_agent_tool_config (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_slug  VARCHAR(120) NOT NULL,
    tool_slug   VARCHAR(120) NOT NULL,
    param_key   VARCHAR(120) NOT NULL,
    param_value VARCHAR(500) NOT NULL,
    sort_order  INTEGER      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (agent_slug, tool_slug, param_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_catc_agent
    ON ah_core.capability_agent_tool_config (agent_slug);

CREATE INDEX IF NOT EXISTS idx_ah_core_catc_agent_sort
    ON ah_core.capability_agent_tool_config (agent_slug, sort_order);

-- ============================
-- 7 capability agent tool config rows
-- ============================
INSERT INTO ah_core.capability_agent_tool_config
    (agent_slug, tool_slug, param_key, param_value, sort_order)
VALUES
    -- core-researcher (3 rows, sort_order 1–3)
    (
        'core-researcher',
        'core-web-search',
        'max_results',
        '10',
        1
    ),
    (
        'core-researcher',
        'core-web-fetch',
        'timeout_seconds',
        '30',
        2
    ),
    (
        'core-researcher',
        'core-doc-search',
        'similarity_threshold',
        '0.75',
        3
    ),
    -- core-analyst (2 rows, sort_order 1–2)
    (
        'core-analyst',
        'core-doc-search',
        'max_results',
        '20',
        1
    ),
    (
        'core-analyst',
        'core-doc-read',
        'max_tokens',
        '8000',
        2
    ),
    -- core-planner (2 rows, sort_order 1–2)
    (
        'core-planner',
        'core-todo-create',
        'max_items',
        '50',
        1
    ),
    (
        'core-planner',
        'core-todo-list',
        'include_completed',
        'true',
        2
    )

ON CONFLICT (agent_slug, tool_slug, param_key) DO NOTHING;
