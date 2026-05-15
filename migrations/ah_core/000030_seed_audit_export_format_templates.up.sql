-- Seed platform-managed AUDIT EXPORT FORMAT TEMPLATES in ah_core.
-- Templates are blueprints paired with FUTURE-005 AuditExporter
-- (regulator-facing signed bundles). Each template ties an
-- AuditExportFormat + SignatureAlgorithm + filename pattern + retention
-- policy into one ready-to-instantiate export configuration.

CREATE TABLE IF NOT EXISTS ah_core.audit_export_format_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- slug is dotted-path namespace (kebab-case).
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- export_format MATCHES FUTURE-005 AuditExportFormat enum byte-for-byte.
    export_format               VARCHAR(32)  NOT NULL,
    -- signature_algorithm MATCHES FUTURE-005 SignatureAlgorithm enum.
    signature_algorithm         VARCHAR(32)  NOT NULL,
    -- compliance_profile is a label tying this template to a regulatory
    -- frame (gdpr/hipaa/sox/pci_dss/soc2/iso27001/generic_audit).
    compliance_profile          VARCHAR(32)  NOT NULL,
    -- filename_pattern is a Go template (or simple substitution) used
    -- to generate the export filename. Tokens: {tenant}, {kit}, {period}.
    filename_pattern            VARCHAR(160) NOT NULL,
    -- retention_days is the minimum platform-side retention window for
    -- the generated bundle (regulators often require multi-year retention).
    retention_days              INTEGER      NOT NULL,
    -- includes_raw_evidence indicates whether the bundle ships raw
    -- evidence files (true) or only the aggregated report (false).
    includes_raw_evidence       BOOLEAN      NOT NULL DEFAULT TRUE,
    -- requires_admin_review marks templates that require admin sign-off
    -- before generation (typically regulator-facing high-stakes profiles).
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    -- is_recommended marks safe one-click defaults.
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_aef_format     ON ah_core.audit_export_format_template (export_format);
CREATE INDEX IF NOT EXISTS idx_ah_core_aef_profile    ON ah_core.audit_export_format_template (compliance_profile);
CREATE INDEX IF NOT EXISTS idx_ah_core_aef_active     ON ah_core.audit_export_format_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_aef_rec        ON ah_core.audit_export_format_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 8 templates covering the 6 compliance profiles + 2
-- generic patterns. Mix of formats (csv/json/pdf/xlsx/zip_bundle) and
-- signatures (sha256 / sha256-rsa / sha256-ecdsa / ed25519).

INSERT INTO ah_core.audit_export_format_template
    (slug, name, description, export_format, signature_algorithm, compliance_profile, filename_pattern, retention_days, includes_raw_evidence, requires_admin_review, is_recommended, sort_order)
VALUES
    -- generic baselines (2): one quick-look, one full bundle.
    ('generic-monthly-summary',
     'Generic Monthly Summary (PDF, sha256)',
     'Monthly governance summary as a single PDF with a sha256 checksum. Use for internal review or low-stakes audits where a signed bundle is overkill.',
     'pdf', 'sha256', 'generic_audit',
     'agenthub-{tenant}-monthly-{period}.pdf',
     365, FALSE, FALSE, TRUE, 10),

    ('generic-quarterly-bundle',
     'Generic Quarterly Bundle (zip_bundle, sha256-rsa)',
     'Quarterly full evidence bundle as a zip with sha256-rsa signature. Default option for tenants without a specific compliance frame.',
     'zip_bundle', 'sha256-rsa', 'generic_audit',
     'agenthub-{tenant}-{kit}-{period}.zip',
     1095, TRUE, FALSE, TRUE, 20),

    -- regulated profiles (6).
    ('gdpr-data-subject-export',
     'GDPR Data-Subject Request Export (json, ed25519)',
     'GDPR Article 15 data-subject access request export — JSON for machine parsing + ed25519 signature for non-repudiation. Fast turnaround required by GDPR (1 month).',
     'json', 'ed25519', 'gdpr',
     'gdpr-{tenant}-dsar-{period}.json',
     2555, TRUE, TRUE, TRUE, 30),

    ('hipaa-phi-access-bundle',
     'HIPAA PHI Access Bundle (zip_bundle, sha256-rsa)',
     'HIPAA-compliant PHI access audit bundle. zip_bundle with sha256-rsa signature; 6-year retention per HIPAA security rule.',
     'zip_bundle', 'sha256-rsa', 'hipaa',
     'hipaa-{tenant}-phi-{period}.zip',
     2190, TRUE, TRUE, TRUE, 40),

    ('sox-financial-controls-bundle',
     'SOX Financial Controls Bundle (xlsx, sha256-ecdsa)',
     'SOX section 404 controls evidence as XLSX (auditor-friendly spreadsheet) + sha256-ecdsa signature. 7-year retention per SOX records-management.',
     'xlsx', 'sha256-ecdsa', 'sox',
     'sox-{tenant}-controls-{period}.xlsx',
     2555, TRUE, TRUE, TRUE, 50),

    ('pci-cardholder-data-export',
     'PCI-DSS Cardholder Data Export (zip_bundle, ed25519)',
     'PCI-DSS req 10 cardholder data access export — zip with ed25519 signature for forensics + tamper-evidence. Minimum 1 year + immediate access for last 90 days.',
     'zip_bundle', 'ed25519', 'pci_dss',
     'pci-{tenant}-cde-{period}.zip',
     365, TRUE, TRUE, TRUE, 60),

    ('soc2-trust-service-criteria',
     'SOC 2 Trust Service Criteria Bundle (zip_bundle, sha256-rsa)',
     'SOC 2 Type II annual audit bundle covering CC1-CC9 trust service criteria. zip + sha256-rsa for auditor verification. Recommended one-click for SaaS tenants.',
     'zip_bundle', 'sha256-rsa', 'soc2',
     'soc2-{tenant}-tsc-{period}.zip',
     1095, TRUE, FALSE, TRUE, 70),

    ('iso27001-isms-controls-csv',
     'ISO 27001 ISMS Controls Export (csv, sha256-rsa)',
     'ISO 27001 ISMS Annex A controls evidence as CSV (analyst-friendly) + sha256-rsa signature. 3-year retention to cover certification cycle.',
     'csv', 'sha256-rsa', 'iso27001',
     'iso27001-{tenant}-isms-{period}.csv',
     1095, TRUE, TRUE, TRUE, 80)
ON CONFLICT (slug) DO NOTHING;
