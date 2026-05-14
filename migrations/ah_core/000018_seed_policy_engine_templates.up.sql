-- Seed platform-managed DEFAULT POLICY ENGINE TEMPLATES in ah_core.
-- Templates are pre-configured policy bundles tenants enable to enforce
-- common compliance/safety profiles without writing rules from scratch.
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 §11 (governance — pluggable policy backends)
--   - GOV-002 PolicyEngine + StaticDenyPolicyEngine + LimitsBackedPolicyEngine
--
-- Catalog entries describe AVAILABLE templates — tenants opt-in by
-- creating a policy from a template (template values become the policy's
-- initial deny rules / approval rules).

CREATE TABLE IF NOT EXISTS ah_core.policy_engine_template (
    id                       UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- slug is the stable identifier.
    slug                     VARCHAR(64)  NOT NULL UNIQUE,
    display_name             VARCHAR(255) NOT NULL,
    description              TEXT         NOT NULL,
    -- compliance_profile classifies the regulatory framework.
    -- One of: gdpr / hipaa / sox / pci_dss / soc2 / iso27001 / generic_safety.
    compliance_profile       VARCHAR(32)  NOT NULL,
    -- engine_kind specifies which GOV-002 PolicyEngine variant to instantiate.
    -- One of: static_deny / limits_backed / chained.
    engine_kind              VARCHAR(32)  NOT NULL DEFAULT 'static_deny',
    -- deny_tools is the comma-separated list of tool slugs to deny by default.
    deny_tools               TEXT         NOT NULL DEFAULT '',
    -- require_approval_tools is the comma-separated list of tools that
    -- trigger PolicyRequireApproval (GOV-003 checkpoint).
    require_approval_tools   TEXT         NOT NULL DEFAULT '',
    -- obligations is the comma-separated list of obligations (e.g. audit_log,
    -- evidence_header) every Allow decision carries.
    obligations              TEXT         NOT NULL DEFAULT '',
    -- requires_admin_review: tenant admin must approve template enablement.
    requires_admin_review    BOOLEAN      NOT NULL DEFAULT FALSE,
    -- is_recommended marks platform-suggested defaults.
    is_recommended           BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order               INTEGER      NOT NULL DEFAULT 0,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_policy_template_slug              ON ah_core.policy_engine_template (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_policy_template_compliance_profile ON ah_core.policy_engine_template (compliance_profile);
CREATE INDEX IF NOT EXISTS idx_ah_core_policy_template_engine_kind       ON ah_core.policy_engine_template (engine_kind);
CREATE INDEX IF NOT EXISTS idx_ah_core_policy_template_is_active         ON ah_core.policy_engine_template (is_active);

-- ============================
-- 7 templates covering common compliance contexts
-- ============================
INSERT INTO ah_core.policy_engine_template
    (slug, display_name, description, compliance_profile, engine_kind,
     deny_tools, require_approval_tools, obligations,
     requires_admin_review, is_recommended, sort_order) VALUES

('generic-safety',
 'Generic Safety',
 'Baseline safety: blocks shell + DDL operations + force-push. Recommended for all new tenants.',
 'generic_safety', 'static_deny',
 'shell,execute-sql-ddl,git-force-push',
 '',
 'audit_log',
 FALSE, TRUE, 10),

('gdpr-strict',
 'GDPR Strict',
 'GDPR compliance: blocks PII export to non-EU regions, requires approval for any data export operation.',
 'gdpr', 'static_deny',
 'export-data-non-eu,share-pii-external',
 'export-data,download-customer-list,share-with-third-party',
 'audit_log,evidence_header,gdpr_consent_check',
 TRUE, FALSE, 20),

('hipaa-strict',
 'HIPAA Strict',
 'HIPAA compliance: blocks PHI handling outside covered systems, requires approval for any health data access.',
 'hipaa', 'static_deny',
 'export-phi-external,share-phi-non-baa-vendor',
 'access-patient-record,export-medical-data',
 'audit_log,evidence_header,phi_access_log',
 TRUE, FALSE, 30),

('sox-financial',
 'SOX Financial Controls',
 'SOX compliance for financial systems: requires approval for any change to revenue/journal/audit-trail tools.',
 'sox', 'static_deny',
 'modify-journal-entry-direct,delete-audit-record',
 'modify-revenue-rule,modify-journal-entry,export-financial-statement',
 'audit_log,sox_change_record,evidence_header',
 TRUE, FALSE, 40),

('pci-dss-cardholder',
 'PCI-DSS Cardholder Data',
 'PCI-DSS compliance: blocks raw card data handling, requires approval for tokenization/detokenization.',
 'pci_dss', 'static_deny',
 'log-raw-pan,store-cvv,export-cardholder-data-plain',
 'tokenize-pan,detokenize-pan,access-cardholder-vault',
 'audit_log,evidence_header,pci_dss_record',
 TRUE, FALSE, 50),

('soc2-baseline',
 'SOC2 Baseline',
 'SOC2 Type II baseline: requires approval for security/availability/processing-integrity sensitive operations.',
 'soc2', 'static_deny',
 'disable-encryption,delete-backup,modify-access-rule-direct',
 'modify-access-rule,disable-monitoring,modify-retention-policy',
 'audit_log,soc2_change_record',
 FALSE, TRUE, 60),

('iso27001-info-security',
 'ISO 27001 Information Security',
 'ISO 27001 compliance: blocks classified-data exposure, requires approval for system changes affecting confidentiality.',
 'iso27001', 'static_deny',
 'export-classified-data,share-classified-non-isms',
 'modify-classification-policy,disable-monitoring,export-internal-doc',
 'audit_log,iso27001_change_record',
 TRUE, FALSE, 70);
