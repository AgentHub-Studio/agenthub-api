package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreTelemetrySink represents a platform-managed telemetry sink catalog
// entry. Tenants opt-in by enabling a sink and providing endpoint/creds
// in their tenant settings. ah_core stores the catalog descriptors only.
//
// Inspired by PDF arXiv:2604.14228v1 §7 (observability) + §11 (audit) +
// CLAUDE.md (OpenTelemetry + Prometheus + Grafana stack).
type CoreTelemetrySink struct {
	ID                       uuid.UUID
	Slug                     string
	DisplayName              string
	Description              string
	SinkKind                 string // traces / metrics / logs / events / quality_reports / audit
	Protocol                 string // otlp_http / otlp_grpc / prometheus_remote_write / loki_push / webhook / kafka / s3
	Vendor                   string
	DefaultEndpointTemplate  string
	RequiresAuth             bool
	AuthType                 string // api_key / bearer_token / oauth2 / basic / mtls / none
	DocumentationURL         string
	IsOfficial               bool
	IsActive                 bool
	SortOrder                int
}

// CoreTelemetrySinkLoader loads platform-managed sink catalog.
// Like other core loaders, non-fatal when schema missing.
type CoreTelemetrySinkLoader struct {
	pool *pgxpool.Pool
}

// NewCoreTelemetrySinkLoader creates a CoreTelemetrySinkLoader.
func NewCoreTelemetrySinkLoader(pool *pgxpool.Pool) *CoreTelemetrySinkLoader {
	return &CoreTelemetrySinkLoader{pool: pool}
}

// LoadAll returns all active sinks, ordered by sort_order then slug.
func (l *CoreTelemetrySinkLoader) LoadAll(ctx context.Context) ([]CoreTelemetrySink, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, display_name, description,
		       sink_kind, protocol, vendor,
		       COALESCE(default_endpoint_template, '') AS default_endpoint_template,
		       requires_auth, auth_type,
		       COALESCE(documentation_url, '') AS documentation_url,
		       is_official, is_active, sort_order
		  FROM ah_core.telemetry_sink
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.telemetry_sink not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query telemetry_sink: %w", err)
	}
	defer rows.Close()

	var sinks []CoreTelemetrySink
	for rows.Next() {
		var s CoreTelemetrySink
		if err := rows.Scan(
			&s.ID, &s.Slug, &s.DisplayName, &s.Description,
			&s.SinkKind, &s.Protocol, &s.Vendor,
			&s.DefaultEndpointTemplate,
			&s.RequiresAuth, &s.AuthType, &s.DocumentationURL,
			&s.IsOfficial, &s.IsActive, &s.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan telemetry_sink: %w", err)
		}
		sinks = append(sinks, s)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate telemetry_sink: %w", err)
	}
	return sinks, nil
}

// FindBySlug returns one sink by slug.
func (l *CoreTelemetrySinkLoader) FindBySlug(ctx context.Context, slug string) (CoreTelemetrySink, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreTelemetrySink{}, false, err
	}
	for _, s := range all {
		if s.Slug == slug {
			return s, true, nil
		}
	}
	return CoreTelemetrySink{}, false, nil
}

// LoadByKind returns active sinks for a specific sink_kind.
func (l *CoreTelemetrySinkLoader) LoadByKind(ctx context.Context, kind string) ([]CoreTelemetrySink, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreTelemetrySink
	for _, s := range all {
		if s.SinkKind == kind {
			matched = append(matched, s)
		}
	}
	return matched, nil
}

// SeedExpectedTelemetrySinkSlugs is the canonical list of slugs the seed
// migration 000016_seed_telemetry_sinks installs.
var SeedExpectedTelemetrySinkSlugs = []string{
	// traces (3)
	"otel-collector-traces",
	"jaeger-traces",
	"honeycomb-traces",
	// metrics (2)
	"prometheus-remote-write",
	"grafana-cloud-metrics",
	// logs (1)
	"loki-push",
	// webhooks (3)
	"webhook-events",
	"webhook-quality-reports",
	"webhook-audit",
}

// SeedExpectedTelemetrySinkKinds is the closed set of sink_kind values.
var SeedExpectedTelemetrySinkKinds = []string{
	"traces",
	"metrics",
	"logs",
	"events",
	"quality_reports",
	"audit",
}

// SeedExpectedTelemetrySinkProtocols is the closed set of protocols seeded.
var SeedExpectedTelemetrySinkProtocols = []string{
	"otlp_http",
	"otlp_grpc",
	"prometheus_remote_write",
	"loki_push",
	"webhook",
}

// SeedExpectedTelemetrySinkAuthTypes is the closed set of auth types.
var SeedExpectedTelemetrySinkAuthTypes = []string{
	"none",
	"api_key",
	"bearer_token",
	"basic",
	"oauth2",
	"mtls",
}
