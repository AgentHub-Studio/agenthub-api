-- Seed platform-managed DEFAULT TELEMETRY SINKS in ah_core.
-- Tenants need pre-configured destinations for OBS-001 events,
-- OBS-009 quality reports, GOV-005 governance reports, and metrics.
--
-- Inspired by:
--   - PDF arXiv:2604.14228v1 §7 (observability) + §11 (audit)
--   - CLAUDE.md (OpenTelemetry + Prometheus + Grafana stack)
--
-- Catalog entries describe AVAILABLE sinks — tenants opt-in by enabling
-- a sink and providing endpoint/credentials in their tenant settings.

CREATE TABLE IF NOT EXISTS ah_core.telemetry_sink (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    -- slug is the stable identifier.
    slug            VARCHAR(64)  NOT NULL UNIQUE,
    display_name    VARCHAR(255) NOT NULL,
    description     TEXT         NOT NULL,
    -- sink_kind classifies what the sink RECEIVES.
    -- One of: traces / metrics / logs / events / quality_reports / audit.
    sink_kind       VARCHAR(32)  NOT NULL,
    -- protocol identifies the wire protocol.
    -- One of: otlp_http / otlp_grpc / prometheus_remote_write /
    --         loki_push / webhook / kafka / s3.
    protocol        VARCHAR(32)  NOT NULL,
    -- vendor identifies the upstream maintainer (jaeger / honeycomb /
    -- prometheus / loki / grafana / datadog / newrelic / generic).
    vendor          VARCHAR(64)  NOT NULL,
    -- default_endpoint_template is a hint for tenant config — the
    -- runtime substitutes {{tenantId}} or {{region}} placeholders.
    default_endpoint_template TEXT,
    -- requires_auth: must tenant supply credentials before sending?
    requires_auth   BOOLEAN      NOT NULL DEFAULT TRUE,
    -- auth_type: api_key / bearer_token / oauth2 / basic / mtls / none.
    auth_type       VARCHAR(32)  NOT NULL DEFAULT 'none',
    -- documentation_url for vendor-side setup docs.
    documentation_url TEXT,
    -- is_official: vetted by AgentHub (vs community-contributed).
    is_official     BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order      INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ah_core_telemetry_sink_slug      ON ah_core.telemetry_sink (slug);
CREATE INDEX IF NOT EXISTS idx_ah_core_telemetry_sink_kind      ON ah_core.telemetry_sink (sink_kind);
CREATE INDEX IF NOT EXISTS idx_ah_core_telemetry_sink_protocol  ON ah_core.telemetry_sink (protocol);
CREATE INDEX IF NOT EXISTS idx_ah_core_telemetry_sink_is_active ON ah_core.telemetry_sink (is_active);

-- ============================
-- TRACES (3) — OpenTelemetry
-- ============================
INSERT INTO ah_core.telemetry_sink
    (slug, display_name, description, sink_kind, protocol, vendor,
     default_endpoint_template, requires_auth, auth_type,
     documentation_url, is_official, sort_order) VALUES

('otel-collector-traces',
 'OpenTelemetry Collector (Traces)',
 'Generic OTLP HTTP traces sink — works with any OTel collector deployment.',
 'traces', 'otlp_http', 'opentelemetry',
 'http://otel-collector:4318/v1/traces',
 FALSE, 'none',
 'https://opentelemetry.io/docs/collector/', TRUE, 10),

('jaeger-traces',
 'Jaeger Traces',
 'Jaeger native trace ingestion via OTLP HTTP.',
 'traces', 'otlp_http', 'jaeger',
 'http://jaeger:14268/api/traces',
 FALSE, 'none',
 'https://www.jaegertracing.io/docs/', TRUE, 20),

('honeycomb-traces',
 'Honeycomb Traces',
 'Honeycomb cloud trace ingestion via OTLP gRPC.',
 'traces', 'otlp_grpc', 'honeycomb',
 'api.honeycomb.io:443',
 TRUE, 'api_key',
 'https://docs.honeycomb.io/getting-data-in/opentelemetry/', TRUE, 30);

-- ============================
-- METRICS (2)
-- ============================
INSERT INTO ah_core.telemetry_sink
    (slug, display_name, description, sink_kind, protocol, vendor,
     default_endpoint_template, requires_auth, auth_type,
     documentation_url, is_official, sort_order) VALUES

('prometheus-remote-write',
 'Prometheus Remote Write',
 'Push metrics to a Prometheus-compatible backend (Mimir, Cortex, Thanos, Grafana Cloud).',
 'metrics', 'prometheus_remote_write', 'prometheus',
 'http://prometheus:9090/api/v1/write',
 FALSE, 'none',
 'https://prometheus.io/docs/specs/prw/', TRUE, 40),

('grafana-cloud-metrics',
 'Grafana Cloud Metrics',
 'Grafana Cloud metrics ingestion via Prometheus remote write.',
 'metrics', 'prometheus_remote_write', 'grafana',
 'https://prometheus-prod-{{region}}.grafana.net/api/prom/push',
 TRUE, 'basic',
 'https://grafana.com/docs/grafana-cloud/send-data/metrics/', TRUE, 50);

-- ============================
-- LOGS (1)
-- ============================
INSERT INTO ah_core.telemetry_sink
    (slug, display_name, description, sink_kind, protocol, vendor,
     default_endpoint_template, requires_auth, auth_type,
     documentation_url, is_official, sort_order) VALUES

('loki-push',
 'Grafana Loki',
 'Push logs to Loki (self-hosted or Grafana Cloud) via /loki/api/v1/push.',
 'logs', 'loki_push', 'grafana',
 'http://loki:3100/loki/api/v1/push',
 FALSE, 'none',
 'https://grafana.com/docs/loki/latest/reference/api/', TRUE, 60);

-- ============================
-- EVENTS / QUALITY REPORTS / AUDIT (3) — generic webhooks
-- ============================
INSERT INTO ah_core.telemetry_sink
    (slug, display_name, description, sink_kind, protocol, vendor,
     default_endpoint_template, requires_auth, auth_type,
     documentation_url, is_official, sort_order) VALUES

('webhook-events',
 'Generic Events Webhook',
 'POST OBS-001 RunEvents to a tenant-configured webhook URL (JSON body per event).',
 'events', 'webhook', 'generic',
 NULL,
 TRUE, 'bearer_token',
 NULL, TRUE, 70),

('webhook-quality-reports',
 'Quality Report Webhook',
 'POST OBS-009 QualityReports to a tenant-configured webhook URL on every evaluator run.',
 'quality_reports', 'webhook', 'generic',
 NULL,
 TRUE, 'bearer_token',
 NULL, TRUE, 80),

('webhook-audit',
 'Audit Log Webhook',
 'POST GOV-001 audit entries + GOV-005 governance reports to a tenant compliance system.',
 'audit', 'webhook', 'generic',
 NULL,
 TRUE, 'bearer_token',
 NULL, TRUE, 90);
