package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreNotificationRouteTemplate represents a platform-managed route
// connecting TRIGGER (event type + severity filter) → SINK (webhook template).
type CoreNotificationRouteTemplate struct {
	ID                          uuid.UUID
	Slug                        string
	DisplayName                 string
	Description                 string
	TriggerEvent                string
	MinSeverity                 string // info/warn/critical or empty
	TargetWebhookTemplateSlug   string // app-level FK to ah_core.webhook_endpoint_template
	AggregationWindowSeconds    int
	MaxPerHour                  int
	RequiresDedup               bool
	IsRecommended               bool
	IsActive                    bool
	SortOrder                   int
}

// CoreNotificationRouteTemplateLoader loads notification route templates.
type CoreNotificationRouteTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreNotificationRouteTemplateLoader creates loader.
func NewCoreNotificationRouteTemplateLoader(pool *pgxpool.Pool) *CoreNotificationRouteTemplateLoader {
	return &CoreNotificationRouteTemplateLoader{pool: pool}
}

// LoadAll returns all active templates.
func (l *CoreNotificationRouteTemplateLoader) LoadAll(ctx context.Context) ([]CoreNotificationRouteTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, display_name, description, trigger_event,
		       min_severity, target_webhook_template_slug,
		       aggregation_window_seconds, max_per_hour,
		       requires_dedup, is_recommended, is_active, sort_order
		  FROM ah_core.notification_route_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.notification_route_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query notification_route_template: %w", err)
	}
	defer rows.Close()

	var templates []CoreNotificationRouteTemplate
	for rows.Next() {
		var t CoreNotificationRouteTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.DisplayName, &t.Description, &t.TriggerEvent,
			&t.MinSeverity, &t.TargetWebhookTemplateSlug,
			&t.AggregationWindowSeconds, &t.MaxPerHour,
			&t.RequiresDedup, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan notification_route_template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate notification_route_template: %w", err)
	}
	return templates, nil
}

// FindBySlug returns one template by slug.
func (l *CoreNotificationRouteTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreNotificationRouteTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreNotificationRouteTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreNotificationRouteTemplate{}, false, nil
}

// LoadByTriggerEvent returns templates triggered by a specific event.
func (l *CoreNotificationRouteTemplateLoader) LoadByTriggerEvent(ctx context.Context, event string) ([]CoreNotificationRouteTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreNotificationRouteTemplate
	for _, t := range all {
		if t.TriggerEvent == event {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns recommended templates.
func (l *CoreNotificationRouteTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreNotificationRouteTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreNotificationRouteTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedNotificationRouteTemplateSlugs is the canonical list.
var SeedExpectedNotificationRouteTemplateSlugs = []string{
	"runs-to-slack",
	"quality-fails-to-slack",
	"governance-alerts-to-pagerduty",
	"checkpoint-pending-to-slack",
	"checkpoint-timeout-to-pagerduty",
	"silent-failure-to-pagerduty",
	"audit-events-to-webhook",
	"coherence-drift-to-slack",
}

// SeedExpectedNotificationRouteTriggerEvents is the closed set.
var SeedExpectedNotificationRouteTriggerEvents = []string{
	"run_complete", "quality_report", "governance_alert",
	"checkpoint_pending", "checkpoint_timeout", "silent_failure",
	"audit_event", "coherence_drift",
}

// SeedExpectedNotificationRouteSeverities is the closed set (plus empty).
var SeedExpectedNotificationRouteSeverities = []string{
	"", "info", "warn", "critical",
}

// SeedRecommendedNotificationRouteTemplateSlugs is the curated subset.
var SeedRecommendedNotificationRouteTemplateSlugs = []string{
	"runs-to-slack",
	"quality-fails-to-slack",
	"governance-alerts-to-pagerduty",
	"checkpoint-pending-to-slack",
	"silent-failure-to-pagerduty",
}
