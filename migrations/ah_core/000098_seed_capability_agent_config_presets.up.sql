-- Seed capability-specific agent config presets in ah_core
-- (migration 000098). These 3 presets define LLM configuration profiles for
-- the three capability agents introduced in migration 000091:
--
--   core-researcher  — capability-research-config  (temperature 0.3, 8192 tokens)
--   core-analyst     — capability-analysis-config  (temperature 0.1, 16384 tokens)
--   core-planner     — capability-planning-config  (temperature 0.5, 4096 tokens)
--
-- Each preset captures the recommended model configuration for a capability agent,
-- including the model provider, model identifier, temperature, and max token budget.
-- Temperature and max_tokens are tuned per agent purpose:
--
--   Research  : moderate temperature for broad exploration, large context for
--               processing multiple sources.
--   Analysis  : lowest temperature for consistent, reproducible outputs when
--               analyzing documents and synthesizing structured findings.
--   Planning  : higher temperature for creative task decomposition while keeping
--               context budget lean (plans should be concise).
--
-- All 3:
--   - use provider=anthropic, model=claude-sonnet-4-6 (as of 2026-05-11)
--   - are is_recommended = TRUE (surfaces in the UI model-config picker)
--   - have sort_order ≥ 100 (capability-layer range, above any future platform rows)
--
-- The agent_config_preset table does not exist before this migration;
-- it is created here in ah_core (schema created by migration 000001).

CREATE TABLE IF NOT EXISTS ah_core.agent_config_preset (
    id             UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    slug           VARCHAR(64)   NOT NULL UNIQUE,
    name           VARCHAR(255)  NOT NULL,
    description    TEXT          NOT NULL,
    -- model_provider is the LLM provider key, e.g. "anthropic", "openai".
    model_provider VARCHAR(64)   NOT NULL,
    -- model_id is the provider-specific model identifier.
    model_id       VARCHAR(128)  NOT NULL,
    -- temperature controls randomness. Range: 0.0 (deterministic) to 1.0 (creative).
    temperature    NUMERIC(4, 2) NOT NULL CHECK (temperature >= 0.0 AND temperature <= 1.0),
    -- max_tokens is the maximum number of tokens the model may generate per call.
    max_tokens     INTEGER       NOT NULL CHECK (max_tokens > 0),
    is_recommended BOOLEAN       NOT NULL DEFAULT FALSE,
    sort_order     INTEGER       NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_agent_config_preset_slug      ON ah_core.agent_config_preset (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_agent_config_preset_provider  ON ah_core.agent_config_preset (model_provider);
CREATE INDEX IF NOT EXISTS idx_ah_core_agent_config_preset_recommend ON ah_core.agent_config_preset (is_recommended);

-- ============================
-- 3 capability agent config presets (one per capability agent)
-- ============================
INSERT INTO ah_core.agent_config_preset
    (slug, name, description, model_provider, model_id,
     temperature, max_tokens, is_recommended, sort_order)
VALUES

-- 1. Research Agent Config — moderate temperature, large context window.
('capability-research-config',
 'Research Agent Config',
 'LLM configuration preset for the Web Researcher capability agent. Uses a moderate temperature (0.3) for broad, exploratory research synthesis while maintaining enough determinism for source attribution. Large max_tokens budget (8192) accommodates multi-source summaries.',
 'anthropic',
 'claude-sonnet-4-6',
 0.3,
 8192,
 TRUE,
 100),

-- 2. Analysis Agent Config — lowest temperature, maximum context window.
('capability-analysis-config',
 'Analysis Agent Config',
 'LLM configuration preset for the Document Analyst capability agent. Uses the lowest temperature (0.1) for highly consistent, reproducible analysis outputs with structured findings. Maximum max_tokens budget (16384) supports deep document analysis across large corpora.',
 'anthropic',
 'claude-sonnet-4-6',
 0.1,
 16384,
 TRUE,
 101),

-- 3. Planning Agent Config — higher temperature, lean context window.
('capability-planning-config',
 'Planning Agent Config',
 'LLM configuration preset for the Task Planner capability agent. Uses a higher temperature (0.5) to encourage creative task decomposition and flexible planning strategies. Lean max_tokens budget (4096) keeps plans concise and actionable.',
 'anthropic',
 'claude-sonnet-4-6',
 0.5,
 4096,
 TRUE,
 102)

ON CONFLICT (slug) DO NOTHING;
