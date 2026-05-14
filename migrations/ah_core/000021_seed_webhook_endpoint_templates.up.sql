-- Seed platform-managed DEFAULT WEBHOOK ENDPOINT TEMPLATES in ah_core.
-- Templates are pre-configured outbound webhook destinations tenants
-- can subscribe to platform events without writing receiver code from scratch.
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 §6.1 (event subscriptions)
--   - GOV-001 audit trail + OBS-001 events that need external sinks

CREATE TABLE IF NOT EXISTS ah_core.webhook_endpoint_template (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            VARCHAR(64)  NOT NULL UNIQUE,
    display_name    VARCHAR(255) NOT NULL,
    description     TEXT         NOT NULL,
    -- target_kind classifies the receiver.
    -- One of: slack / teams / discord / pagerduty / opsgenie /
    --         generic_http / email / sms.
    target_kind     VARCHAR(32)  NOT NULL,
    -- url_template is the endpoint URL with placeholders.
    url_template    TEXT         NOT NULL,
    -- method is the HTTP verb (default POST).
    method          VARCHAR(8)   NOT NULL DEFAULT 'POST',
    -- payload_template is the JSON body template (with placeholders).
    payload_template TEXT        NOT NULL,
    -- subscribed_events is the comma-separated list of event types
    -- this endpoint receives.
    subscribed_events VARCHAR(512) NOT NULL DEFAULT '',
    -- requires_auth indicates whether endpoint needs credentials.
    requires_auth   BOOLEAN      NOT NULL DEFAULT FALSE,
    -- auth_type: api_key / bearer_token / hmac_signature / oauth2 / basic / none.
    auth_type       VARCHAR(32)  NOT NULL DEFAULT 'none',
    -- retry_policy: comma-separated retry attempts ms (e.g. "1000,5000,30000").
    retry_policy    VARCHAR(255) NOT NULL DEFAULT '1000,5000,30000',
    -- documentation_url for vendor receiver docs.
    documentation_url TEXT,
    is_recommended  BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order      INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_webhook_template_slug      ON ah_core.webhook_endpoint_template (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_webhook_template_kind      ON ah_core.webhook_endpoint_template (target_kind);
CREATE INDEX IF NOT EXISTS idx_ah_core_webhook_template_is_active ON ah_core.webhook_endpoint_template (is_active);

-- ============================
-- 8 templates covering common notification destinations
-- ============================
INSERT INTO ah_core.webhook_endpoint_template
    (slug, display_name, description, target_kind, url_template, method,
     payload_template, subscribed_events,
     requires_auth, auth_type, retry_policy,
     documentation_url, is_recommended, sort_order) VALUES

('slack-incoming-webhook',
 'Slack Incoming Webhook',
 'POST event notifications to a Slack channel via incoming webhook URL.',
 'slack', '{{slack_webhook_url}}', 'POST',
 '{"text": "{{event_summary}}", "attachments": [{"color": "{{severity_color}}", "fields": [{"title": "Run ID", "value": "{{run_id}}", "short": true}]}]}',
 'run_complete,quality_report,governance_alert,checkpoint_pending',
 TRUE, 'none', '1000,5000,30000',
 'https://api.slack.com/messaging/webhooks', TRUE, 10),

('teams-incoming-webhook',
 'Microsoft Teams Webhook',
 'POST event notifications to a Microsoft Teams channel.',
 'teams', '{{teams_webhook_url}}', 'POST',
 '{"@type": "MessageCard", "@context": "http://schema.org/extensions", "summary": "{{event_summary}}", "themeColor": "{{severity_color_hex}}", "title": "AgentHub Notification", "text": "{{event_details}}"}',
 'run_complete,quality_report,governance_alert',
 TRUE, 'none', '1000,5000,30000',
 'https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook',
 FALSE, 20),

('discord-webhook',
 'Discord Webhook',
 'POST event notifications to a Discord channel.',
 'discord', '{{discord_webhook_url}}', 'POST',
 '{"content": "{{event_summary}}", "embeds": [{"title": "Run {{run_id}}", "description": "{{event_details}}", "color": {{severity_color_int}}}]}',
 'run_complete,quality_report',
 TRUE, 'none', '1000,5000,30000',
 'https://support.discord.com/hc/en-us/articles/228383668',
 FALSE, 30),

('pagerduty-events-v2',
 'PagerDuty Events API v2',
 'Trigger PagerDuty incidents on critical alerts (paging integration).',
 'pagerduty', 'https://events.pagerduty.com/v2/enqueue', 'POST',
 '{"routing_key": "{{routing_key}}", "event_action": "trigger", "payload": {"summary": "{{event_summary}}", "severity": "{{severity}}", "source": "agenthub", "custom_details": {"run_id": "{{run_id}}"}}}',
 'governance_alert,checkpoint_timeout,silent_failure',
 TRUE, 'api_key', '1000,5000,30000',
 'https://developer.pagerduty.com/docs/3d063fd4814a6-events-api-v2-overview',
 TRUE, 40),

('opsgenie-alert-api',
 'Opsgenie Alert API',
 'Create Opsgenie alerts for incident escalation.',
 'opsgenie', 'https://api.opsgenie.com/v2/alerts', 'POST',
 '{"message": "{{event_summary}}", "alias": "{{run_id}}", "priority": "{{priority}}", "details": {"run_id": "{{run_id}}", "tenant": "{{tenant_id}}"}}',
 'governance_alert,silent_failure',
 TRUE, 'api_key', '1000,5000,30000',
 'https://docs.opsgenie.com/docs/alert-api',
 FALSE, 50),

('generic-https-json',
 'Generic HTTPS JSON Endpoint',
 'POST raw event JSON to a custom HTTPS endpoint. Bearer-token auth recommended.',
 'generic_http', '{{custom_endpoint_url}}', 'POST',
 '{{event_json}}',
 '',
 TRUE, 'bearer_token', '1000,5000,30000,120000',
 NULL, TRUE, 60),

('email-smtp-relay',
 'Email (SMTP Relay)',
 'Send event summaries via email through configured SMTP relay.',
 'email', 'smtp://{{smtp_relay}}', 'POST',
 '{"to": "{{recipient}}", "subject": "AgentHub: {{event_summary}}", "body": "{{event_details}}"}',
 'governance_alert,run_complete',
 TRUE, 'basic', '5000,30000,300000',
 NULL, FALSE, 70),

('twilio-sms',
 'Twilio SMS',
 'Send urgent event notifications via SMS through Twilio.',
 'sms', 'https://api.twilio.com/2010-04-01/Accounts/{{account_sid}}/Messages.json', 'POST',
 '{"From": "{{from_number}}", "To": "{{to_number}}", "Body": "AgentHub: {{event_summary}}"}',
 'silent_failure,checkpoint_timeout',
 TRUE, 'basic', '5000,30000,120000',
 'https://www.twilio.com/docs/sms/quickstart',
 FALSE, 80);
