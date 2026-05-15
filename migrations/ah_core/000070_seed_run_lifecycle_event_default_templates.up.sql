-- EXT-002a paired seed: run lifecycle event templates for web-adapted hook events.
-- Provides 6 starter templates covering the four new events (RunStarted, RunComplete,
-- ContextWindowAlert, KnowledgeBaseQueried) with recommended handler_kind presets
-- (webhook / notification / audit) so fresh tenants know what to wire up.
--
-- Idempotent: ON CONFLICT (slug) DO NOTHING.

CREATE TABLE IF NOT EXISTS ah_core.run_lifecycle_event_template (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug             TEXT        UNIQUE NOT NULL,
    hook_event       TEXT        NOT NULL
                         CHECK (hook_event IN (
                             'RunStarted', 'RunComplete',
                             'ContextWindowAlert', 'KnowledgeBaseQueried'
                         )),
    handler_kind     TEXT        NOT NULL
                         CHECK (handler_kind IN ('webhook', 'notification', 'audit')),
    description      TEXT        NOT NULL,
    handler_config   JSONB       NOT NULL DEFAULT '{}',
    enabled_by_default BOOLEAN   NOT NULL DEFAULT false,
    sort_order       INTEGER     NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO ah_core.run_lifecycle_event_template
    (id, slug, hook_event, handler_kind, description, handler_config, enabled_by_default, sort_order)
VALUES
    (
        'dddddddd-0001-0000-0000-000000000001',
        'run-started-audit-log',
        'RunStarted',
        'audit',
        'Write a structured audit entry to ClickHouse when an agentic run begins, capturing tenant, agent slug, session id, and timestamp.',
        '{"sink":"clickhouse","table":"agenthub_run_audit","fields":["tenant_id","agent_slug","session_id","started_at"]}',
        true,
        10
    ),
    (
        'dddddddd-0002-0000-0000-000000000002',
        'run-complete-webhook-notify',
        'RunComplete',
        'webhook',
        'POST run summary (agent slug, duration_ms, tool_call_count, stop_reason) to a configurable webhook URL when the run finishes.',
        '{"method":"POST","content_type":"application/json","payload_template":{"agent":"$AGENT_SLUG","duration_ms":"$DURATION_MS","tool_calls":"$TOOL_CALL_COUNT","stop_reason":"$STOP_REASON"}}',
        false,
        20
    ),
    (
        'dddddddd-0003-0000-0000-000000000003',
        'run-complete-notification-slack',
        'RunComplete',
        'notification',
        'Send a Slack notification when an agentic run completes so the operator is alerted without polling the UI.',
        '{"channel":"#agenthub-runs","format":"text","template":"Run completed: $AGENT_SLUG finished in $DURATION_MS ms (stop: $STOP_REASON)"}',
        false,
        30
    ),
    (
        'dddddddd-0004-0000-0000-000000000004',
        'context-window-alert-compact',
        'ContextWindowAlert',
        'notification',
        'Emit an in-app notification when the context budget reaches 80 percent, prompting the operator to trigger a manual compact or adjust the context strategy.',
        '{"threshold_pct":80,"notification_target":"operator","severity":"warning","message":"Context window at $USED_PCT%. Consider compacting."}',
        true,
        40
    ),
    (
        'dddddddd-0005-0000-0000-000000000005',
        'knowledge-base-queried-audit',
        'KnowledgeBaseQueried',
        'audit',
        'Record every RAG vector-search to the audit log for LGPD/GDPR compliance evidence, capturing knowledge base id, query hash, and result count.',
        '{"sink":"clickhouse","table":"agenthub_kb_audit","fields":["kb_id","query_hash","result_count","queried_at"]}',
        true,
        50
    ),
    (
        'dddddddd-0006-0000-0000-000000000006',
        'knowledge-base-queried-webhook',
        'KnowledgeBaseQueried',
        'webhook',
        'POST RAG query metadata to an external DLP or data-governance webhook so it can verify that the knowledge base access is policy-compliant.',
        '{"method":"POST","content_type":"application/json","payload_template":{"kb_id":"$KB_ID","query":"$QUERY_HASH","tenant":"$TENANT_ID"}}',
        false,
        60
    )
ON CONFLICT (slug) DO NOTHING;
