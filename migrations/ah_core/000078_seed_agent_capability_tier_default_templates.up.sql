-- Seed platform-managed AGENT CAPABILITY TIER templates in ah_core.
-- Inspired by PDF arXiv:2604.14228v1 §13 (Managed Agents design: independently
-- replaceable session / harness / sandbox interfaces) + §5 (permission modes).
--
-- AgentHub-web adaptation: capability tiers bundle permission mode, tool access,
-- context budget, and governance level into pre-configured presets. A fresh
-- tenant gets these tiers and can assign them to agents without custom setup.

CREATE TABLE IF NOT EXISTS ah_core.agent_capability_tier_template (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            VARCHAR(64)  NOT NULL UNIQUE,
    label           VARCHAR(128) NOT NULL,
    description     TEXT         NOT NULL,
    -- permission_mode maps to §5 permission modes (default/plan/auto/bypassPermissions).
    permission_mode VARCHAR(32)  NOT NULL DEFAULT 'default',
    -- tool_access_level: read-only, standard, full, none.
    tool_access_level VARCHAR(32) NOT NULL DEFAULT 'standard',
    -- context_window_fraction: max fraction of context window this tier may use (0.0–1.0).
    context_window_fraction NUMERIC(4,3) NOT NULL DEFAULT 1.000,
    -- max_tool_calls_per_turn: hard cap on tool calls per LLM turn. 0 = unlimited.
    max_tool_calls_per_turn INTEGER NOT NULL DEFAULT 0,
    -- allows_background_run: true if agents at this tier can run without active user session.
    allows_background_run BOOLEAN NOT NULL DEFAULT FALSE,
    -- requires_human_checkpoint: true if tier mandates a human review checkpoint per session.
    requires_human_checkpoint BOOLEAN NOT NULL DEFAULT FALSE,
    -- governance_level: none / standard / strict.
    governance_level VARCHAR(16) NOT NULL DEFAULT 'standard',
    -- design_value_profile: slug of the dominant DesignValue (§13 Table 4).
    -- human_authority / safety / reliability / capability / adaptability
    design_value_profile VARCHAR(32) NOT NULL DEFAULT 'reliability',
    -- recommended_for: JSONB array of agent role slugs this tier fits.
    recommended_for JSONB NOT NULL DEFAULT '[]',
    sort_order      INTEGER     NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_cap_tier_slug       ON ah_core.agent_capability_tier_template (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_cap_tier_perm       ON ah_core.agent_capability_tier_template (permission_mode);
CREATE INDEX IF NOT EXISTS idx_ah_core_cap_tier_governance ON ah_core.agent_capability_tier_template (governance_level);

-- ============================
-- 5 capability tier presets
-- ============================
INSERT INTO ah_core.agent_capability_tier_template
    (slug, label, description, permission_mode, tool_access_level,
     context_window_fraction, max_tool_calls_per_turn,
     allows_background_run, requires_human_checkpoint,
     governance_level, design_value_profile,
     recommended_for, sort_order) VALUES

('read-only',
 'Read-Only Tier',
 'Agent can only read data. No write tools, no background execution, no human checkpoints needed. Safest default for research and documentation agents.',
 'default', 'read-only',
 0.500, 10,
 FALSE, FALSE,
 'standard', 'safety',
 '["read-researcher","documentation-guide"]', 10),

('standard',
 'Standard Tier',
 'Balanced capability with full tool access, standard permission mode, and reliability governance. Default for most agents. Suitable for general-purpose assistants and planners.',
 'default', 'standard',
 1.000, 0,
 FALSE, FALSE,
 'standard', 'reliability',
 '["general-assistant","planner","validator"]', 20),

('background',
 'Background Tier',
 'Allows background execution without active user session. Used by proactive agents (KAIROS heartbeat pattern). Requires stricter governance to compensate for reduced human oversight.',
 'default', 'standard',
 1.000, 20,
 TRUE, FALSE,
 'strict', 'capability',
 '["monitoring-agent","kairos-agent","batch-agent"]', 30),

('governed',
 'Governed Tier',
 'Strict governance with mandatory human checkpoint per session and audit-trail enforcement. Required for regulated environments (GDPR, HIPAA, SOX). Aligned with human_authority design value.',
 'default', 'full',
 1.000, 50,
 FALSE, TRUE,
 'strict', 'human_authority',
 '["compliance-agent","audit-agent","financial-agent"]', 40),

('autonomous',
 'Autonomous Tier',
 'Maximum capability: bypassPermissions mode, unlimited tool calls, background execution. Reserved for fully trusted automation pipelines with pre-approved scopes. Use with caution.',
 'bypassPermissions', 'full',
 1.000, 0,
 TRUE, FALSE,
 'none', 'capability',
 '["pipeline-agent","batch-processor","integration-agent"]', 50)

ON CONFLICT (slug) DO NOTHING;
