-- Seed platform-managed BACKGROUND JOB TRIGGER TEMPLATES in ah_core.
-- Templates are blueprints paired with FUTURE-003 background_run loop
-- (BackgroundTrigger enum: schedule_cron / event_arrived /
-- threshold_crossed / absence_timeout). Tenants instantiate these to
-- drive worker-claim background runs without inventing thresholds.

CREATE TABLE IF NOT EXISTS ah_core.background_job_trigger_template (
    id                          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- slug is dotted-path namespace (kebab-case).
    slug                        VARCHAR(96)  NOT NULL UNIQUE,
    name                        VARCHAR(160) NOT NULL,
    description                 TEXT         NOT NULL,
    -- trigger_kind matches FUTURE-003 BackgroundTrigger enum byte-for-byte.
    trigger_kind                VARCHAR(48)  NOT NULL,
    -- trigger_config_json is the trigger-kind-specific configuration
    -- shape (cron expression for schedule_cron, event_name + filter for
    -- event_arrived, metric + threshold + comparator for threshold_crossed,
    -- absence_window for absence_timeout).
    trigger_config_json         TEXT         NOT NULL,
    -- target_workflow_slug references a workflow_template slug to run.
    target_workflow_slug        VARCHAR(96)  NOT NULL,
    -- max_runs_per_day caps how often the trigger fires (rate limit).
    max_runs_per_day            INTEGER      NOT NULL DEFAULT 24,
    -- claim_timeout_seconds matches FUTURE-003 worker-claim model;
    -- runs unfinished after this are reclaimable.
    claim_timeout_seconds       INTEGER      NOT NULL DEFAULT 300,
    -- on_failure_action describes what runner does on consecutive failures
    -- ("retry_with_backoff", "alert_admin", "quarantine_trigger").
    on_failure_action           VARCHAR(48)  NOT NULL,
    -- max_consecutive_failures before on_failure_action fires.
    max_consecutive_failures    INTEGER      NOT NULL DEFAULT 3,
    -- requires_admin_review marks triggers that admin must vet before activating
    -- (regulatory automations, irreversible side effects).
    requires_admin_review       BOOLEAN      NOT NULL DEFAULT FALSE,
    -- is_recommended marks safe one-click defaults.
    is_recommended              BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order                  INTEGER      NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_bgjob_kind     ON ah_core.background_job_trigger_template (trigger_kind);
CREATE INDEX IF NOT EXISTS idx_ah_core_bgjob_workflow ON ah_core.background_job_trigger_template (target_workflow_slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_bgjob_active   ON ah_core.background_job_trigger_template (is_active);
CREATE INDEX IF NOT EXISTS idx_ah_core_bgjob_rec      ON ah_core.background_job_trigger_template (is_recommended) WHERE is_recommended = TRUE;

-- Seed catalog: 8 templates, 2 per FUTURE-003 BackgroundTrigger kind.
-- target_workflow_slug references existing seeded workflow_templates
-- (validated cross-table via integration test).

INSERT INTO ah_core.background_job_trigger_template
    (slug, name, description, trigger_kind, trigger_config_json, target_workflow_slug, max_runs_per_day, claim_timeout_seconds, on_failure_action, max_consecutive_failures, requires_admin_review, is_recommended, sort_order)
VALUES
    -- schedule_cron (2): daily-digest + hourly-health-check.
    ('daily-digest-summary',
     'Daily Digest Summary',
     'Generates a daily digest of agent activity and emails it to admins. Runs once per day at 08:00 UTC. Useful for ops awareness without manual log scraping.',
     'schedule_cron', '{"cron":"0 8 * * *","timezone":"UTC"}',
     'document-summary', 1, 600, 'alert_admin', 3, FALSE, TRUE, 10),

    ('hourly-health-check',
     'Hourly Health Check',
     'Runs a health-check workflow every hour to verify all configured tools/MCPs respond. Provides early signal of upstream outages.',
     'schedule_cron', '{"cron":"0 * * * *","timezone":"UTC"}',
     'incident-triage', 24, 300, 'retry_with_backoff', 3, FALSE, TRUE, 20),

    -- event_arrived (2): kb-document-uploaded + new-user-onboarded.
    ('on-kb-document-uploaded',
     'On KB Document Uploaded → Re-Index',
     'Fires when a knowledge_base document is uploaded; runs the embed-and-index workflow so RAG queries see new content within minutes (not on next cron).',
     'event_arrived', '{"event_name":"kb.document.uploaded"}',
     'document-summary', 1000, 1200, 'retry_with_backoff', 5, FALSE, TRUE, 30),

    ('on-new-user-onboarded',
     'On New User Onboarded → Run Onboarding Workflow',
     'Fires when a new tenant user is provisioned; runs the onboarding workflow (welcome message + capability baseline snapshot per FUTURE-006).',
     'event_arrived', '{"event_name":"user.onboarded"}',
     'customer-onboarding', 200, 600, 'alert_admin', 3, FALSE, TRUE, 40),

    -- threshold_crossed (2): cost-budget + error-rate.
    ('on-cost-budget-exceeded',
     'On Cost Budget Exceeded → Alert + Pause',
     'Fires when daily LLM cost crosses tenant budget threshold. Runs a notification workflow and (admin opt-in) pauses non-critical agents. Requires admin review because pausing has business impact.',
     'threshold_crossed', '{"metric":"llm.cost.daily_usd","threshold":100,"comparator":"gt"}',
     'incident-triage', 24, 300, 'alert_admin', 1, TRUE, TRUE, 50),

    ('on-error-rate-spike',
     'On Error Rate Spike → Run Triage',
     'Fires when 5xx error rate from agent API crosses 1%. Runs triage workflow that captures recent traces and notifies on-call.',
     'threshold_crossed', '{"metric":"api.error_rate.5xx","threshold":0.01,"comparator":"gt","window_seconds":300}',
     'incident-triage', 48, 600, 'alert_admin', 1, FALSE, TRUE, 60),

    -- absence_timeout (2): user-inactive + agent-inactive.
    ('on-user-inactive-30-days',
     'On User Inactive 30 Days → Capability Snapshot',
     'Fires when a user has not interacted with any agent for 30 days. Runs a capability-snapshot workflow (marks dormant) and recommends decision_independence review per FUTURE-006.',
     'absence_timeout', '{"absence_subject":"user","absence_seconds":2592000}',
     'customer-onboarding', 100, 600, 'retry_with_backoff', 3, FALSE, TRUE, 70),

    ('on-agent-unused-90-days',
     'On Agent Unused 90 Days → Archive Recommendation',
     'Fires when a published agent has not been invoked for 90 days. Runs an archival workflow that flags it for admin review (do not auto-archive — that loses tenant work).',
     'absence_timeout', '{"absence_subject":"agent","absence_seconds":7776000}',
     'compliance-export', 50, 1200, 'alert_admin', 3, TRUE, TRUE, 80)
ON CONFLICT (slug) DO NOTHING;
