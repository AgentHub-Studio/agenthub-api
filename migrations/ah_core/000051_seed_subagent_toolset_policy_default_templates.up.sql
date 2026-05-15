-- SUB-005-paired: subagent toolset isolation policy default templates.
-- 4 templates 1:1 with SubagentToolsetIsolationMode so fresh tenants
-- pick a subagent tool-pool isolation stance without inventing it.

CREATE TABLE IF NOT EXISTS ah_core.subagent_toolset_policy_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_isolation_mode           TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    safety_posture                  TEXT NOT NULL,
    sample_allowed_tool_names       JSONB NOT NULL DEFAULT '[]'::jsonb,
    sample_blocked_tool_names       JSONB NOT NULL DEFAULT '[]'::jsonb,
    sample_depth_threshold_for_agent INTEGER NOT NULL DEFAULT 0,
    sample_category_prefixes        JSONB NOT NULL DEFAULT '[]'::jsonb,
    sample_category_suffixes        JSONB NOT NULL DEFAULT '[]'::jsonb,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_subagent_toolset_dt_mode
    ON ah_core.subagent_toolset_policy_default_template(target_isolation_mode);
CREATE INDEX IF NOT EXISTS idx_subagent_toolset_dt_use_case
    ON ah_core.subagent_toolset_policy_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_subagent_toolset_dt_active
    ON ah_core.subagent_toolset_policy_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_subagent_toolset_dt_recommended
    ON ah_core.subagent_toolset_policy_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.subagent_toolset_policy_default_template
    (id, slug, name, description, target_isolation_mode, target_use_case,
     safety_posture, sample_allowed_tool_names, sample_blocked_tool_names,
     sample_depth_threshold_for_agent, sample_category_prefixes,
     sample_category_suffixes,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('88888888-8888-8888-8888-000000000001',
     'documentation-readonly-allowlist',
     'Documentation Subagent — Read-Only Allowlist',
     'Most restrictive stance: subagent sees only an explicit allowlist of read-only tools (Read, Grep, Glob, document_search). Used for documentation generators, summarizers, and analyzers that should never mutate.',
     'explicit_allowlist', 'documentation_generator',
     'strict',
     '["Read","Grep","Glob","document_search"]'::jsonb,
     '[]'::jsonb, 0,
     '[]'::jsonb, '[]'::jsonb,
     'general', TRUE, TRUE, TRUE, 10),

    ('88888888-8888-8888-8888-000000000002',
     'research-minus-sharp-edges',
     'Research Subagent — Minus Sharp Edges',
     'Routine research stance: subagent inherits parent pool minus a small blocklist of high-blast-radius tools (Bash, Write, execute-sql). Use when the subagent needs broad investigation but no mutation.',
     'parent_minus_blocklist', 'research_assistant',
     'balanced',
     '[]'::jsonb,
     '["Bash","Write","execute-sql"]'::jsonb, 0,
     '[]'::jsonb, '[]'::jsonb,
     'general', FALSE, TRUE, TRUE, 20),

    ('88888888-8888-8888-8888-000000000003',
     'recursion-safety-depth-filtered',
     'Recursion Safety — Depth-Filtered Agent',
     'Hides the Agent tool at depth > 1 so subagents cannot recursively spawn new subagents indefinitely. Required for any tenant that uses multi-level orchestration to prevent runaway loops.',
     'depth_filtered', 'recursion_safety',
     'conservative',
     '[]'::jsonb,
     '[]'::jsonb, 1,
     '[]'::jsonb, '[]'::jsonb,
     'general', TRUE, TRUE, TRUE, 30),

    ('88888888-8888-8888-8888-000000000004',
     'admin-surface-categorical-exclusion',
     'Admin Surface — Categorical Exclusion',
     'Blocks every tool whose name starts with "admin_" or ends with "_dangerous" from any subagent (depth > 0). Used to keep sensitive surfaces inaccessible while inheritance is otherwise broad.',
     'categorical_exclusion', 'sensitive_surface_protection',
     'conservative',
     '[]'::jsonb,
     '[]'::jsonb, 0,
     '["admin_"]'::jsonb, '["_dangerous"]'::jsonb,
     'general', TRUE, TRUE, TRUE, 40)
ON CONFLICT (slug) DO NOTHING;
