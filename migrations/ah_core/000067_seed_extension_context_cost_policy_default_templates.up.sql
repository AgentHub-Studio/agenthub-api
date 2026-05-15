-- EXT-010-paired: extension context-cost policy default templates.
-- 5 starter cost policies, one per ExtensionContextCostCategory band,
-- so fresh extension authors calibrate against proven exemplars.
--
-- DB-level CHECK constraints encode EXT-010 invariants:
--   - all token fields >= 0
--   - declared category matches per_turn_tokens band (anti-drift)

CREATE TABLE IF NOT EXISTS ah_core.extension_context_cost_policy_default_template (
    id                          UUID PRIMARY KEY,
    slug                        TEXT NOT NULL UNIQUE,
    category                    TEXT NOT NULL UNIQUE, -- matches EXT-010 enum byte-for-byte
    extension_kind              TEXT NOT NULL,
    static_header_tokens        INTEGER NOT NULL,
    per_turn_tokens             INTEGER NOT NULL,
    per_tool_call_tokens        INTEGER NOT NULL,
    max_budget_per_session      INTEGER NOT NULL,
    description                 TEXT NOT NULL,
    example_use_case            TEXT NOT NULL,
    is_recommended              BOOLEAN NOT NULL DEFAULT TRUE,
    is_active                   BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT chk_ext_cost_header_nonneg     CHECK (static_header_tokens >= 0),
    CONSTRAINT chk_ext_cost_per_turn_nonneg   CHECK (per_turn_tokens >= 0),
    CONSTRAINT chk_ext_cost_per_call_nonneg   CHECK (per_tool_call_tokens >= 0),
    CONSTRAINT chk_ext_cost_max_budget_nonneg CHECK (max_budget_per_session >= 0),
    CONSTRAINT chk_ext_cost_band_matches      CHECK (
        (category = 'micro'  AND per_turn_tokens <= 100) OR
        (category = 'small'  AND per_turn_tokens >  100 AND per_turn_tokens <=  500) OR
        (category = 'medium' AND per_turn_tokens >  500 AND per_turn_tokens <= 2000) OR
        (category = 'large'  AND per_turn_tokens > 2000 AND per_turn_tokens <= 8000) OR
        (category = 'heavy'  AND per_turn_tokens > 8000)
    )
);

CREATE INDEX IF NOT EXISTS idx_ext_cost_dt_category
    ON ah_core.extension_context_cost_policy_default_template(category);
CREATE INDEX IF NOT EXISTS idx_ext_cost_dt_active
    ON ah_core.extension_context_cost_policy_default_template(is_active);

INSERT INTO ah_core.extension_context_cost_policy_default_template
    (id, slug, category, extension_kind,
     static_header_tokens, per_turn_tokens, per_tool_call_tokens,
     max_budget_per_session, description, example_use_case,
     is_recommended, is_active, sort_order)
VALUES
    ('00000009-0000-0000-0000-000000000001',
     'micro-readonly-lookup',
     'micro',
     'readonly_lookup',
     80, 40, 20, 5000,
     'Minimum-footprint extension. Static header carries a tiny tool definition; per-turn cost is just the tool stub.',
     'Single-purpose read-only lookup (e.g., status check, single-doc fetcher, currency converter).',
     TRUE, TRUE, 10),

    ('00000009-0000-0000-0000-000000000002',
     'small-curated-search',
     'small',
     'curated_search',
     250, 200, 60, 15000,
     'Curated search extension with 1-2 tools. Header includes brief usage hints; per-turn adds tool stubs + small system fragment.',
     'Specialized search tool (e.g., internal docs search, code symbol resolver).',
     TRUE, TRUE, 20),

    ('00000009-0000-0000-0000-000000000003',
     'medium-rag-bundle',
     'medium',
     'rag_bundle',
     800, 1200, 150, 50000,
     'RAG-style extension that adds knowledge-base context + 3-4 tools. Per-turn cost dominated by retrieved chunks.',
     'Knowledge base RAG with citation tools (e.g., docs assistant, support knowledge base).',
     TRUE, TRUE, 30),

    ('00000009-0000-0000-0000-000000000004',
     'large-coding-suite',
     'large',
     'coding_suite',
     1500, 4000, 250, 150000,
     'Coding extension with many tools (read/write/search/diff/test/lint). Per-turn cost reflects rich tool definitions and recent file context.',
     'Code-editing suite with file-level awareness (e.g., refactor copilot, test author).',
     TRUE, TRUE, 40),

    ('00000009-0000-0000-0000-000000000005',
     'heavy-multimodal-vision',
     'heavy',
     'multimodal_vision',
     3000, 12000, 500, 400000,
     'Heavy multimodal extension that injects image/document descriptors and many vision tools. Per-turn cost dominated by encoded media.',
     'Visual analysis extension (e.g., screenshot debugger, design review, OCR pipelines).',
     TRUE, TRUE, 50)
ON CONFLICT (slug) DO NOTHING;
