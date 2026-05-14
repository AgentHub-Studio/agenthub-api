-- Seed platform-managed COMPLIANCE EVIDENCE KIT templates in ah_core.
-- Kits are pre-configured packages that collect audit/quality/decision
-- evidence required by specific regulations (GDPR, HIPAA, SOX, PCI-DSS,
-- SOC2, ISO27001). Pairs with policy_engine_templates (categoria 14)
-- + GOV-001 audit + GOV-005 governance reports.
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 §11 (regulator-facing export)
--   - Evidence collection patterns from compliance frameworks

CREATE TABLE IF NOT EXISTS ah_core.compliance_evidence_kit (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            VARCHAR(64)  NOT NULL UNIQUE,
    display_name    VARCHAR(255) NOT NULL,
    description     TEXT         NOT NULL,
    -- compliance_profile classifies the regulation.
    -- One of: gdpr/hipaa/sox/pci_dss/soc2/iso27001/generic_audit/generic_security.
    compliance_profile VARCHAR(32) NOT NULL,
    -- collection_period_days: how far back the kit collects evidence.
    collection_period_days INTEGER NOT NULL DEFAULT 90,
    -- evidence components included in the kit:
    includes_audit_trail        BOOLEAN NOT NULL DEFAULT TRUE,
    includes_quality_reports    BOOLEAN NOT NULL DEFAULT FALSE,
    includes_governance_decisions BOOLEAN NOT NULL DEFAULT TRUE,
    includes_user_consent_logs  BOOLEAN NOT NULL DEFAULT FALSE,
    includes_data_access_logs   BOOLEAN NOT NULL DEFAULT FALSE,
    -- export_formats is comma-separated (csv/json/pdf/xlsx).
    export_formats VARCHAR(128) NOT NULL DEFAULT 'csv,json',
    -- retention_days for the generated kit (must be >= GDPR floor 365).
    retention_days INTEGER NOT NULL DEFAULT 365,
    -- requires_admin_signoff: kit generation triggers admin approval flow.
    requires_admin_signoff BOOLEAN NOT NULL DEFAULT FALSE,
    -- target_webhook_template_slug: optional auto-deliver to webhook
    -- (app-level FK to ah_core.webhook_endpoint_template).
    target_webhook_template_slug VARCHAR(64),
    is_recommended BOOLEAN NOT NULL DEFAULT FALSE,
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order     INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_compliance_kit_slug    ON ah_core.compliance_evidence_kit (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_compliance_kit_profile ON ah_core.compliance_evidence_kit (compliance_profile);
CREATE INDEX IF NOT EXISTS idx_ah_core_compliance_kit_active  ON ah_core.compliance_evidence_kit (is_active);

-- ============================
-- 8 evidence kits covering major regulations
-- ============================
INSERT INTO ah_core.compliance_evidence_kit
    (slug, display_name, description, compliance_profile,
     collection_period_days,
     includes_audit_trail, includes_quality_reports,
     includes_governance_decisions, includes_user_consent_logs,
     includes_data_access_logs,
     export_formats, retention_days, requires_admin_signoff,
     target_webhook_template_slug, is_recommended, sort_order) VALUES

('gdpr-quarterly-audit',
 'GDPR Quarterly Audit Kit',
 'Quarterly evidence pack: audit trail + governance decisions + consent logs + data access logs. PDF export for regulator submission.',
 'gdpr', 90,
 TRUE, FALSE, TRUE, TRUE, TRUE,
 'csv,json,pdf', 2555,
 TRUE, 'generic-https-json', FALSE, 10),

('hipaa-monthly-phi-access',
 'HIPAA Monthly PHI Access Kit',
 'Monthly PHI access audit: data access logs + governance decisions + audit trail. CSV for compliance officer.',
 'hipaa', 30,
 TRUE, FALSE, TRUE, FALSE, TRUE,
 'csv,json,pdf', 2190,
 TRUE, 'generic-https-json', FALSE, 20),

('sox-quarterly-financial-controls',
 'SOX Quarterly Financial Controls Kit',
 'Quarterly SOX evidence: governance decisions + audit trail for financial-affecting changes. XLSX + PDF.',
 'sox', 90,
 TRUE, FALSE, TRUE, FALSE, FALSE,
 'csv,json,xlsx,pdf', 2555,
 TRUE, 'generic-https-json', FALSE, 30),

('pci-dss-quarterly-cardholder',
 'PCI-DSS Quarterly Cardholder Data Kit',
 'Quarterly PCI evidence: data access logs + governance + audit for cardholder operations.',
 'pci_dss', 90,
 TRUE, FALSE, TRUE, FALSE, TRUE,
 'csv,json,pdf', 1095,
 TRUE, 'generic-https-json', FALSE, 40),

('soc2-quarterly-trust-criteria',
 'SOC2 Type II Quarterly Trust Criteria Kit',
 'Quarterly SOC2 evidence: full audit trail + quality reports + governance decisions. JSON for SOC2 auditor.',
 'soc2', 90,
 TRUE, TRUE, TRUE, FALSE, FALSE,
 'csv,json', 2555,
 FALSE, 'generic-https-json', TRUE, 50),

('iso27001-annual-isms',
 'ISO 27001 Annual ISMS Kit',
 'Annual ISMS evidence: full year of audit + governance + data access. Comprehensive PDF report.',
 'iso27001', 365,
 TRUE, FALSE, TRUE, FALSE, TRUE,
 'csv,json,pdf', 2555,
 TRUE, 'generic-https-json', FALSE, 60),

('generic-monthly-audit',
 'Generic Monthly Audit Kit',
 'Lightweight monthly audit for non-regulated tenants. Recommended baseline.',
 'generic_audit', 30,
 TRUE, FALSE, TRUE, FALSE, FALSE,
 'csv,json', 365,
 FALSE, NULL, TRUE, 70),

('generic-weekly-security',
 'Generic Weekly Security Kit',
 'Weekly security review: governance decisions + access logs. Recommended for security teams.',
 'generic_security', 7,
 TRUE, FALSE, TRUE, FALSE, TRUE,
 'csv,json', 365,
 FALSE, NULL, TRUE, 80);
