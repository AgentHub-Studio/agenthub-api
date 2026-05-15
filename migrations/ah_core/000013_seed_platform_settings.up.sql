-- Seed platform-managed DEFAULT SETTINGS in ah_core.
-- Settings are platform-wide tunables every tenant inherits unless they
-- explicitly override (per-tenant settings live in their own schema).
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 Section 6.1 (settings is one of 10 plugin
--     manifest component types)
--   - Claude Code's user/team/org settings layering
--
-- AgentHub adapts: settings here are PLATFORM defaults (lowest precedence).
-- Tenants override via tenant settings; per-agent runtime config wins last.
--
-- Categories scoped at seed time:
--   runner       — agentic loop budget / depth / timeout tunables
--   evaluator    — OBS-008 quality evaluator thresholds
--   policy       — GOV-002 policy engine selection
--   checkpoint   — GOV-003 human-control deadline / cost trigger
--   ui           — frontend defaults (style, telemetry visibility)
--   security     — defensive defaults (audit retention, dangerous-tool deny)
--   compaction   — CTX-010 context window thresholds

CREATE TABLE IF NOT EXISTS ah_core.platform_setting (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- key is the dotted-path identifier (e.g. "runner.default_max_iterations").
    key             VARCHAR(128) NOT NULL UNIQUE,
    -- value is stored as TEXT; the runtime parses per value_type.
    value           TEXT         NOT NULL,
    -- value_type drives parsing/validation: string / number / boolean / json.
    value_type      VARCHAR(16)  NOT NULL DEFAULT 'string',
    -- category groups related settings in the UI / config dump.
    category        VARCHAR(32)  NOT NULL,
    description     TEXT         NOT NULL,
    -- is_overridable: when FALSE, tenants cannot override this setting
    -- (security baseline / global invariants).
    is_overridable  BOOLEAN      NOT NULL DEFAULT TRUE,
    -- is_active controls visibility (allows soft-disable without dropping rows).
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order      INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_platform_setting_key       ON ah_core.platform_setting (key);
CREATE INDEX IF NOT EXISTS idx_ah_core_platform_setting_category  ON ah_core.platform_setting (category);
CREATE INDEX IF NOT EXISTS idx_ah_core_platform_setting_is_active ON ah_core.platform_setting (is_active);

-- ============================
-- RUNNER (4)
-- ============================
INSERT INTO ah_core.platform_setting (key, value, value_type, category, description, is_overridable, sort_order) VALUES
('runner.default_max_iterations', '25', 'number', 'runner',
 'Default maximum iterations per agentic run before the budget cap kicks in.',
 TRUE, 10),

('runner.default_max_budget_usd', '1.00', 'number', 'runner',
 'Default maximum LLM cost (USD) per agentic run before the budget cap kicks in.',
 TRUE, 20),

('runner.default_max_depth', '3', 'number', 'runner',
 'Default maximum subagent recursion depth per run.',
 TRUE, 30),

('runner.elicitation_timeout_ms', '300000', 'number', 'runner',
 'Default ms to wait for user response in an ask_user elicitation (5 minutes).',
 TRUE, 40);

-- ============================
-- EVALUATOR (3) — OBS-008
-- ============================
INSERT INTO ah_core.platform_setting (key, value, value_type, category, description, is_overridable, sort_order) VALUES
('evaluator.default_engine', 'heuristic', 'string', 'evaluator',
 'Default RunEvaluator implementation (OBS-008): heuristic | llm-judge.',
 TRUE, 50),

('evaluator.fail_threshold', '0.4', 'number', 'evaluator',
 'OverallScore below this triggers EvalFail decision.',
 TRUE, 60),

('evaluator.warn_threshold', '0.7', 'number', 'evaluator',
 'OverallScore below this (and >= fail) triggers EvalWarn decision.',
 TRUE, 70);

-- ============================
-- POLICY (1) — GOV-002
-- ============================
INSERT INTO ah_core.platform_setting (key, value, value_type, category, description, is_overridable, sort_order) VALUES
('policy.default_engine', 'noop', 'string', 'policy',
 'Default PolicyEngine implementation (GOV-002): noop | policy-limits | static-deny.',
 TRUE, 80);

-- ============================
-- CHECKPOINT (2) — GOV-003
-- ============================
INSERT INTO ah_core.platform_setting (key, value, value_type, category, description, is_overridable, sort_order) VALUES
('checkpoint.default_deadline_seconds', '600', 'number', 'checkpoint',
 'Default deadline (seconds) for human-control checkpoints (GOV-003) — 10 minutes.',
 TRUE, 90),

('checkpoint.cost_threshold_usd', '5.00', 'number', 'checkpoint',
 'Cost above this (USD) arms a CheckpointPreCostThreshold before execution.',
 TRUE, 100);

-- ============================
-- UI (2)
-- ============================
INSERT INTO ah_core.platform_setting (key, value, value_type, category, description, is_overridable, sort_order) VALUES
('ui.default_output_style_slug', 'conversational', 'string', 'ui',
 'Default output style slug for new agents (FK to ah_core.output_style.slug).',
 TRUE, 110),

('ui.show_run_progress', 'true', 'boolean', 'ui',
 'Whether the chat UI shows the run-progress sidebar by default.',
 TRUE, 120);

-- ============================
-- SECURITY (2 — both NOT overridable)
-- ============================
INSERT INTO ah_core.platform_setting (key, value, value_type, category, description, is_overridable, sort_order) VALUES
('security.deny_dangerous_tools', 'true', 'boolean', 'security',
 'When TRUE, dangerous tools (file delete, shell, etc.) require explicit allow-rules per tenant.',
 FALSE, 130),

('security.audit_retention_days', '365', 'number', 'security',
 'Default audit log retention period in days. Cannot be lowered by tenant.',
 FALSE, 140);

-- ============================
-- COMPACTION (1) — CTX-010
-- ============================
INSERT INTO ah_core.platform_setting (key, value, value_type, category, description, is_overridable, sort_order) VALUES
('compaction.context_threshold_pct', '0.7', 'number', 'compaction',
 'Trigger context compaction when token usage reaches this fraction of model context window.',
 TRUE, 150);
