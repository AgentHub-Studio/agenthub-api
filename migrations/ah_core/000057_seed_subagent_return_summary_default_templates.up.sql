-- SUB-010-paired: subagent return summary default templates.
-- 5 pre-vetted return summary shapes spanning outcome variety so fresh
-- tenants have ready summary patterns matching SubagentReturnOutcome
-- enum + sample redaction posture.

CREATE TABLE IF NOT EXISTS ah_core.subagent_return_summary_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_outcome                  TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    sample_findings                 JSONB NOT NULL DEFAULT '[]'::jsonb,
    sample_artifacts                JSONB NOT NULL DEFAULT '[]'::jsonb,
    sample_next_steps               JSONB NOT NULL DEFAULT '[]'::jsonb,
    redact_findings                 BOOLEAN NOT NULL DEFAULT FALSE,
    redact_artifacts                BOOLEAN NOT NULL DEFAULT FALSE,
    redact_transcript_hash          BOOLEAN NOT NULL DEFAULT FALSE,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_subagent_return_dt_outcome
    ON ah_core.subagent_return_summary_default_template(target_outcome);
CREATE INDEX IF NOT EXISTS idx_subagent_return_dt_use_case
    ON ah_core.subagent_return_summary_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_subagent_return_dt_active
    ON ah_core.subagent_return_summary_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_subagent_return_dt_recommended
    ON ah_core.subagent_return_summary_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.subagent_return_summary_default_template
    (id, slug, name, description, target_outcome, target_use_case,
     sample_findings, sample_artifacts, sample_next_steps,
     redact_findings, redact_artifacts, redact_transcript_hash,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('eeeeeeee-eeee-eeee-eeee-000000000001',
     'success-with-artifacts',
     'Success — With Artifacts',
     'Routine successful completion: the subagent finished its task and produced concrete artifacts. Sample shows findings + artifacts + next-steps populated; no redaction.',
     'success', 'task_completion',
     '["All 12 endpoints documented","No deprecated patterns found"]'::jsonb,
     '["/tmp/api-docs.md","/tmp/coverage-report.html"]'::jsonb,
     '["Review with API team"]'::jsonb,
     FALSE, FALSE, FALSE,
     'general', FALSE, TRUE, TRUE, 10),

    ('eeeeeeee-eeee-eeee-eeee-000000000002',
     'partial-needs-followup',
     'Partial — Needs Follow-up',
     'Subagent finished part of the task but hit a constraint (max turns / timeout / permission). Sample emphasises next-steps so the parent agent (or human) can continue.',
     'partial', 'incremental_progress',
     '["Indexed 8 of 12 files before timeout"]'::jsonb,
     '["/tmp/partial-index.json"]'::jsonb,
     '["Re-run with longer timeout","Index remaining 4 files manually"]'::jsonb,
     FALSE, FALSE, FALSE,
     'general', FALSE, TRUE, TRUE, 20),

    ('eeeeeeee-eeee-eeee-eeee-000000000003',
     'failed-error',
     'Failed — Error',
     'Subagent could not complete (exhausted retries / unrecoverable error / gave up). Sample emphasises findings explaining WHY so the parent can diagnose without reading the transcript.',
     'failed', 'error_diagnosis',
     '["Connection refused after 3 retries","Tool execute-sql returned permission_denied"]'::jsonb,
     '[]'::jsonb,
     '["Verify VPN connectivity","Request execute-sql permission grant"]'::jsonb,
     FALSE, FALSE, FALSE,
     'general', TRUE, TRUE, TRUE, 30),

    ('eeeeeeee-eeee-eeee-eeee-000000000004',
     'aborted-by-parent',
     'Aborted — By Parent',
     'Subagent run was aborted before completion (parent cancelled / user changed direction). Sample emphasises whatever partial signal was captured before abort.',
     'aborted', 'interruption_handling',
     '["Initial signal: high error rate observed before abort"]'::jsonb,
     '[]'::jsonb,
     '[]'::jsonb,
     FALSE, FALSE, FALSE,
     'general', TRUE, TRUE, TRUE, 40),

    ('eeeeeeee-eeee-eeee-eeee-000000000005',
     'audit-with-redaction',
     'Audit — With Redaction',
     'Compliance-strict shape: findings/artifacts/transcript_hash all redacted on emission. Used when the summary travels outside the tenant boundary (marketplace agent hand-off / external auditor export).',
     'success', 'compliance_export',
     '["[finding placeholder]"]'::jsonb,
     '["[artifact placeholder]"]'::jsonb,
     '["[next-step placeholder]"]'::jsonb,
     TRUE, TRUE, TRUE,
     'general', TRUE, TRUE, TRUE, 50)
ON CONFLICT (slug) DO NOTHING;
