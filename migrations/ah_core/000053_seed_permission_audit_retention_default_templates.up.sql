-- PERM-010-paired: permission audit retention + redaction default templates.
-- 5 templates spanning the compliance ladder so fresh tenants pick
-- audit lifecycle policy without inventing TTLs and redaction flags.

CREATE TABLE IF NOT EXISTS ah_core.permission_audit_retention_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    compliance_posture              TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    allow_ttl_days                  INTEGER NOT NULL DEFAULT 0,
    deny_ttl_days                   INTEGER NOT NULL DEFAULT 0,
    confirm_approved_ttl_days       INTEGER NOT NULL DEFAULT 0,
    confirm_denied_ttl_days         INTEGER NOT NULL DEFAULT 0,
    confirm_escalated_ttl_days      INTEGER NOT NULL DEFAULT 0,
    redact_input_snippet            BOOLEAN NOT NULL DEFAULT FALSE,
    redact_matched_rule             BOOLEAN NOT NULL DEFAULT FALSE,
    redact_run_id                   BOOLEAN NOT NULL DEFAULT FALSE,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_perm_audit_ret_dt_posture
    ON ah_core.permission_audit_retention_default_template(compliance_posture);
CREATE INDEX IF NOT EXISTS idx_perm_audit_ret_dt_use_case
    ON ah_core.permission_audit_retention_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_perm_audit_ret_dt_active
    ON ah_core.permission_audit_retention_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_perm_audit_ret_dt_recommended
    ON ah_core.permission_audit_retention_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.permission_audit_retention_default_template
    (id, slug, name, description, compliance_posture, target_use_case,
     allow_ttl_days, deny_ttl_days, confirm_approved_ttl_days,
     confirm_denied_ttl_days, confirm_escalated_ttl_days,
     redact_input_snippet, redact_matched_rule, redact_run_id,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-000000000001',
     'minimal-30day',
     'Minimal — 30 Day Retention',
     'Lightest retention: every decision tier kept 30 days, no redaction. Used by tenants with no compliance requirements who only want a recent audit trail for debugging.',
     'minimal', 'debug_only',
     30, 30, 30, 30, 30,
     FALSE, FALSE, FALSE,
     'general', FALSE, TRUE, TRUE, 10),

    ('aaaaaaaa-aaaa-aaaa-aaaa-000000000002',
     'balanced-90d-allow-365d-deny',
     'Balanced — 90d Allow / 365d Deny',
     'Recommended default: allows kept 90 days, denies kept 1 year, confirms kept 180 days. No redaction. Good for most tenants without specific regulatory mandates.',
     'balanced', 'standard_audit',
     90, 365, 180, 180, 180,
     FALSE, FALSE, FALSE,
     'general', FALSE, TRUE, TRUE, 20),

    ('aaaaaaaa-aaaa-aaaa-aaaa-000000000003',
     'regulated-7y-deny-2y-confirm',
     'Regulated — 7y Deny / 2y Confirm',
     'For regulated tenants (financial, healthcare): denies kept 7 years (regulatory floor), confirm-denied kept 7 years (user refusal record), confirm-approved kept 2 years. Input snippets redacted on export.',
     'regulated', 'compliance_audit',
     365, 2555, 730, 2555, 730,
     TRUE, FALSE, FALSE,
     'general', TRUE, TRUE, TRUE, 30),

    ('aaaaaaaa-aaaa-aaaa-aaaa-000000000004',
     'strict-pii-redacted-export',
     'Strict PII — Redacted Export',
     'For tenants handling PII (GDPR / LGPD): all retention windows match regulated tier BUT both input_snippet and matched_rule are redacted on export. Run IDs preserved for internal correlation but stripped at the boundary.',
     'pii_strict', 'gdpr_lgpd',
     365, 2555, 730, 2555, 730,
     TRUE, TRUE, TRUE,
     'general', TRUE, TRUE, TRUE, 40),

    ('aaaaaaaa-aaaa-aaaa-aaaa-000000000005',
     'forensic-hold-never-expires',
     'Forensic Hold — Never Expires',
     'For tenants under active investigation or legal hold: every decision tier set to 0 (never expires). No automatic deletion until the hold is explicitly released. Requires admin review (legal/compliance approval to enable).',
     'forensic_hold', 'legal_hold',
     0, 0, 0, 0, 0,
     FALSE, FALSE, FALSE,
     'general', TRUE, TRUE, TRUE, 50)
ON CONFLICT (slug) DO NOTHING;
