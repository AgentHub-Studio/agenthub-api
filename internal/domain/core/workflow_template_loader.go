package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreWorkflowTemplate represents a platform-managed workflow blueprint.
// Workflows are end-to-end agent + skill + KB bundles for common
// automations (FAQ answer, code review, customer onboarding, etc.).
type CoreWorkflowTemplate struct {
	ID                      uuid.UUID
	Slug                    string
	DisplayName             string
	Description             string
	WorkflowKind            string // qa/extraction/review/onboarding/research/monitoring/compliance
	TargetAgentSlug         string
	RequiresSkills          string // comma-separated
	RequiresKBKind          string
	EstimatedSteps          int
	EstimatedCostUSD        float64
	RequiresHumanCheckpoint bool
	IsRecommended           bool
	IsActive                bool
	SortOrder               int
}

// RequiresSkillsList returns parsed skill slugs.
func (t CoreWorkflowTemplate) RequiresSkillsList() []string {
	return parseCommaList(t.RequiresSkills)
}

// CoreWorkflowTemplateLoader loads workflow templates.
type CoreWorkflowTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreWorkflowTemplateLoader creates a CoreWorkflowTemplateLoader.
func NewCoreWorkflowTemplateLoader(pool *pgxpool.Pool) *CoreWorkflowTemplateLoader {
	return &CoreWorkflowTemplateLoader{pool: pool}
}

// LoadAll returns all active workflow templates.
func (l *CoreWorkflowTemplateLoader) LoadAll(ctx context.Context) ([]CoreWorkflowTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, display_name, description, workflow_kind,
		       target_agent_slug, requires_skills, requires_kb_kind,
		       estimated_steps, estimated_cost_usd,
		       requires_human_checkpoint, is_recommended, is_active, sort_order
		  FROM ah_core.workflow_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.workflow_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query workflow_template: %w", err)
	}
	defer rows.Close()

	var templates []CoreWorkflowTemplate
	for rows.Next() {
		var t CoreWorkflowTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.DisplayName, &t.Description, &t.WorkflowKind,
			&t.TargetAgentSlug, &t.RequiresSkills, &t.RequiresKBKind,
			&t.EstimatedSteps, &t.EstimatedCostUSD,
			&t.RequiresHumanCheckpoint, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan workflow_template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate workflow_template: %w", err)
	}
	return templates, nil
}

// FindBySlug returns one template by slug.
func (l *CoreWorkflowTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreWorkflowTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreWorkflowTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreWorkflowTemplate{}, false, nil
}

// LoadByKind returns templates for a specific workflow kind.
func (l *CoreWorkflowTemplateLoader) LoadByKind(ctx context.Context, kind string) ([]CoreWorkflowTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreWorkflowTemplate
	for _, t := range all {
		if t.WorkflowKind == kind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended.
func (l *CoreWorkflowTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreWorkflowTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreWorkflowTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedWorkflowTemplateSlugs is the canonical list.
var SeedExpectedWorkflowTemplateSlugs = []string{
	"faq-answer",
	"document-summary",
	"code-review",
	"customer-onboarding",
	"invoice-processing",
	"research-brief",
	"incident-triage",
	"compliance-export",
}

// SeedExpectedWorkflowTemplateKinds is the closed set.
var SeedExpectedWorkflowTemplateKinds = []string{
	"qa", "extraction", "review", "onboarding",
	"research", "monitoring", "compliance",
}

// SeedRecommendedWorkflowTemplateSlugs is the curated subset.
var SeedRecommendedWorkflowTemplateSlugs = []string{
	"faq-answer",
	"document-summary",
	"code-review",
	"research-brief",
}

// SeedHumanCheckpointWorkflowTemplateSlugs lists workflows that require
// HUMAN-004 understanding checkpoint before expensive ops.
var SeedHumanCheckpointWorkflowTemplateSlugs = []string{
	"code-review",
	"invoice-processing",
	"incident-triage",
	"compliance-export",
}
