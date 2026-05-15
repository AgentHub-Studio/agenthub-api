-- Seed platform-managed DEFAULT LLM MODELS in ah_core.
-- Tenants need a usable LLM model picker out of the box — without a
-- configured model, no agent can run. Catalog covers the major
-- providers + cost tiers so tenants can pick by trade-off.
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 §4 (context window + tool support are
--     first-class agent capabilities)
--   - PDF §11.4 (cost-aware model selection)
--
-- Catalog entries describe AVAILABLE models — tenants opt-in by
-- selecting one as the agent's `model_config.model` value. ah_core
-- does NOT store API keys (those live in tenant settings).

CREATE TABLE IF NOT EXISTS ah_core.llm_model (
    id                            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- slug is the stable identifier, "<provider>/<model>" format.
    slug                          VARCHAR(128) NOT NULL UNIQUE,
    -- display_name is the UI-friendly label.
    display_name                  VARCHAR(255) NOT NULL,
    description                   TEXT         NOT NULL,
    -- provider identifies the LLM gateway (openai / anthropic /
    -- openrouter / ollama / google).
    provider                      VARCHAR(32)  NOT NULL,
    -- model_name is what the provider expects in the API call (the
    -- raw model string, possibly differs from slug).
    model_name                    VARCHAR(255) NOT NULL,
    -- context_window is the max input+output tokens the model accepts.
    context_window                INTEGER      NOT NULL,
    -- supports_tools: can the model emit tool_calls (PDF §6.1 — meta-tool dispatch)?
    supports_tools                BOOLEAN      NOT NULL DEFAULT TRUE,
    -- supports_vision: can the model accept image inputs?
    supports_vision               BOOLEAN      NOT NULL DEFAULT FALSE,
    -- supports_streaming: can the model stream tokens (PDF §4 — SSE tokens)?
    supports_streaming            BOOLEAN      NOT NULL DEFAULT TRUE,
    -- default_temperature is the recommended sampling temperature.
    default_temperature           NUMERIC(4,2) NOT NULL DEFAULT 0.7,
    -- default_max_tokens is the recommended response cap.
    default_max_tokens            INTEGER      NOT NULL DEFAULT 4096,
    -- cost per 1M tokens — useful for evaluator's cost dimension (OBS-008).
    cost_per_1m_input_tokens_usd  NUMERIC(10,4) NOT NULL DEFAULT 0,
    cost_per_1m_output_tokens_usd NUMERIC(10,4) NOT NULL DEFAULT 0,
    -- is_recommended marks the platform-suggested default for new agents
    -- in this provider's tier.
    is_recommended                BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                     BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                    INTEGER      NOT NULL DEFAULT 0,
    created_at                    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_llm_model_slug      ON ah_core.llm_model (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_llm_model_provider  ON ah_core.llm_model (provider);
CREATE INDEX IF NOT EXISTS idx_ah_core_llm_model_is_active ON ah_core.llm_model (is_active);

-- ============================
-- ANTHROPIC (2)
-- ============================
INSERT INTO ah_core.llm_model
    (slug, display_name, description, provider, model_name, context_window,
     supports_tools, supports_vision, supports_streaming,
     default_temperature, default_max_tokens,
     cost_per_1m_input_tokens_usd, cost_per_1m_output_tokens_usd,
     is_recommended, sort_order) VALUES

('anthropic/claude-haiku-4-5',
 'Claude Haiku 4.5',
 'Fast, lightweight Claude model — recommended for high-throughput agents and routine tasks.',
 'anthropic', 'claude-haiku-4-5-20251001', 200000,
 TRUE, TRUE, TRUE,
 0.7, 8192,
 0.80, 4.00,
 TRUE, 10),

('anthropic/claude-sonnet-4-6',
 'Claude Sonnet 4.6',
 'Balanced Claude model — strong reasoning at moderate cost. Good default for most agents.',
 'anthropic', 'claude-sonnet-4-6', 200000,
 TRUE, TRUE, TRUE,
 0.7, 8192,
 3.00, 15.00,
 FALSE, 20);

-- ============================
-- OPENAI (2)
-- ============================
INSERT INTO ah_core.llm_model
    (slug, display_name, description, provider, model_name, context_window,
     supports_tools, supports_vision, supports_streaming,
     default_temperature, default_max_tokens,
     cost_per_1m_input_tokens_usd, cost_per_1m_output_tokens_usd,
     is_recommended, sort_order) VALUES

('openai/gpt-4o-mini',
 'GPT-4o mini',
 'Compact OpenAI model — recommended for cost-sensitive agents.',
 'openai', 'gpt-4o-mini', 128000,
 TRUE, TRUE, TRUE,
 0.7, 16384,
 0.15, 0.60,
 TRUE, 30),

('openai/gpt-4o',
 'GPT-4o',
 'Flagship OpenAI multimodal model.',
 'openai', 'gpt-4o', 128000,
 TRUE, TRUE, TRUE,
 0.7, 16384,
 2.50, 10.00,
 FALSE, 40);

-- ============================
-- OPENROUTER (2)
-- ============================
INSERT INTO ah_core.llm_model
    (slug, display_name, description, provider, model_name, context_window,
     supports_tools, supports_vision, supports_streaming,
     default_temperature, default_max_tokens,
     cost_per_1m_input_tokens_usd, cost_per_1m_output_tokens_usd,
     is_recommended, sort_order) VALUES

('openrouter/mistralai/mistral-nemo',
 'Mistral Nemo (OpenRouter)',
 'Mid-size Mistral model via OpenRouter — recommended for BDD/test environments (validated stable).',
 'openrouter', 'mistralai/mistral-nemo', 128000,
 TRUE, FALSE, TRUE,
 0.7, 4096,
 0.15, 0.15,
 TRUE, 50),

('openrouter/meta-llama/llama-3-8b-instruct',
 'Llama 3 8B Instruct (OpenRouter)',
 'Open-weight Llama 3 8B via OpenRouter — low-cost option for non-critical agents.',
 'openrouter', 'meta-llama/llama-3-8b-instruct', 8192,
 TRUE, FALSE, TRUE,
 0.7, 2048,
 0.06, 0.06,
 FALSE, 60);

-- ============================
-- OLLAMA (1) — local, free
-- ============================
INSERT INTO ah_core.llm_model
    (slug, display_name, description, provider, model_name, context_window,
     supports_tools, supports_vision, supports_streaming,
     default_temperature, default_max_tokens,
     cost_per_1m_input_tokens_usd, cost_per_1m_output_tokens_usd,
     is_recommended, sort_order) VALUES

('ollama/llama3.2',
 'Llama 3.2 (Ollama local)',
 'Local-host Llama 3.2 via Ollama — zero cost, requires self-hosted runtime.',
 'ollama', 'llama3.2', 128000,
 TRUE, FALSE, TRUE,
 0.7, 4096,
 0.00, 0.00,
 TRUE, 70);
