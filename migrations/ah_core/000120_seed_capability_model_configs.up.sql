-- Seed capability_model_config rows in ah_core (migration 000120).
-- These 9 rows encode the per-agent default LLM model configuration
-- for the three core capability agents: model ID, temperature, and
-- max_tokens. Values act as sane defaults that the orchestrator uses
-- when no tenant override is configured for a given agent.
--
-- core-researcher (3 rows — fast + factual):
--   default_model → claude-haiku-4-5-20251001  (fast + cost-effective for search-heavy workloads)
--   temperature   → 0.3                         (low: factual accuracy in research outputs)
--   max_tokens    → 4096                        (moderate: summaries + citations, not essays)
--
-- core-analyst (3 rows — deep reasoning):
--   default_model → claude-sonnet-4-6           (balanced capability for complex analysis)
--   temperature   → 0.2                         (very low: deterministic, reproducible analysis)
--   max_tokens    → 8192                        (longer: detailed reports with structured sections)
--
-- core-planner (3 rows — reasoning + creativity):
--   default_model → claude-sonnet-4-6           (capable reasoning for complex multi-step planning)
--   temperature   → 0.5                         (moderate: some creativity for plan alternatives)
--   max_tokens    → 8192                        (longer: detailed plans with steps, trade-offs, timelines)
--
-- Design notes:
--   - PRIMARY KEY (id BIGSERIAL) — auto-generated numeric PK; agent model
--     config identity is (agent_slug, config_key) enforced by UNIQUE constraint.
--   - agent_slug TEXT NOT NULL: the capability agent that owns this config entry
--     (e.g. "core-researcher", "core-analyst", "core-planner").
--   - config_key TEXT NOT NULL: machine-readable key ("default_model", "temperature", "max_tokens").
--   - config_value TEXT NOT NULL: the config value encoded as text (model IDs and numeric strings).
--   - description TEXT NOT NULL DEFAULT '': human-readable explanation of the config choice.
--   - ON CONFLICT (agent_slug, config_key) DO NOTHING — idempotent seeds.
--   - Table is created in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.capability_model_config (
    id          BIGSERIAL   NOT NULL,
    agent_slug  TEXT        NOT NULL,
    config_key  TEXT        NOT NULL,
    config_value TEXT       NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_capability_model_config PRIMARY KEY (id),
    CONSTRAINT uq_capability_model_config_agent_key UNIQUE (agent_slug, config_key)
);

CREATE INDEX IF NOT EXISTS idx_ah_core_capability_model_config_agent_slug
    ON ah_core.capability_model_config (agent_slug);

-- ==============================
-- 9 capability model config rows
-- ==============================
INSERT INTO ah_core.capability_model_config
    (agent_slug, config_key, config_value, description)
VALUES
    -- core-researcher (fast, factual, search-heavy)
    ('core-researcher',
     'default_model',
     'claude-haiku-4-5-20251001',
     'Default model: fast and cost-effective for search-heavy workloads'),
    ('core-researcher',
     'temperature',
     '0.3',
     'Low temperature for factual accuracy in research outputs'),
    ('core-researcher',
     'max_tokens',
     '4096',
     'Moderate output length: summaries and citations, not long essays'),

    -- core-analyst (deep reasoning, high quality)
    ('core-analyst',
     'default_model',
     'claude-sonnet-4-6',
     'Default model: balanced capability for complex analysis tasks'),
    ('core-analyst',
     'temperature',
     '0.2',
     'Very low temperature for deterministic, reproducible analysis'),
    ('core-analyst',
     'max_tokens',
     '8192',
     'Longer outputs for detailed analysis reports with structured sections'),

    -- core-planner (reasoning + creativity for trade-off evaluation)
    ('core-planner',
     'default_model',
     'claude-sonnet-4-6',
     'Default model: capable reasoning for complex multi-step planning'),
    ('core-planner',
     'temperature',
     '0.5',
     'Moderate temperature: some creativity for exploring plan alternatives'),
    ('core-planner',
     'max_tokens',
     '8192',
     'Longer outputs for detailed plans with steps, trade-offs, and timelines')

ON CONFLICT (agent_slug, config_key) DO NOTHING;
