-- PERM-005-paired: permission hook default templates.
-- 6 hook pattern blueprints covering both PermissionHookPhase values
-- (before_evaluate / after_evaluate) × override outcomes so fresh
-- tenants have ready-to-implement hook patterns without inventing
-- semantics.

CREATE TABLE IF NOT EXISTS ah_core.permission_hook_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_phase                    TEXT NOT NULL,
    target_outcome                  TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    default_priority                INTEGER NOT NULL DEFAULT 100,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_perm_hook_dt_phase
    ON ah_core.permission_hook_default_template(target_phase);
CREATE INDEX IF NOT EXISTS idx_perm_hook_dt_outcome
    ON ah_core.permission_hook_default_template(target_outcome);
CREATE INDEX IF NOT EXISTS idx_perm_hook_dt_use_case
    ON ah_core.permission_hook_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_perm_hook_dt_active
    ON ah_core.permission_hook_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_perm_hook_dt_recommended
    ON ah_core.permission_hook_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.permission_hook_default_template
    (id, slug, name, description, target_phase, target_outcome,
     target_use_case, default_priority, recommended_for_tenant_kind,
     requires_admin_review, is_recommended, is_active, sort_order)
VALUES
    ('44444444-4444-4444-4444-000000000001',
     'oncall-bypass-allow',
     'On-Call User Bypass Allow',
     'Before-phase hook that short-circuits the engine when the requesting user is on the active on-call rotation. Used for emergency response: on-call engineers should never be blocked by routine deny rules during an incident.',
     'before_evaluate', 'override_allow',
     'oncall_escalation', 10, 'general', TRUE, TRUE, TRUE, 10),

    ('44444444-4444-4444-4444-000000000002',
     'business-hours-gate-deny',
     'Business-Hours-Only Deny',
     'Before-phase hook that denies mutating tools outside business hours. Reduces the blast radius of agentic mishaps when no human reviewer is available to intercept.',
     'before_evaluate', 'override_deny',
     'time_window_gate', 20, 'general', TRUE, TRUE, TRUE, 20),

    ('44444444-4444-4444-4444-000000000003',
     'pii-input-escalate-confirm',
     'PII Input Escalate to Confirm',
     'After-phase hook that escalates Allow → Confirm when the tool input matches a PII heuristic (customer_data, email, ssn, etc). Keeps routine queries fast while inserting a human checkpoint on sensitive data.',
     'after_evaluate', 'override_confirm',
     'data_sensitivity', 30, 'general', TRUE, TRUE, TRUE, 30),

    ('44444444-4444-4444-4444-000000000004',
     'audit-trace-observer-continue',
     'Audit Trace Observer',
     'After-phase hook that only OBSERVES (returns continue) — does not change the decision but records the engine outcome for GOV-001/OBS-003 audit. Recommended for every tenant that needs decision-level traceability.',
     'after_evaluate', 'continue',
     'audit_observability', 90, 'general', FALSE, TRUE, TRUE, 40),

    ('44444444-4444-4444-4444-000000000005',
     'rate-limit-cooldown-deny',
     'Rate-Limit Cooldown Deny',
     'Before-phase hook that denies a tool when the same agent has called it more than N times in the last M minutes. Defends against runaway loops and accidental DoS without rewriting deny rules.',
     'before_evaluate', 'override_deny',
     'rate_limit', 25, 'general', TRUE, TRUE, TRUE, 50),

    ('44444444-4444-4444-4444-000000000006',
     'sensitive-customer-deny',
     'Sensitive Customer Deny',
     'After-phase hook that overrides Allow → Deny when the tool input references a customer tagged as sensitive in CRM. Stricter than the PII confirm because the policy is "no automation may touch these customers at all".',
     'after_evaluate', 'override_deny',
     'customer_protection', 15, 'general', TRUE, TRUE, TRUE, 60)
ON CONFLICT (slug) DO NOTHING;
