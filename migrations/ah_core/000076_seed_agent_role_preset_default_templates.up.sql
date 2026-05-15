-- 000076: seed ah_core.agent_role_preset_template
-- Adapts Claude Code's 6 built-in subagent types (PDF arXiv:2604.14228v1 §8) to
-- AgentHub's web-first, multi-tenant platform. Tenants may use these presets as
-- starting points when creating agents via the web UI.
--
-- Mapping:
--   Explore       → read-researcher     (read-only investigation mode)
--   Plan          → planner             (structured plans, await approval)
--   General-purpose → general-assistant (default all-around agent)
--   Claude Code Guide → documentation-guide (onboarding/help)
--   Verification  → validator           (test/lint/quality checks)
--   Statusline-setup → NOT_APPLICABLE_WEB (terminal-specific)
CREATE TABLE IF NOT EXISTS ah_core.agent_role_preset_template (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                 TEXT NOT NULL UNIQUE,
    label                TEXT NOT NULL,
    description          TEXT NOT NULL,
    source_subagent_type TEXT NOT NULL,        -- Claude Code built-in type this maps to
    default_context_mode TEXT NOT NULL DEFAULT 'inline' CHECK (default_context_mode IN ('inline','fork')),
    default_effort_level TEXT NOT NULL DEFAULT 'medium',
    allowed_tools        JSONB NOT NULL DEFAULT '[]',
    disallowed_tools     JSONB NOT NULL DEFAULT '[]',
    permission_mode      TEXT NOT NULL DEFAULT 'default',
    recommended_for      JSONB NOT NULL DEFAULT '[]',
    sort_order           INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT agent_role_preset_template_slug_format CHECK (slug ~ '^[a-z0-9][a-z0-9-]*[a-z0-9]$')
);

INSERT INTO ah_core.agent_role_preset_template
    (slug, label, description, source_subagent_type, default_context_mode, default_effort_level,
     allowed_tools, disallowed_tools, permission_mode, recommended_for, sort_order)
VALUES
    (
        'general-assistant',
        'General Assistant',
        'Broadly capable agent for all-around tasks. Balances speed and quality. Use when no specialist is needed.',
        'general-purpose',
        'inline', 'medium',
        '[]', '[]',
        'default',
        '["customer-support","qa","content-creation","data-analysis","general-queries"]',
        10
    ),
    (
        'read-researcher',
        'Read-Only Researcher',
        'Investigation-focused agent with write tools disabled. Safe for auditing, code review, and read-only exploration of knowledge bases.',
        'explore',
        'inline', 'medium',
        '["document-search","knowledge-base-query","web-search"]',
        '["file-write","sql-mutate","http-post","http-patch","http-delete"]',
        'default',
        '["auditing","code-review","research","document-retrieval","competitive-intelligence"]',
        20
    ),
    (
        'planner',
        'Structured Planner',
        'Produces a detailed action plan and STOPS before executing. A human or orchestrator reviews the plan before triggering execution. Inspired by Claude Code Plan subagent.',
        'plan',
        'inline', 'high',
        '[]', '[]',
        'plan',
        '["multi-step-tasks","risk-sensitive-operations","onboarding-workflows","project-kickoff"]',
        30
    ),
    (
        'validator',
        'Validator',
        'Runs verification checks — test suites, data validation, lint, quality gates. Returns a structured pass/fail report. Inspired by Claude Code Verification subagent.',
        'verification',
        'fork', 'medium',
        '[]', '[]',
        'default',
        '["ci-checks","data-quality","form-validation","compliance-spot-checks","regression-detection"]',
        40
    ),
    (
        'documentation-guide',
        'Documentation Guide',
        'Onboarding and documentation assistance agent. Explains features, generates help content, and guides users through AgentHub capabilities. Inspired by Claude Code Guide subagent.',
        'claude-code-guide',
        'inline', 'low',
        '["knowledge-base-query","document-search"]', '[]',
        'default',
        '["onboarding","help-desk","feature-explanation","tutorial-generation","user-documentation"]',
        50
    )
ON CONFLICT (slug) DO NOTHING;
