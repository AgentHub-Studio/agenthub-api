package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityWebhookTemplateLoader loads capability-specific webhook
// notification templates from ah_core.webhook_notification_template.
// These 3 templates (sort_order 100-102) represent notification events fired
// by the three capability agents introduced in migration 000091:
//
//   - capability-research-complete — fires when core-researcher finishes a task
//   - capability-analysis-done     — fires when core-analyst completes analysis
//   - capability-tasks-updated     — fires when core-planner updates task list
//
// Seeded by migration 000097. The table ah_core.webhook_notification_template is
// also created by that migration. Distinct from ah_core.webhook_endpoint_template
// (migration 000021) which catalogs outbound delivery destinations; this table
// catalogs the events themselves.
//
// Non-fatal when the ah_core schema or webhook_notification_template table is
// missing — supports fresh deployments where migration 000097 has not yet run.
type CoreCapabilityWebhookTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityWebhookTemplateLoader creates a CoreCapabilityWebhookTemplateLoader
// backed by pool.
func NewCoreCapabilityWebhookTemplateLoader(pool *pgxpool.Pool) *CoreCapabilityWebhookTemplateLoader {
	return &CoreCapabilityWebhookTemplateLoader{pool: pool}
}

// CoreWebhookNotificationTemplate is a platform-managed webhook notification
// event template. Describes what fires and the expected payload shape.
type CoreWebhookNotificationTemplate struct {
	ID            uuid.UUID
	Slug          string
	Name          string
	Description   string
	EventType     string
	PayloadSchema string // raw JSONB string
	IsRecommended bool
	IsActive      bool
	SortOrder     int
}

// LoadCapabilityWebhookTemplates returns all active capability webhook
// notification templates from ah_core.webhook_notification_template
// WHERE slug = ANY($1) AND is_active = true, ordered by sort_order.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityWebhookTemplateLoader) LoadCapabilityWebhookTemplates(ctx context.Context) ([]CoreWebhookNotificationTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, event_type,
		       payload_schema::text,
		       is_recommended, is_active, sort_order
		  FROM ah_core.webhook_notification_template
		 WHERE slug = ANY($1)
		   AND is_active = true
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityWebhookTemplateSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.webhook_notification_template not accessible, capability webhook templates unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability webhook templates: %w", err)
	}
	defer rows.Close()

	var templates []CoreWebhookNotificationTemplate
	for rows.Next() {
		var t CoreWebhookNotificationTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.EventType,
			&t.PayloadSchema,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability webhook template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.webhook_notification_template not accessible (post-iter), capability webhook templates unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability webhook templates: %w", err)
	}
	return templates, nil
}

// FindBySlug returns a single capability webhook template by slug.
// Returns (CoreWebhookNotificationTemplate{}, false, nil) when not found.
func (l *CoreCapabilityWebhookTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreWebhookNotificationTemplate, bool, error) {
	all, err := l.LoadCapabilityWebhookTemplates(ctx)
	if err != nil {
		return CoreWebhookNotificationTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreWebhookNotificationTemplate{}, false, nil
}

// ============================================================
// Seed catalog constants — migration 000097 (2026-05-11).
// ============================================================

// SeedCapabilityWebhookTemplateSlugs is the canonical closed set of capability
// webhook notification template slugs seeded in migration 000097. One template
// per capability agent: research-complete (researcher), analysis-done (analyst),
// tasks-updated (planner).
var SeedCapabilityWebhookTemplateSlugs = []string{
	"capability-research-complete",
	"capability-analysis-done",
	"capability-tasks-updated",
}

// SeedCapabilityWebhookTemplateCount is the expected row count after migration 000097.
const SeedCapabilityWebhookTemplateCount = 3

// SeedCapabilityWebhookTemplateEvents is the canonical closed set of event_type
// values seeded in migration 000097. Each event type corresponds to a capability
// agent completion or state-change event.
var SeedCapabilityWebhookTemplateEvents = []string{
	"research_complete",
	"analysis_done",
	"tasks_updated",
}

// SeedResearchCompleteWebhookSlug is the webhook notification template slug for
// the core-researcher capability agent. Fires when a research task completes.
const SeedResearchCompleteWebhookSlug = "capability-research-complete"

// SeedAnalysisDoneWebhookSlug is the webhook notification template slug for
// the core-analyst capability agent. Fires when document analysis completes.
const SeedAnalysisDoneWebhookSlug = "capability-analysis-done"

// SeedTasksUpdatedWebhookSlug is the webhook notification template slug for
// the core-planner capability agent. Fires when the task list is updated.
const SeedTasksUpdatedWebhookSlug = "capability-tasks-updated"
