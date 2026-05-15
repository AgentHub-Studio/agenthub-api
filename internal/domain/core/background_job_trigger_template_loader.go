package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreBackgroundJobTriggerTemplate is a platform-managed blueprint
// paired with FUTURE-003 background_run loop. Each template describes
// one trigger configuration tenants can instantiate.
type CoreBackgroundJobTriggerTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TriggerKind              string
	TriggerConfigJSON        string
	TargetWorkflowSlug       string
	MaxRunsPerDay            int
	ClaimTimeoutSeconds      int
	OnFailureAction          string
	MaxConsecutiveFailures   int
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// TriggerConfig parses the JSON config into a generic map. Returns an
// error if JSON is malformed.
func (t CoreBackgroundJobTriggerTemplate) TriggerConfig() (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal([]byte(t.TriggerConfigJSON), &out); err != nil {
		return nil, fmt.Errorf("background_job_trigger_template %s: invalid trigger_config_json: %w", t.Slug, err)
	}
	return out, nil
}

// CoreBackgroundJobTriggerTemplateLoader loads bg-job trigger templates.
type CoreBackgroundJobTriggerTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreBackgroundJobTriggerTemplateLoader creates the loader.
func NewCoreBackgroundJobTriggerTemplateLoader(pool *pgxpool.Pool) *CoreBackgroundJobTriggerTemplateLoader {
	return &CoreBackgroundJobTriggerTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreBackgroundJobTriggerTemplateLoader) LoadAll(ctx context.Context) ([]CoreBackgroundJobTriggerTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, trigger_kind, trigger_config_json,
		       target_workflow_slug, max_runs_per_day, claim_timeout_seconds,
		       on_failure_action, max_consecutive_failures,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.background_job_trigger_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.background_job_trigger_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query background_job_trigger_template: %w", err)
	}
	defer rows.Close()

	var out []CoreBackgroundJobTriggerTemplate
	for rows.Next() {
		var t CoreBackgroundJobTriggerTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TriggerKind, &t.TriggerConfigJSON,
			&t.TargetWorkflowSlug, &t.MaxRunsPerDay, &t.ClaimTimeoutSeconds,
			&t.OnFailureAction, &t.MaxConsecutiveFailures,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan background_job_trigger_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate background_job_trigger_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreBackgroundJobTriggerTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreBackgroundJobTriggerTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreBackgroundJobTriggerTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreBackgroundJobTriggerTemplate{}, false, nil
}

// LoadByTriggerKind filters by FUTURE-003 trigger kind.
func (l *CoreBackgroundJobTriggerTemplateLoader) LoadByTriggerKind(ctx context.Context, kind string) ([]CoreBackgroundJobTriggerTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreBackgroundJobTriggerTemplate
	for _, t := range all {
		if t.TriggerKind == kind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByOnFailureAction filters by failure action.
func (l *CoreBackgroundJobTriggerTemplateLoader) LoadByOnFailureAction(ctx context.Context, action string) ([]CoreBackgroundJobTriggerTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreBackgroundJobTriggerTemplate
	for _, t := range all {
		if t.OnFailureAction == action {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreBackgroundJobTriggerTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreBackgroundJobTriggerTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreBackgroundJobTriggerTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedBgJobTriggerTemplateSlugs is the closed canonical set.
var SeedExpectedBgJobTriggerTemplateSlugs = []string{
	"daily-digest-summary",
	"hourly-health-check",
	"on-kb-document-uploaded",
	"on-new-user-onboarded",
	"on-cost-budget-exceeded",
	"on-error-rate-spike",
	"on-user-inactive-30-days",
	"on-agent-unused-90-days",
}

// SeedExpectedBgJobTriggerTemplateKinds matches FUTURE-003 BackgroundTrigger
// enum byte-for-byte.
var SeedExpectedBgJobTriggerTemplateKinds = []string{
	"schedule_cron", "event_arrived", "threshold_crossed", "absence_timeout",
}

// SeedExpectedBgJobTriggerTemplateOnFailureActions is the closed action set.
var SeedExpectedBgJobTriggerTemplateOnFailureActions = []string{
	"retry_with_backoff", "alert_admin", "quarantine_trigger",
}

// SeedRecommendedBgJobTriggerTemplateSlugs is the safe one-click subset.
// All 8 are recommended because each pairs with a FUTURE-003 documented
// trigger pattern; tenants opt in per template, not per category.
var SeedRecommendedBgJobTriggerTemplateSlugs = []string{
	"daily-digest-summary",
	"hourly-health-check",
	"on-kb-document-uploaded",
	"on-new-user-onboarded",
	"on-cost-budget-exceeded",
	"on-error-rate-spike",
	"on-user-inactive-30-days",
	"on-agent-unused-90-days",
}

// SeedAdminReviewBgJobTriggerTemplateSlugs is the closed admin-review set
// (cost-pause has business impact; agent-archival risks losing tenant work).
var SeedAdminReviewBgJobTriggerTemplateSlugs = []string{
	"on-cost-budget-exceeded",
	"on-agent-unused-90-days",
}

// SeedExpectedBgJobTriggerTemplateRowCount = 8.
const SeedExpectedBgJobTriggerTemplateRowCount = 8
