-- Seed platform-managed DEFAULT KNOWLEDGE BASE TEMPLATES in ah_core.
-- Templates are pre-configured KB blueprints (chunk strategy + embedding
-- provider + retention policy) so tenants can create useful KBs in one
-- click instead of configuring every parameter.
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 §4 (RAG pipeline) + §6.1 (knowledge bases as
--     core component)
--   - CLAUDE.md (RAG pipeline: extract → chunk → embed → index → search)
--
-- Catalog entries describe AVAILABLE templates — tenants opt-in by
-- creating a KB from a template (template values become the KB's defaults).

CREATE TABLE IF NOT EXISTS ah_core.knowledge_base_template (
    id                       UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- slug is the stable identifier.
    slug                     VARCHAR(64)  NOT NULL UNIQUE,
    display_name             VARCHAR(255) NOT NULL,
    description              TEXT         NOT NULL,
    -- template_kind classifies the use case.
    -- One of: faq / documentation / internal_wiki / chat_history /
    --         api_reference / regulatory / customer_support.
    template_kind            VARCHAR(32)  NOT NULL,
    -- default_embedding_provider_slug references ah_core.embedding_provider.slug.
    -- Cross-table FK (app-level — DB does not enforce so seed order
    -- doesn't matter).
    default_embedding_provider_slug VARCHAR(128) NOT NULL,
    -- chunk_strategy: how documents are split (fixed_size / semantic / sentence / paragraph).
    chunk_strategy           VARCHAR(32)  NOT NULL,
    -- chunk_size_tokens is the target chunk size when strategy=fixed_size.
    chunk_size_tokens        INTEGER      NOT NULL DEFAULT 512,
    -- chunk_overlap_tokens for sliding-window contexts.
    chunk_overlap_tokens     INTEGER      NOT NULL DEFAULT 50,
    -- recommended_top_k is the default retrieval count.
    recommended_top_k        INTEGER      NOT NULL DEFAULT 5,
    -- supported_doc_types is the comma-separated whitelist of file types.
    supported_doc_types      VARCHAR(255) NOT NULL DEFAULT 'pdf,docx,txt,md,html',
    -- requires_admin_review: tenant admin must approve KB creation
    -- (used for regulatory templates).
    requires_admin_review    BOOLEAN      NOT NULL DEFAULT FALSE,
    -- is_recommended marks platform-suggested templates for fresh tenants.
    is_recommended           BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order               INTEGER      NOT NULL DEFAULT 0,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_kb_template_slug      ON ah_core.knowledge_base_template (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_kb_template_kind      ON ah_core.knowledge_base_template (template_kind);
CREATE INDEX IF NOT EXISTS idx_ah_core_kb_template_is_active ON ah_core.knowledge_base_template (is_active);

-- ============================
-- 7 templates covering common KB use cases
-- ============================
INSERT INTO ah_core.knowledge_base_template
    (slug, display_name, description, template_kind,
     default_embedding_provider_slug,
     chunk_strategy, chunk_size_tokens, chunk_overlap_tokens,
     recommended_top_k, supported_doc_types,
     requires_admin_review, is_recommended, sort_order) VALUES

('faq-template',
 'FAQ',
 'Question-answer pairs. Small chunks for direct answer retrieval. Recommended for first-time tenants.',
 'faq',
 'local/e5-large',
 'sentence', 256, 0,
 3, 'md,txt,html',
 FALSE, TRUE, 10),

('documentation-template',
 'Product Documentation',
 'Markdown / HTML technical documentation. Semantic chunking preserves logical sections.',
 'documentation',
 'local/e5-large',
 'semantic', 768, 100,
 5, 'md,html,txt',
 FALSE, TRUE, 20),

('internal-wiki-template',
 'Internal Wiki',
 'Mixed-format internal wiki content. Larger chunks for narrative context.',
 'internal_wiki',
 'local/e5-large',
 'paragraph', 1024, 128,
 5, 'md,html,docx,txt',
 FALSE, FALSE, 30),

('chat-history-template',
 'Chat History Search',
 'Past conversation messages indexed for "did we discuss this before?" retrieval. Small chunks per message.',
 'chat_history',
 'local/e5-large',
 'fixed_size', 256, 32,
 10, 'json,txt',
 FALSE, FALSE, 40),

('api-reference-template',
 'API Reference',
 'OpenAPI/Swagger specs and code examples. Larger chunks preserve endpoint signatures.',
 'api_reference',
 'openai/text-embedding-3-small',
 'semantic', 1024, 128,
 5, 'json,yaml,md',
 FALSE, TRUE, 50),

('regulatory-template',
 'Regulatory Documents',
 'Compliance / legal / regulatory text. Large chunks preserve clause context. ADMIN APPROVAL required.',
 'regulatory',
 'openai/text-embedding-3-large',
 'paragraph', 1536, 256,
 8, 'pdf,docx',
 TRUE, FALSE, 60),

('customer-support-template',
 'Customer Support',
 'Support ticket history + KB articles. Mixed chunk sizes optimized for "find similar past issues" retrieval.',
 'customer_support',
 'local/e5-large',
 'semantic', 512, 64,
 7, 'md,txt,html,json',
 FALSE, TRUE, 70);
