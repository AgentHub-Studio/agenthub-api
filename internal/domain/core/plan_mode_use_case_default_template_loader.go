package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePlanModeUseCaseDefaultTemplate is a platform-managed blueprint
// paired with PERM-003a plan-mode evaluator. Each template captures a
// proven human-in-the-loop workflow: which extra tools count as
// read-only during planning, whether rationale is mandatory, max
// planned actions, and auto-approve threshold.
type CorePlanModeUseCaseDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetUseCase            string
	SafetyPosture            string
	ExtraReadOnlyToolsJSON   string
	RequiresRationale        bool
	MaxPlannedActions        int
	AutoApproveThreshold     int
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// ExtraReadOnlyTools parses the JSONB array of tool names that should
// be treated as read-only during plan mode (extends the default
// PERM-003a DefaultPlanModeReadOnlyClassifier).
func (t CorePlanModeUseCaseDefaultTemplate) ExtraReadOnlyTools() ([]string, error) {
	var out []string
	if err := json.Unmarshal([]byte(t.ExtraReadOnlyToolsJSON), &out); err != nil {
		return nil, fmt.Errorf("plan_mode_use_case_template %s: invalid extra_read_only_tools: %w", t.Slug, err)
	}
	return out, nil
}

// CorePlanModeUseCaseDefaultTemplateLoader loads templates.
type CorePlanModeUseCaseDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePlanModeUseCaseDefaultTemplateLoader creates the loader.
func NewCorePlanModeUseCaseDefaultTemplateLoader(pool *pgxpool.Pool) *CorePlanModeUseCaseDefaultTemplateLoader {
	return &CorePlanModeUseCaseDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CorePlanModeUseCaseDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CorePlanModeUseCaseDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_use_case, safety_posture,
		       extra_read_only_tools::text, requires_rationale,
		       max_planned_actions, auto_approve_threshold,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.plan_mode_use_case_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.plan_mode_use_case_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query plan_mode_use_case_default_template: %w", err)
	}
	defer rows.Close()

	var out []CorePlanModeUseCaseDefaultTemplate
	for rows.Next() {
		var t CorePlanModeUseCaseDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetUseCase, &t.SafetyPosture,
			&t.ExtraReadOnlyToolsJSON, &t.RequiresRationale,
			&t.MaxPlannedActions, &t.AutoApproveThreshold,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan plan_mode_use_case_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate plan_mode_use_case_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CorePlanModeUseCaseDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePlanModeUseCaseDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePlanModeUseCaseDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePlanModeUseCaseDefaultTemplate{}, false, nil
}

// LoadByUseCase filters by use_case label.
func (l *CorePlanModeUseCaseDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CorePlanModeUseCaseDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePlanModeUseCaseDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadBySafetyPosture filters by posture.
func (l *CorePlanModeUseCaseDefaultTemplateLoader) LoadBySafetyPosture(ctx context.Context, posture string) ([]CorePlanModeUseCaseDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePlanModeUseCaseDefaultTemplate
	for _, t := range all {
		if t.SafetyPosture == posture {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CorePlanModeUseCaseDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CorePlanModeUseCaseDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePlanModeUseCaseDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedPMUDTemplateSlugs is the closed canonical set.
var SeedExpectedPMUDTemplateSlugs = []string{
	"dry-run-preview",
	"scoped-change-review",
	"destructive-audit",
	"multi-step-refactor",
	"cross-tenant-migration",
}

// SeedExpectedPMUDTemplateUseCases is the closed use-case set.
var SeedExpectedPMUDTemplateUseCases = []string{
	"dry_run", "scoped_change",
	"destructive_audit", "multi_step_refactor", "cross_tenant_migration",
}

// SeedExpectedPMUDTemplateSafetyPostures is the closed posture set.
var SeedExpectedPMUDTemplateSafetyPostures = []string{
	"permissive", "balanced", "strict",
}

// SeedExpectedPMUDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedPMUDTemplateTenantKinds = []string{"general"}

// SeedRecommendedPMUDTemplateSlugs — all 5 are recommended.
var SeedRecommendedPMUDTemplateSlugs = []string{
	"dry-run-preview",
	"scoped-change-review",
	"destructive-audit",
	"multi-step-refactor",
	"cross-tenant-migration",
}

// SeedAdminReviewPMUDTemplateSlugs — destructive_audit, multi_step_refactor,
// cross_tenant_migration require admin review.
var SeedAdminReviewPMUDTemplateSlugs = []string{
	"destructive-audit",
	"multi-step-refactor",
	"cross-tenant-migration",
}

// SeedExpectedPMUDTemplateRowCount = 5.
const SeedExpectedPMUDTemplateRowCount = 5
