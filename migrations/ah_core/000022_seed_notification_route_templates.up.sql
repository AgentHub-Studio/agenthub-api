-- Seed platform-managed DEFAULT NOTIFICATION ROUTE TEMPLATES in ah_core.
-- Routes connect TRIGGERS (event types + filters) → SINKS (webhook
-- endpoint templates). Pre-configured for common alerting patterns
-- so tenants don't manually wire each event type to each destination.
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 §11 (alerting routing — severity ladders)
--   - GOV-001 audit + OBS-001 events + GOV-005 governance reports

CREATE TABLE IF NOT EXISTS ah_core.notification_route_template (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            VARCHAR(64)  NOT NULL UNIQUE,
    display_name    VARCHAR(255) NOT NULL,
    description     TEXT         NOT NULL,
    -- trigger_event is the event type that fires this route.
    trigger_event   VARCHAR(64)  NOT NULL,
    -- min_severity filters by severity (info / warn / critical).
    -- Empty = all severities.
    min_severity    VARCHAR(16)  NOT NULL DEFAULT '',
    -- target_webhook_template_slug references ah_core.webhook_endpoint_template.slug.
    target_webhook_template_slug VARCHAR(64) NOT NULL,
    -- aggregation_window_seconds: when > 0, batch events within window
    -- before firing route (reduces alert spam).
    aggregation_window_seconds INTEGER NOT NULL DEFAULT 0,
    -- max_per_hour rate limits the route (0 = unlimited).
    max_per_hour    INTEGER      NOT NULL DEFAULT 0,
    -- requires_dedup: true = dedup by (event_type + subject_id) within window.
    requires_dedup  BOOLEAN      NOT NULL DEFAULT FALSE,
    -- is_recommended marks platform-suggested defaults.
    is_recommended  BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order      INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_notif_route_template_slug    ON ah_core.notification_route_template (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_notif_route_template_event   ON ah_core.notification_route_template (trigger_event);
CREATE INDEX IF NOT EXISTS idx_ah_core_notif_route_template_target  ON ah_core.notification_route_template (target_webhook_template_slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_notif_route_template_active  ON ah_core.notification_route_template (is_active);

-- ============================
-- 8 routes covering common alerting patterns
-- ============================
INSERT INTO ah_core.notification_route_template
    (slug, display_name, description, trigger_event, min_severity,
     target_webhook_template_slug, aggregation_window_seconds, max_per_hour,
     requires_dedup, is_recommended, sort_order) VALUES

('runs-to-slack',
 'Run Completions to Slack',
 'Fire Slack notification on every run_complete (with 60s aggregation to reduce spam).',
 'run_complete', '',
 'slack-incoming-webhook', 60, 100,
 FALSE, TRUE, 10),

('quality-fails-to-slack',
 'Quality Failures to Slack',
 'Notify on EvalFail quality reports (OBS-009) — surface bad agent outputs.',
 'quality_report', 'critical',
 'slack-incoming-webhook', 0, 50,
 TRUE, TRUE, 20),

('governance-alerts-to-pagerduty',
 'Governance Alerts to PagerDuty',
 'Page on critical governance alerts (GOV-005) — compliance events that need oncall response.',
 'governance_alert', 'critical',
 'pagerduty-events-v2', 0, 20,
 TRUE, TRUE, 30),

('checkpoint-pending-to-slack',
 'Pending Checkpoints to Slack',
 'Notify when GOV-003/HUMAN-004 checkpoint awaits human response.',
 'checkpoint_pending', '',
 'slack-incoming-webhook', 0, 200,
 TRUE, TRUE, 40),

('checkpoint-timeout-to-pagerduty',
 'Checkpoint Timeouts to PagerDuty',
 'Page when checkpoints time out without human response — risk of stalled runs.',
 'checkpoint_timeout', 'warn',
 'pagerduty-events-v2', 0, 10,
 TRUE, FALSE, 50),

('silent-failure-to-pagerduty',
 'Silent Failure Detection to PagerDuty',
 'Page when OBS-007 silent failure detection fires — agent stuck or stalled.',
 'silent_failure', 'critical',
 'pagerduty-events-v2', 0, 10,
 FALSE, TRUE, 60),

('audit-events-to-webhook',
 'Audit Events to Compliance Webhook',
 'Stream all audit events (GOV-001) to tenant compliance webhook.',
 'audit_event', '',
 'generic-https-json', 0, 0,
 FALSE, FALSE, 70),

('coherence-drift-to-slack',
 'Coherence Drift Findings to Slack',
 'Notify on HUMAN-005 coherence report findings (drift detection).',
 'coherence_drift', 'warn',
 'slack-incoming-webhook', 300, 30,
 TRUE, FALSE, 80);
