-- SUB-006-paired: subagent permission inheritance mode default templates.
-- 4 templates 1:1 with PermissionInheritanceMode enum so fresh tenants
-- pick a subagent permission composition stance without inventing it.

CREATE TABLE IF NOT EXISTS ah_core.subagent_inheritance_mode_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_inheritance_mode         TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    risk_posture                    TEXT NOT NULL,
    expected_audit_signals          JSONB NOT NULL DEFAULT '[]'::jsonb,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_subagent_inh_dt_mode
    ON ah_core.subagent_inheritance_mode_default_template(target_inheritance_mode);
CREATE INDEX IF NOT EXISTS idx_subagent_inh_dt_use_case
    ON ah_core.subagent_inheritance_mode_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_subagent_inh_dt_active
    ON ah_core.subagent_inheritance_mode_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_subagent_inh_dt_recommended
    ON ah_core.subagent_inheritance_mode_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.subagent_inheritance_mode_default_template
    (id, slug, name, description, target_inheritance_mode, target_use_case,
     risk_posture, expected_audit_signals,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('77777777-7777-7777-7777-000000000001',
     'extend-parent-rights',
     'Extend Parent Rights',
     'Default for routine subagents that extend their parent with extra allows or denies. Allow/Deny/Confirm union; child mode overrides parent mode when set. Use when the subagent is a helper and should keep the parent fully in scope.',
     'inherit_all', 'helper_extension',
     'balanced', '["AddedAllows","AddedDenies"]'::jsonb,
     'general', FALSE, TRUE, TRUE, 10),

    ('77777777-7777-7777-7777-000000000002',
     'sandboxed-worker',
     'Sandboxed Worker',
     'For workers that need their own allow/confirm posture but must NEVER relax the parent deny set. Allow/Confirm come from child only; Deny is parent ∪ child. Use for code generators, formatters, and other narrow utility subagents.',
     'inherit_strict_only', 'narrow_utility',
     'conservative', '["AddedDenies"]'::jsonb,
     'general', TRUE, TRUE, TRUE, 20),

    ('77777777-7777-7777-7777-000000000003',
     'isolated-decoupled',
     'Isolated De-Coupled Worker',
     'For workers that the parent has reviewed and explicitly de-coupled. Parent rules are IGNORED — only child rules apply. Used for incident-response agents that need a different policy or for plugin-supplied workers with their own audit posture.',
     'override_replace', 'explicit_isolation',
     'strict', '["ReasonSummary"]'::jsonb,
     'general', TRUE, TRUE, TRUE, 30),

    ('77777777-7777-7777-7777-000000000004',
     'audit-strict-intersect',
     'Audit-Strict Intersect',
     'For audit-strict tenants: subagent allow MUST be at least as restrictive as parent allow. Allow = parent ∩ child; Deny = parent ∪ child. Tracks DroppedAllows so the auditor sees what the subagent gave up.',
     'merge_intersect', 'compliance_audit',
     'permissive', '["DroppedAllows","AddedDenies"]'::jsonb,
     'general', TRUE, TRUE, TRUE, 40)
ON CONFLICT (slug) DO NOTHING;
