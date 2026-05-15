-- Seed platform-managed DEFAULT SCHEDULED JOB TEMPLATES in ah_core.
-- Templates are pre-configured cron-like jobs tenants can enable to
-- automate recurring tasks without writing schedule definitions from
-- scratch. Pairs with FUTURE-003 BackgroundAgentRun infrastructure.

CREATE TABLE IF NOT EXISTS ah_core.scheduled_job_template (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            VARCHAR(64)  NOT NULL UNIQUE,
    display_name    VARCHAR(255) NOT NULL,
    description     TEXT         NOT NULL,
    -- job_kind classifies the job category.
    job_kind        VARCHAR(32)  NOT NULL,
    -- cron_expression is the standard 5-field cron string.
    cron_expression VARCHAR(64)  NOT NULL,
    -- target_workflow_slug references workflow_template (cross-table FK app-level).
    target_workflow_slug VARCHAR(64) NOT NULL,
    -- timezone defaults to UTC; tenants override in their own config.
    timezone        VARCHAR(64)  NOT NULL DEFAULT 'UTC',
    -- estimated_cost_per_run_usd helps tenants budget recurring spend.
    estimated_cost_per_run_usd NUMERIC(10,4) NOT NULL DEFAULT 0,
    -- requires_admin_approval: enabling creates ongoing cost; some
    -- jobs need admin to opt in.
    requires_admin_approval BOOLEAN NOT NULL DEFAULT FALSE,
    -- max_concurrent_runs limits how many instances of this job can
    -- be running at once (0 = unlimited; 1 = strict serial).
    max_concurrent_runs INTEGER  NOT NULL DEFAULT 1,
    is_recommended  BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order      INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_sched_job_template_slug    ON ah_core.scheduled_job_template (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_sched_job_template_kind    ON ah_core.scheduled_job_template (job_kind);
CREATE INDEX IF NOT EXISTS idx_ah_core_sched_job_template_active  ON ah_core.scheduled_job_template (is_active);

-- ============================
-- 8 templates covering common recurring patterns
-- ============================
INSERT INTO ah_core.scheduled_job_template
    (slug, display_name, description, job_kind, cron_expression,
     target_workflow_slug, timezone,
     estimated_cost_per_run_usd, requires_admin_approval,
     max_concurrent_runs, is_recommended, sort_order) VALUES

('daily-morning-briefing',
 'Daily Morning Briefing',
 'Generates a morning summary at 9am tenant timezone. Recommended for ops teams.',
 'reporting', '0 9 * * *',
 'research-brief', 'UTC',
 0.30, FALSE, 1,
 TRUE, 10),

('hourly-incident-triage',
 'Hourly Incident Triage',
 'Checks alerting backlog every hour and triages new incidents.',
 'monitoring', '0 * * * *',
 'incident-triage', 'UTC',
 0.10, FALSE, 1,
 TRUE, 20),

('weekly-coherence-report',
 'Weekly Coherence Report',
 'Builds HUMAN-005 coherence report every Monday 8am — surfaces drift.',
 'reporting', '0 8 * * MON',
 'research-brief', 'UTC',
 0.20, FALSE, 1,
 TRUE, 30),

('weekly-governance-export',
 'Weekly Governance Export',
 'Generates GOV-005 governance report every Sunday for compliance review.',
 'compliance', '0 6 * * SUN',
 'compliance-export', 'UTC',
 0.50, TRUE, 1,
 FALSE, 40),

('nightly-quality-rollup',
 'Nightly Quality Rollup',
 'Aggregates OBS-009 quality reports nightly; flags fail-rate > 10%.',
 'monitoring', '0 2 * * *',
 'research-brief', 'UTC',
 0.15, FALSE, 1,
 TRUE, 50),

('daily-cost-summary',
 'Daily Cost Summary',
 'Posts yesterday total LLM cost + projection for end of month.',
 'reporting', '0 7 * * *',
 'research-brief', 'UTC',
 0.05, FALSE, 1,
 FALSE, 60),

('monthly-decision-audit',
 'Monthly Decision Audit',
 'Reviews HUMAN-003 decision records from past month for consistency.',
 'compliance', '0 4 1 * *',
 'compliance-export', 'UTC',
 1.00, TRUE, 1,
 FALSE, 70),

('every-15min-stalled-run-sweep',
 'Stalled Run Sweep',
 'Every 15 minutes, scans for runs stuck >30min and force-cancels them.',
 'maintenance', '*/15 * * * *',
 'incident-triage', 'UTC',
 0.02, FALSE, 1,
 TRUE, 80);
