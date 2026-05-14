-- PERM-009-paired: permission restore policy default templates.
-- 4 templates 1:1 with PermissionRestorePolicy enum so fresh tenants
-- pick a resume/fork permission restore stance without inventing it.

CREATE TABLE IF NOT EXISTS ah_core.permission_restore_policy_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_restore_policy           TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    safety_posture                  TEXT NOT NULL,
    surviving_durabilities          JSONB NOT NULL DEFAULT '[]'::jsonb,
    flags_first_use_confirmation    BOOLEAN NOT NULL DEFAULT FALSE,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_perm_restore_dt_policy
    ON ah_core.permission_restore_policy_default_template(target_restore_policy);
CREATE INDEX IF NOT EXISTS idx_perm_restore_dt_use_case
    ON ah_core.permission_restore_policy_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_perm_restore_dt_active
    ON ah_core.permission_restore_policy_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_perm_restore_dt_recommended
    ON ah_core.permission_restore_policy_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.permission_restore_policy_default_template
    (id, slug, name, description, target_restore_policy, target_use_case,
     safety_posture, surviving_durabilities, flags_first_use_confirmation,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('99999999-9999-9999-9999-000000000001',
     'discard-all-fresh-context',
     'Discard All — Fresh Context',
     'Most conservative stance: every previously-granted Allow/Confirm is dropped on resume/fork; only Deny grants survive. Forces the LLM to re-request each permission. Default for fresh tenants without admin opt-in.',
     'discard_all', 'fresh_resume',
     'strict',
     '[]'::jsonb, FALSE,
     'general', TRUE, TRUE, TRUE, 10),

    ('99999999-9999-9999-9999-000000000002',
     'preserve-durable-routine',
     'Preserve Durable — Routine Resume',
     'Routine resume stance: keeps Persisted and ExplicitAdmin grants; drops SessionScoped and OneShot. Suitable for tenants who want continuity for grants the admin already vetted but not for transient session-scoped escalations.',
     'preserve_durable_only', 'routine_resume',
     'balanced',
     '["persisted","explicit_admin"]'::jsonb, FALSE,
     'general', FALSE, TRUE, TRUE, 20),

    ('99999999-9999-9999-9999-000000000003',
     'preserve-explicit-compliance',
     'Preserve Explicit Only — Compliance',
     'Compliance-strict stance: only ExplicitAdmin grants survive resume/fork. Every persisted grant from a non-admin user is dropped — admins must re-confirm. Used by regulated tenants where any non-admin grant requires fresh consent.',
     'preserve_explicit_grants', 'compliance_audit',
     'conservative',
     '["explicit_admin"]'::jsonb, FALSE,
     'general', TRUE, TRUE, TRUE, 30),

    ('99999999-9999-9999-9999-000000000004',
     'strict-re-request-audit',
     'Strict Re-Request — Audit',
     'Most defensive stance: drops EVERY Allow/Confirm grant AND flags the runtime to require user confirmation on the first invocation of every tool, regardless of static rules. Used by audit-strict tenants resuming after long gaps.',
     'strict_re_request', 'audit_strict_resume',
     'strict',
     '[]'::jsonb, TRUE,
     'general', TRUE, TRUE, TRUE, 40)
ON CONFLICT (slug) DO NOTHING;
