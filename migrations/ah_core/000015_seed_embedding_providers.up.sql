-- Seed platform-managed DEFAULT EMBEDDING PROVIDERS in ah_core.
-- RAG indexing requires a configured embedding provider — without one,
-- knowledge bases cannot be indexed and document search returns nothing.
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 §4 (RAG pipeline retrieval)
--   - CLAUDE.md (agenthub-embedding service: E5-Large 1024dim default)
--
-- Catalog entries describe AVAILABLE providers — tenants opt-in by
-- selecting one as their KB's embedding_provider value. ah_core does
-- NOT store API keys.

CREATE TABLE IF NOT EXISTS ah_core.embedding_provider (
    id                    UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- slug is the stable identifier, "<provider>/<model>" format.
    slug                  VARCHAR(128) NOT NULL UNIQUE,
    display_name          VARCHAR(255) NOT NULL,
    description           TEXT         NOT NULL,
    -- provider identifies the gateway (openai / anthropic / local /
    -- huggingface / ollama).
    provider              VARCHAR(32)  NOT NULL,
    -- model_name is what the provider expects (raw model string).
    model_name            VARCHAR(255) NOT NULL,
    -- dimensions is the embedding vector size (must match pgvector column).
    dimensions            INTEGER      NOT NULL,
    -- max_input_tokens is the model's max input chunk size.
    max_input_tokens      INTEGER      NOT NULL,
    -- supports_batch indicates whether the provider supports batched
    -- embedding requests (cost/latency optimization).
    supports_batch        BOOLEAN      NOT NULL DEFAULT TRUE,
    -- cost_per_1m_tokens_usd: useful for cost-aware KB indexing.
    cost_per_1m_tokens_usd NUMERIC(10,4) NOT NULL DEFAULT 0,
    -- is_recommended marks the platform-suggested default for fresh KBs.
    is_recommended        BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active             BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order            INTEGER      NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_embedding_provider_slug      ON ah_core.embedding_provider (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_embedding_provider_provider  ON ah_core.embedding_provider (provider);
CREATE INDEX IF NOT EXISTS idx_ah_core_embedding_provider_is_active ON ah_core.embedding_provider (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_embedding_provider_dimensions ON ah_core.embedding_provider (dimensions);

-- ============================
-- LOCAL (1) — agenthub-embedding service (CLAUDE.md default)
-- ============================
INSERT INTO ah_core.embedding_provider
    (slug, display_name, description, provider, model_name,
     dimensions, max_input_tokens,
     supports_batch, cost_per_1m_tokens_usd,
     is_recommended, sort_order) VALUES

('local/e5-large',
 'E5-Large (local)',
 'Multilingual E5-Large embedding model via agenthub-embedding service. AgentHub default — zero external cost, runs in-cluster.',
 'local', 'intfloat/multilingual-e5-large',
 1024, 512,
 TRUE, 0.00,
 TRUE, 10);

-- ============================
-- OPENAI (2)
-- ============================
INSERT INTO ah_core.embedding_provider
    (slug, display_name, description, provider, model_name,
     dimensions, max_input_tokens,
     supports_batch, cost_per_1m_tokens_usd,
     is_recommended, sort_order) VALUES

('openai/text-embedding-3-small',
 'OpenAI text-embedding-3-small',
 'Compact OpenAI embedding — recommended for cost-sensitive RAG.',
 'openai', 'text-embedding-3-small',
 1536, 8191,
 TRUE, 0.02,
 TRUE, 20),

('openai/text-embedding-3-large',
 'OpenAI text-embedding-3-large',
 'Flagship OpenAI embedding — high-quality at moderate cost.',
 'openai', 'text-embedding-3-large',
 3072, 8191,
 TRUE, 0.13,
 FALSE, 30);

-- ============================
-- HUGGINGFACE (1) — open-weight via HuggingFace inference
-- ============================
INSERT INTO ah_core.embedding_provider
    (slug, display_name, description, provider, model_name,
     dimensions, max_input_tokens,
     supports_batch, cost_per_1m_tokens_usd,
     is_recommended, sort_order) VALUES

('huggingface/bge-small-en-v1.5',
 'BAAI BGE Small EN v1.5 (HuggingFace)',
 'Open-weight English embedding model via HuggingFace Inference. Low-latency, low-cost option.',
 'huggingface', 'BAAI/bge-small-en-v1.5',
 384, 512,
 TRUE, 0.00,
 FALSE, 40);

-- ============================
-- OLLAMA (1) — fully local
-- ============================
INSERT INTO ah_core.embedding_provider
    (slug, display_name, description, provider, model_name,
     dimensions, max_input_tokens,
     supports_batch, cost_per_1m_tokens_usd,
     is_recommended, sort_order) VALUES

('ollama/nomic-embed-text',
 'Nomic Embed Text (Ollama local)',
 'Local-host Nomic embedding via Ollama — zero cost, requires self-hosted runtime.',
 'ollama', 'nomic-embed-text',
 768, 8192,
 FALSE, 0.00,
 TRUE, 50);
