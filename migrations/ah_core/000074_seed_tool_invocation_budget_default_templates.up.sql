-- TOOL-010 paired seed: tool invocation budget templates.
-- Default budget configurations for common agent archetypes so fresh tenants
-- have sensible call-count guardrails without writing them from scratch.
-- Each template captures: total_cap, policy, and per-category caps (JSON).
-- Category caps use the same keys as ToolInvocationCategory Go enum:
--   read / mutate / external / internal / unknown
--
-- Idempotent: ON CONFLICT (slug) DO NOTHING.

CREATE TABLE IF NOT EXISTS ah_core.tool_invocation_budget_template (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug             TEXT        UNIQUE NOT NULL,
    label            TEXT        NOT NULL,
    description      TEXT        NOT NULL,
    total_cap        INTEGER     NOT NULL DEFAULT 0,
    policy           TEXT        NOT NULL CHECK (policy IN ('deny', 'warn', 'report')),
    category_caps    JSONB       NOT NULL DEFAULT '{}',
    sort_order       INTEGER     NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO ah_core.tool_invocation_budget_template
    (id, slug, label, description, total_cap, policy, category_caps, sort_order)
VALUES
    (
        'bbbb0001-0000-0000-0000-000000000001',
        'unlimited',
        'Unlimited',
        'No cap on tool calls. Suitable for development, exploration, and power-user sessions where cost control is handled externally.',
        0,
        'warn',
        '{}',
        10
    ),
    (
        'bbbb0002-0000-0000-0000-000000000002',
        'interactive-standard',
        'Interactive Standard',
        'Balanced budget for typical conversational chat sessions. Caps mutate calls to prevent runaway writes; external calls limited to control API costs.',
        200,
        'warn',
        '{"mutate": 30, "external": 50}',
        20
    ),
    (
        'bbbb0003-0000-0000-0000-000000000003',
        'batch-processing',
        'Batch Processing',
        'High-volume budget for background data-processing agents. High total cap, strict deny on unexpected mutate calls (data pipelines should be read-heavy).',
        1000,
        'deny',
        '{"mutate": 20, "external": 200}',
        30
    ),
    (
        'bbbb0004-0000-0000-0000-000000000004',
        'compliance-audit',
        'Compliance Audit',
        'Read-only budget for compliance and security audit agents. Any mutating call is blocked — deny policy enforces strict read-only access.',
        500,
        'deny',
        '{"mutate": 0, "external": 50}',
        40
    ),
    (
        'bbbb0005-0000-0000-0000-000000000005',
        'cost-controlled',
        'Cost Controlled',
        'Tight budget for cost-sensitive tenants. Low total cap with report policy; LLM sees exhaustion notice and wraps up gracefully.',
        50,
        'report',
        '{"external": 10, "mutate": 5}',
        50
    ),
    (
        'bbbb0006-0000-0000-0000-000000000006',
        'research-assistant',
        'Research Assistant',
        'Generous read and external budget for research agents. Mutate calls tightly limited — research sessions should not modify customer data.',
        300,
        'warn',
        '{"mutate": 5, "external": 150, "read": 200}',
        60
    )
ON CONFLICT (slug) DO NOTHING;
