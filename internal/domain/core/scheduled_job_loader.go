package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreScheduledJobTemplate represents a platform-managed scheduled job
// template. Pairs with FUTURE-003 BackgroundAgentRun infrastructure.
type CoreScheduledJobTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	DisplayName              string
	Description              string
	JobKind                  string // reporting/monitoring/compliance/maintenance
	CronExpression           string
	TargetWorkflowSlug       string // cross-table FK to ah_core.workflow_template.slug
	Timezone                 string
	EstimatedCostPerRunUSD   float64
	RequiresAdminApproval    bool
	MaxConcurrentRuns        int
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// CoreScheduledJobTemplateLoader loads scheduled job templates.
type CoreScheduledJobTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreScheduledJobTemplateLoader creates loader.
func NewCoreScheduledJobTemplateLoader(pool *pgxpool.Pool) *CoreScheduledJobTemplateLoader {
	return &CoreScheduledJobTemplateLoader{pool: pool}
}

// LoadAll returns all active templates.
func (l *CoreScheduledJobTemplateLoader) LoadAll(ctx context.Context) ([]CoreScheduledJobTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, display_name, description, job_kind,
		       cron_expression, target_workflow_slug, timezone,
		       estimated_cost_per_run_usd, requires_admin_approval,
		       max_concurrent_runs,
		       is_recommended, is_active, sort_order
		  FROM ah_core.scheduled_job_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.scheduled_job_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query scheduled_job_template: %w", err)
	}
	defer rows.Close()

	var templates []CoreScheduledJobTemplate
	for rows.Next() {
		var t CoreScheduledJobTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.DisplayName, &t.Description, &t.JobKind,
			&t.CronExpression, &t.TargetWorkflowSlug, &t.Timezone,
			&t.EstimatedCostPerRunUSD, &t.RequiresAdminApproval,
			&t.MaxConcurrentRuns,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan scheduled_job_template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate scheduled_job_template: %w", err)
	}
	return templates, nil
}

// FindBySlug returns one template by slug.
func (l *CoreScheduledJobTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreScheduledJobTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreScheduledJobTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreScheduledJobTemplate{}, false, nil
}

// LoadByJobKind returns templates for a kind.
func (l *CoreScheduledJobTemplateLoader) LoadByJobKind(ctx context.Context, kind string) ([]CoreScheduledJobTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreScheduledJobTemplate
	for _, t := range all {
		if t.JobKind == kind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns recommended templates.
func (l *CoreScheduledJobTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreScheduledJobTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreScheduledJobTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedScheduledJobTemplateSlugs is the canonical list.
var SeedExpectedScheduledJobTemplateSlugs = []string{
	"daily-morning-briefing",
	"hourly-incident-triage",
	"weekly-coherence-report",
	"weekly-governance-export",
	"nightly-quality-rollup",
	"daily-cost-summary",
	"monthly-decision-audit",
	"every-15min-stalled-run-sweep",
}

// SeedExpectedScheduledJobTemplateKinds is the closed set.
var SeedExpectedScheduledJobTemplateKinds = []string{
	"reporting", "monitoring", "compliance", "maintenance",
}

// SeedRecommendedScheduledJobTemplateSlugs is the curated subset.
var SeedRecommendedScheduledJobTemplateSlugs = []string{
	"daily-morning-briefing",
	"hourly-incident-triage",
	"weekly-coherence-report",
	"nightly-quality-rollup",
	"every-15min-stalled-run-sweep",
}

// SeedAdminApprovalScheduledJobTemplateSlugs lists jobs that need admin
// approval (ongoing cost commitment).
var SeedAdminApprovalScheduledJobTemplateSlugs = []string{
	"weekly-governance-export",
	"monthly-decision-audit",
}
