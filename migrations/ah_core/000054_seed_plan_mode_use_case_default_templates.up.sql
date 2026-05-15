-- PERM-003a-paired: plan mode use-case default templates.
-- 5 templates spanning the human-in-the-loop ladder so fresh tenants
-- pick proven plan-mode workflows without inventing semantics.

CREATE TABLE IF NOT EXISTS ah_core.plan_mode_use_case_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    safety_posture                  TEXT NOT NULL,
    extra_read_only_tools           JSONB NOT NULL DEFAULT '[]'::jsonb,
    requires_rationale              BOOLEAN NOT NULL DEFAULT FALSE,
    max_planned_actions             INTEGER NOT NULL DEFAULT 0,
    auto_approve_threshold          INTEGER NOT NULL DEFAULT 0,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_plan_mode_dt_use_case
    ON ah_core.plan_mode_use_case_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_plan_mode_dt_posture
    ON ah_core.plan_mode_use_case_default_template(safety_posture);
CREATE INDEX IF NOT EXISTS idx_plan_mode_dt_active
    ON ah_core.plan_mode_use_case_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_plan_mode_dt_recommended
    ON ah_core.plan_mode_use_case_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.plan_mode_use_case_default_template
    (id, slug, name, description, target_use_case, safety_posture,
     extra_read_only_tools, requires_rationale, max_planned_actions,
     auto_approve_threshold,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('bbbbbbbb-bbbb-bbbb-bbbb-000000000001',
     'dry-run-preview',
     'Dry-Run Preview',
     'Light plan-mode for low-stakes work: every mutating action is recorded but no rationale required and small auto-approve threshold. Used when the LLM is exploring options and the user just wants to see what it WOULD do before committing.',
     'dry_run', 'permissive',
     '["document_search","memory_recall"]'::jsonb, FALSE, 0,
     3,
     'general', FALSE, TRUE, TRUE, 10),

    ('bbbbbbbb-bbbb-bbbb-bbbb-000000000002',
     'scoped-change-review',
     'Scoped Change Review',
     'Routine plan-mode for tenants who run the agent on bounded tasks. Rationale required so the operator can audit intent; reasonable max-actions cap prevents runaway plans.',
     'scoped_change', 'balanced',
     '["document_search"]'::jsonb, TRUE, 20,
     0,
     'general', FALSE, TRUE, TRUE, 20),

    ('bbbbbbbb-bbbb-bbbb-bbbb-000000000003',
     'destructive-audit',
     'Destructive Action Audit',
     'For high-risk operations: rationale required on every record, smaller max-actions cap, admin review on the template itself. Forces the agent to justify each destructive step before user approval.',
     'destructive_audit', 'strict',
     '[]'::jsonb, TRUE, 10,
     0,
     'general', TRUE, TRUE, TRUE, 30),

    ('bbbbbbbb-bbbb-bbbb-bbbb-000000000004',
     'multi-step-refactor',
     'Multi-Step Refactor',
     'For long engineering plans (refactors, migrations within scope): larger max-actions cap (50), rationale required, broader read-only set (allows running tests + searches during planning).',
     'multi_step_refactor', 'balanced',
     '["document_search","memory_recall","Read","Grep","Glob"]'::jsonb, TRUE, 50,
     0,
     'general', TRUE, TRUE, TRUE, 40),

    ('bbbbbbbb-bbbb-bbbb-bbbb-000000000005',
     'cross-tenant-migration',
     'Cross-Tenant Migration',
     'Most defensive plan-mode: zero auto-approve, rationale required, low max-actions to keep batches small, admin review. Used for one-off migrations or data moves that span tenant boundaries.',
     'cross_tenant_migration', 'strict',
     '[]'::jsonb, TRUE, 5,
     0,
     'general', TRUE, TRUE, TRUE, 50)
ON CONFLICT (slug) DO NOTHING;
