-- Seed platform-managed CONTEXT COLLAPSER STRATEGY DEFAULT TEMPLATES in ah_core.
-- Templates are blueprints paired with CTX-012 ContextCollapser. Each
-- row encodes a (collapse_strategy + consumer_kind + use_case) preset
-- so fresh tenants pick a read-time projection profile without inventing
-- their own strategy mix.

CREATE TABLE IF NOT EXISTS ah_core.context_collapser_strategy_default_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- collapse_strategy MATCHES CTX-012 CollapseStrategy enum byte-for-byte.
    collapse_strategy           VARCHAR(32)  NOT NULL,
    -- consumer_kind labels the intended downstream consumer.
    consumer_kind               VARCHAR(48)  NOT NULL,
    -- preserve_errors_always reflects the CTX-012 invariant that errors
    -- always survive — declared explicitly for admin clarity.
    preserve_errors_always      BOOLEAN      NOT NULL DEFAULT TRUE,
    -- preserve_compact_summary_always reflects PDF §9.2 resume contract.
    preserve_compact_summary_always BOOLEAN  NOT NULL DEFAULT TRUE,
    target_use_case             VARCHAR(48)  NOT NULL,
    recommended_for_tenant_kind VARCHAR(48)  NOT NULL,
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_ccsd_strategy ON ah_core.context_collapser_strategy_default_template (collapse_strategy);
CREATE INDEX IF NOT EXISTS idx_ah_core_ccsd_consumer ON ah_core.context_collapser_strategy_default_template (consumer_kind);
CREATE INDEX IF NOT EXISTS idx_ah_core_ccsd_active   ON ah_core.context_collapser_strategy_default_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_ccsd_rec      ON ah_core.context_collapser_strategy_default_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 6 strategy presets covering common consumer kinds.

INSERT INTO ah_core.context_collapser_strategy_default_template
    (slug, name, description, collapse_strategy, consumer_kind,
     preserve_errors_always, preserve_compact_summary_always,
     target_use_case, recommended_for_tenant_kind,
     requires_admin_review, is_recommended, sort_order)
VALUES
    ('llm-renderer-balanced',
     'LLM Renderer (Balanced)',
     'Default projection for the LLM prompt renderer — drops redundant tool_use messages but keeps tool_result + text. Production-tier balance of context fidelity vs token cost.',
     'balanced', 'llm_renderer',
     TRUE, TRUE,
     'agent_runtime', 'general', FALSE, TRUE, 10),

    ('replay-debug-verbose',
     'Replay Debug (Verbose)',
     'Debug projection that preserves EVERY message verbatim — minimal strategy. Use when investigating a specific run/incident; never use in production hot path.',
     'minimal', 'debugger',
     TRUE, TRUE,
     'incident_replay', 'general', FALSE, TRUE, 20),

    ('cost-dashboard-aggressive',
     'Cost Dashboard (Aggressive)',
     'Aggressive collapse for cost dashboards — keeps only assistant text + errors + summaries. Drops tool calls so dashboard shows business outcomes only.',
     'aggressive', 'cost_dashboard',
     TRUE, TRUE,
     'analytics', 'general', FALSE, TRUE, 30),

    ('audit-export-minimal',
     'Audit Export (Minimal)',
     'Regulator-facing audit export uses minimal strategy (preserves everything) so compliance auditor sees full original transcript. Aligned with FUTURE-005 export contract. Requires admin review (regulator binding).',
     'minimal', 'audit_exporter',
     TRUE, TRUE,
     'compliance_export', 'regulated', TRUE, TRUE, 40),

    ('ui-summary-aggressive',
     'UI Summary (Aggressive)',
     'Aggressive projection for end-user UI summary view — only assistant text + errors. Hides tool internals from end-user; readability over fidelity.',
     'aggressive', 'ui_summary',
     TRUE, TRUE,
     'user_facing_ui', 'general', FALSE, TRUE, 50),

    ('dev-debug-verbose-no-recommendations',
     'Dev Debug Verbose (opt-in)',
     'Verbose minimal collapse for dev tenants debugging custom agents. Useful in dev environments but produces large prompts in production. NOT recommended.',
     'minimal', 'debugger',
     TRUE, TRUE,
     'agent_runtime', 'dev_local', FALSE, FALSE, 60)
ON CONFLICT (slug) DO NOTHING;
