-- Seed platform-managed TOOL RESULT BUDGET DEFAULT TEMPLATES in ah_core.
-- Templates are blueprints paired with CTX-008 ToolResultBudgetEnforcer.
-- Each row encodes a profile (per-call/per-turn/per-run/force-summarize)
-- so fresh tenants pick a budget without inventing thresholds.

CREATE TABLE IF NOT EXISTS ah_core.tool_result_budget_default_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- Mirrors CTX-008 ToolResultBudgetConfig fields. 0 = unlimited.
    per_call_max_tokens         INTEGER      NOT NULL,
    per_turn_max_tokens         INTEGER      NOT NULL,
    per_run_max_tokens          INTEGER      NOT NULL,
    force_summarize_over_tokens INTEGER      NOT NULL,
    target_use_case             VARCHAR(48)  NOT NULL,
    recommended_for_model_family VARCHAR(48) NOT NULL,
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_trbd_use_case ON ah_core.tool_result_budget_default_template (target_use_case);
CREATE INDEX IF NOT EXISTS idx_ah_core_trbd_active   ON ah_core.tool_result_budget_default_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_trbd_rec      ON ah_core.tool_result_budget_default_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 6 budget profiles. Use cases: general / research /
-- code / cost_strict / dev / zero_trust. Model families differ for
-- different context windows.

INSERT INTO ah_core.tool_result_budget_default_template
    (slug, name, description, per_call_max_tokens, per_turn_max_tokens,
     per_run_max_tokens, force_summarize_over_tokens,
     target_use_case, recommended_for_model_family,
     requires_admin_review, is_recommended, sort_order)
VALUES
    ('balanced-default',
     'Balanced Default',
     'Mirrors CTX-008 DefaultToolResultBudgetConfig — safe one-click for fresh tenants. Per-call 4k / per-turn 12k / per-run 50k / force-summarize over 8k.',
     4000, 12000, 50000, 8000,
     'general', 'mid_tier', FALSE, TRUE, 10),

    ('small-context-tight',
     'Small Context Tight',
     'Tight budget for legacy/small-context models (8k window). Per-call 1k / per-turn 3k / per-run 6k / force-summarize over 2k. Tools dominate context unless aggressively capped.',
     1000, 3000, 6000, 2000,
     'general', 'small_local', FALSE, TRUE, 20),

    ('research-heavy',
     'Research Heavy (large context)',
     'Generous budget for research workflows reading many tool outputs (KB queries, web fetches). Per-call 16k / per-turn 64k / per-run 200k. Suits 128k+ context models.',
     16000, 64000, 200000, 32000,
     'research', 'large_context', FALSE, TRUE, 30),

    ('code-heavy',
     'Code Heavy',
     'Code-generation agents pull verbose tool outputs (test runs, build logs, diff outputs). Per-call 8k / per-turn 32k / per-run 100k. Suits mid-tier+ models.',
     8000, 32000, 100000, 16000,
     'code', 'large_context', FALSE, TRUE, 40),

    ('cost-strict',
     'Cost Strict (minimize tokens)',
     'Cost-conscious tenants: aggressive per-call cap forces summarization early; per-turn / per-run keep tight. Trades context fidelity for token budget. REQUIRES ADMIN REVIEW (degrades agent visibility).',
     2000, 6000, 20000, 1500,
     'general', 'mid_tier', TRUE, TRUE, 50),

    ('dev-debug',
     'Dev Debug (unlimited)',
     'Development/debug profile: no caps. Use ONLY in dev — production tenants will exceed model context windows on a single huge tool result. NOT recommended for production.',
     0, 0, 0, 0,
     'general', 'dev_local', FALSE, FALSE, 60)
ON CONFLICT (slug) DO NOTHING;
