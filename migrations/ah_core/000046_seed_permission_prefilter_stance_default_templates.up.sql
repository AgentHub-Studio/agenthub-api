-- PERM-004-paired: permission pre-filter stance default templates.
-- 4 stance blueprints covering interactive/unattended/lockdown/audit
-- so fresh tenants pick a pre-filter posture without inventing
-- semantics. PrefilterStance enum has only 2 values
-- (show_confirm/hide_confirm); templates also bundle a recommended
-- baseline deny pattern so admins do not start from scratch.

CREATE TABLE IF NOT EXISTS ah_core.permission_prefilter_stance_default_template (
    id                              UUID PRIMARY KEY,
    slug                            TEXT NOT NULL UNIQUE,
    name                            TEXT NOT NULL,
    description                     TEXT NOT NULL,
    target_stance                   TEXT NOT NULL,
    target_use_case                 TEXT NOT NULL,
    baseline_deny_patterns          JSONB NOT NULL DEFAULT '[]'::jsonb,
    baseline_confirm_patterns       JSONB NOT NULL DEFAULT '[]'::jsonb,
    recommended_for_tenant_kind     TEXT NOT NULL DEFAULT 'general',
    requires_admin_review           BOOLEAN NOT NULL DEFAULT FALSE,
    is_recommended                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order                      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_perm_prefilter_stance_dt_stance
    ON ah_core.permission_prefilter_stance_default_template(target_stance);
CREATE INDEX IF NOT EXISTS idx_perm_prefilter_stance_dt_use_case
    ON ah_core.permission_prefilter_stance_default_template(target_use_case);
CREATE INDEX IF NOT EXISTS idx_perm_prefilter_stance_dt_active
    ON ah_core.permission_prefilter_stance_default_template(is_active);
CREATE INDEX IF NOT EXISTS idx_perm_prefilter_stance_dt_recommended
    ON ah_core.permission_prefilter_stance_default_template(is_recommended)
    WHERE is_recommended = TRUE;

INSERT INTO ah_core.permission_prefilter_stance_default_template
    (id, slug, name, description, target_stance, target_use_case,
     baseline_deny_patterns, baseline_confirm_patterns,
     recommended_for_tenant_kind, requires_admin_review,
     is_recommended, is_active, sort_order)
VALUES
    ('33333333-3333-3333-3333-000000000001',
     'interactive-default',
     'Interactive Default',
     'Default for chat sessions where a human is present and can answer call-time confirm prompts. Drops dangerous Bash subcommands at pool assembly so the LLM never proposes them; keeps Write visible (confirm tier).',
     'show_confirm',
     'interactive_chat',
     '["Bash(rm -rf)","Bash(sudo)"]'::jsonb,
     '["Write","Edit"]'::jsonb,
     'general', FALSE, TRUE, TRUE, 10),

    ('33333333-3333-3333-3333-000000000002',
     'unattended-batch',
     'Unattended Batch Run',
     'For background subagents and scheduled jobs where no human can confirm. Hides confirm-tier tools entirely so the LLM does not stall waiting for a prompt that will never be answered.',
     'hide_confirm',
     'background_job',
     '["Bash(rm -rf)","Bash(sudo)"]'::jsonb,
     '["Write","Edit"]'::jsonb,
     'general', TRUE, TRUE, TRUE, 20),

    ('33333333-3333-3333-3333-000000000003',
     'lockdown-readonly',
     'Lockdown Read-Only',
     'Incident-response posture: denies every mutating tool at pool assembly so the LLM only sees inspection tools. Used during security investigations where the agent must not change state under any circumstance.',
     'hide_confirm',
     'incident_response',
     '["Bash","Write","Edit","execute-sql"]'::jsonb,
     '[]'::jsonb,
     'general', TRUE, TRUE, TRUE, 30),

    ('33333333-3333-3333-3333-000000000004',
     'audit-strict-trace',
     'Audit-Strict Trace',
     'Compliance-sensitive posture. Keeps confirm-tier visible so the human is in the loop on every mutating call AND records every pre-filter decision in the audit log. Sets a baseline deny pattern for known-dangerous shell invocations.',
     'show_confirm',
     'compliance_audit',
     '["Bash(rm -rf)","Bash(sudo)","Bash(curl * | sh)"]'::jsonb,
     '["Write","Edit","execute-sql"]'::jsonb,
     'general', TRUE, TRUE, TRUE, 40)
ON CONFLICT (slug) DO NOTHING;
