package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityAssessmentTemplate is a platform-managed blueprint for
// measuring human capability over time. Paired with FUTURE-006
// architecture (HumanCapabilitySnapshot). Tenants instantiate these to
// record snapshots without inventing definitions.
type CoreCapabilityAssessmentTemplate struct {
	ID                     uuid.UUID
	Slug                   string
	Name                   string
	Description            string
	TargetDimension        string
	Cadence                string
	RecommendedPeriodDays  int
	MinSampleSize          int
	DeltaAlertThreshold    float64
	EvaluatorKind          string
	RequiresAdminReview    bool
	IsRecommended          bool
	IsActive               bool
	SortOrder              int
}

// CoreCapabilityAssessmentTemplateLoader loads capability assessment templates.
type CoreCapabilityAssessmentTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityAssessmentTemplateLoader creates the loader.
func NewCoreCapabilityAssessmentTemplateLoader(pool *pgxpool.Pool) *CoreCapabilityAssessmentTemplateLoader {
	return &CoreCapabilityAssessmentTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreCapabilityAssessmentTemplateLoader) LoadAll(ctx context.Context) ([]CoreCapabilityAssessmentTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_dimension, cadence,
		       recommended_period_days, min_sample_size, delta_alert_threshold,
		       evaluator_kind, requires_admin_review, is_recommended,
		       is_active, sort_order
		  FROM ah_core.capability_assessment_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_assessment_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_assessment_template: %w", err)
	}
	defer rows.Close()

	var out []CoreCapabilityAssessmentTemplate
	for rows.Next() {
		var t CoreCapabilityAssessmentTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetDimension, &t.Cadence,
			&t.RecommendedPeriodDays, &t.MinSampleSize, &t.DeltaAlertThreshold,
			&t.EvaluatorKind, &t.RequiresAdminReview, &t.IsRecommended,
			&t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_assessment_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability_assessment_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns a single template by slug.
func (l *CoreCapabilityAssessmentTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreCapabilityAssessmentTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreCapabilityAssessmentTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreCapabilityAssessmentTemplate{}, false, nil
}

// LoadByDimension returns templates targeting a specific dimension.
func (l *CoreCapabilityAssessmentTemplateLoader) LoadByDimension(ctx context.Context, dimension string) ([]CoreCapabilityAssessmentTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreCapabilityAssessmentTemplate
	for _, t := range all {
		if t.TargetDimension == dimension {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByCadence returns templates by recommended cadence.
func (l *CoreCapabilityAssessmentTemplateLoader) LoadByCadence(ctx context.Context, cadence string) ([]CoreCapabilityAssessmentTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreCapabilityAssessmentTemplate
	for _, t := range all {
		if t.Cadence == cadence {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreCapabilityAssessmentTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreCapabilityAssessmentTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreCapabilityAssessmentTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedCapabilityTemplateSlugs is the closed canonical seed set.
var SeedExpectedCapabilityTemplateSlugs = []string{
	"domain-knowledge-monthly",
	"decision-independence-monthly",
	"task-throughput-weekly",
	"quality-output-monthly",
	"collaboration-quarterly",
	"holistic-capability-quarterly",
	"onboarding-baseline",
	"incident-postmortem",
}

// SeedExpectedCapabilityTemplateDimensions is the closed set of
// target_dimension values used in the seed (5 FUTURE-006 dimensions
// plus the "multi" sentinel for cross-dimensional templates).
var SeedExpectedCapabilityTemplateDimensions = []string{
	"domain_knowledge",
	"decision_independence",
	"task_throughput",
	"quality_output",
	"collaboration",
	"multi",
}

// SeedExpectedCapabilityTemplateCadences is the closed set of cadences.
var SeedExpectedCapabilityTemplateCadences = []string{
	"weekly", "monthly", "quarterly", "one_shot", "event_driven",
}

// SeedExpectedCapabilityTemplateEvaluatorKinds is the closed set of
// evaluator implementations.
var SeedExpectedCapabilityTemplateEvaluatorKinds = []string{
	"rubric_scored", "behavior_log", "metric_aggregate", "peer_review", "incident_review",
}

// SeedRecommendedCapabilityTemplateSlugs is the safe one-click subset.
var SeedRecommendedCapabilityTemplateSlugs = []string{
	"domain-knowledge-monthly",
	"decision-independence-monthly",
	"task-throughput-weekly",
	"quality-output-monthly",
	"holistic-capability-quarterly",
	"onboarding-baseline",
}

// SeedAdminReviewCapabilityTemplateSlugs is the closed set requiring
// admin vetting (peer review, holistic, incident).
var SeedAdminReviewCapabilityTemplateSlugs = []string{
	"collaboration-quarterly",
	"holistic-capability-quarterly",
	"incident-postmortem",
}

// SeedExpectedCapabilityTemplateRowCount = 8.
const SeedExpectedCapabilityTemplateRowCount = 8
