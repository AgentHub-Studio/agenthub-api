-- CONTEXT-001 paired seed: model capability templates.
-- Each row captures the context budget of a Claude model so the frontend can
-- display model selection with realistic limits and the backend can validate
-- context_window_pct frontmatter values against real numbers.
--
-- Idempotent: ON CONFLICT (model_id) DO NOTHING.

CREATE TABLE IF NOT EXISTS ah_core.model_capability_template (
    model_id             TEXT        PRIMARY KEY,
    family               TEXT        NOT NULL,
    label                TEXT        NOT NULL,
    description          TEXT        NOT NULL,
    context_window       INTEGER     NOT NULL,
    max_output_tokens    INTEGER     NOT NULL,
    recommended_for      JSONB       NOT NULL DEFAULT '[]',
    sort_order           INTEGER     NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO ah_core.model_capability_template
    (model_id, family, label, description, context_window, max_output_tokens, recommended_for, sort_order)
VALUES
    (
        'claude-haiku-4-5-20251001',
        'haiku',
        'Claude Haiku 4.5',
        'Fastest and most compact model. Best for low-latency tasks, simple Q&A, and high-throughput workloads where response speed matters more than reasoning depth.',
        200000,
        8192,
        '["fast-responder"]',
        10
    ),
    (
        'claude-sonnet-4-6',
        'sonnet',
        'Claude Sonnet 4.6',
        'Balanced model for most production workloads. Excellent reasoning, code generation, and document analysis with a good speed/quality trade-off.',
        200000,
        8192,
        '["careful-analyst", "creative-writer", "security-auditor", "data-extractor", "research-assistant"]',
        20
    ),
    (
        'claude-opus-4-7',
        'opus',
        'Claude Opus 4.7',
        'Most capable model for complex multi-step reasoning, deep analysis, and tasks that require maximum intelligence. Higher latency and cost.',
        200000,
        32000,
        '["security-auditor", "research-assistant"]',
        30
    )
ON CONFLICT (model_id) DO NOTHING;
