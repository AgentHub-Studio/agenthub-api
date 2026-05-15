-- Seed platform-managed CONTEXT SECTION BUDGET TEMPLATES in ah_core.
-- Templates are blueprints paired with CTX-001 ContextAssembler. Each
-- template ties a total budget + per-section caps into one ready-to-use
-- profile tenants can pick (small-context model? research-heavy work?
-- conversation-heavy chat?).

CREATE TABLE IF NOT EXISTS ah_core.context_section_budget_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- slug is dotted-path namespace (kebab-case).
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- total_budget_tokens caps the entire ContextEnvelope.
    total_budget_tokens         INTEGER      NOT NULL,
    -- Per-section caps map to ContextSectionKind (CTX-001):
    --   system / memory / rules / skill_catalog / tool_catalog /
    --   kb_summary / recent_messages / aux_prompt / scratchpad
    -- 0 means "no cap for this section".
    cap_system                  INTEGER      NOT NULL DEFAULT 0,
    cap_memory                  INTEGER      NOT NULL DEFAULT 0,
    cap_rules                   INTEGER      NOT NULL DEFAULT 0,
    cap_skill_catalog           INTEGER      NOT NULL DEFAULT 0,
    cap_tool_catalog            INTEGER      NOT NULL DEFAULT 0,
    cap_kb_summary              INTEGER      NOT NULL DEFAULT 0,
    cap_recent_messages         INTEGER      NOT NULL DEFAULT 0,
    cap_aux_prompt              INTEGER      NOT NULL DEFAULT 0,
    cap_scratchpad              INTEGER      NOT NULL DEFAULT 0,
    -- target_use_case is a short label classifying the profile.
    target_use_case             VARCHAR(48)  NOT NULL,
    -- recommended_for_model_family hints which model the budget targets.
    recommended_for_model_family VARCHAR(48) NOT NULL,
    -- is_recommended marks safe one-click defaults.
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_csbt_use_case  ON ah_core.context_section_budget_template (target_use_case);
CREATE INDEX IF NOT EXISTS idx_ah_core_csbt_active    ON ah_core.context_section_budget_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_csbt_rec       ON ah_core.context_section_budget_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 6 budget profiles. Sum of caps may exceed total budget
-- — caps are per-section limits, total enforces the global ceiling
-- (CTX-001 drops lowest-priority sections to fit total).

INSERT INTO ah_core.context_section_budget_template
    (slug, name, description, total_budget_tokens,
     cap_system, cap_memory, cap_rules, cap_skill_catalog, cap_tool_catalog,
     cap_kb_summary, cap_recent_messages, cap_aux_prompt, cap_scratchpad,
     target_use_case, recommended_for_model_family, is_recommended, sort_order)
VALUES
    ('balanced-default',
     'Balanced Default (32k)',
     'Balanced budget for general-purpose agents on mid-tier models. Mirrors CTX-001 DefaultAssemblerConfig — safe one-click for fresh tenants.',
     32000,
     0, 800, 600, 2000, 2000, 1500, 16000, 1000, 1000,
     'general', 'mid_tier', TRUE, 10),

    ('small-context-tight',
     'Small Context Tight (8k)',
     'Tight 8k budget for legacy or low-cost models (e.g. small open-source). Aggressively caps catalogs + recent_messages to fit while preserving system + memory.',
     8000,
     0, 400, 300, 800, 800, 600, 3500, 400, 400,
     'general', 'small_local', TRUE, 20),

    ('research-heavy-128k',
     'Research Heavy (128k)',
     'Research workflows reading many KB sources + long histories. Generous KB cap; deep recent-messages window for multi-turn citation grounding.',
     128000,
     0, 1500, 800, 3000, 3000, 32000, 64000, 2000, 4000,
     'research', 'large_context', TRUE, 30),

    ('conversation-heavy-64k',
     'Conversation Heavy (64k)',
     'Customer-support and chat agents — recent_messages dominates so the agent retains long conversation context. Catalogs stay tight.',
     64000,
     0, 1000, 600, 1500, 1500, 1500, 50000, 1000, 1000,
     'conversation', 'mid_tier', TRUE, 40),

    ('code-heavy-64k',
     'Code Heavy (64k)',
     'Code-review and code-generation agents — bigger tool_catalog (many code tools) + larger scratchpad for reasoning. Smaller KB since code agents pull from repo not docs.',
     64000,
     0, 800, 500, 4000, 8000, 800, 32000, 1000, 8000,
     'code', 'large_context', TRUE, 50),

    ('minimum-viable-4k',
     'Minimum Viable (4k)',
     'Last-resort budget for extreme legacy models. Only system + memory + minimal recent_messages survive — useful when you need ANY agent to run on a 4k-window model.',
     4000,
     0, 200, 200, 400, 400, 0, 1500, 200, 0,
     'general', 'tiny_legacy', FALSE, 60)
ON CONFLICT (slug) DO NOTHING;
