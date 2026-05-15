package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreToolResultBudgetDefaultTemplate is a platform-managed blueprint
// paired with CTX-008 ToolResultBudgetEnforcer. Each row encodes a
// (per-call, per-turn, per-run, force-summarize-over) profile.
type CoreToolResultBudgetDefaultTemplate struct {
	ID                        uuid.UUID
	Slug                      string
	Name                      string
	Description               string
	PerCallMaxTokens          int
	PerTurnMaxTokens          int
	PerRunMaxTokens           int
	ForceSummarizeOverTokens  int
	TargetUseCase             string
	RecommendedForModelFamily string
	RequiresAdminReview       bool
	IsRecommended             bool
	IsActive                  bool
	SortOrder                 int
}

// CoreToolResultBudgetDefaultTemplateLoader loads tool-result budget templates.
type CoreToolResultBudgetDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreToolResultBudgetDefaultTemplateLoader creates the loader.
func NewCoreToolResultBudgetDefaultTemplateLoader(pool *pgxpool.Pool) *CoreToolResultBudgetDefaultTemplateLoader {
	return &CoreToolResultBudgetDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreToolResultBudgetDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreToolResultBudgetDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description,
		       per_call_max_tokens, per_turn_max_tokens, per_run_max_tokens,
		       force_summarize_over_tokens,
		       target_use_case, recommended_for_model_family,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.tool_result_budget_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.tool_result_budget_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query tool_result_budget_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreToolResultBudgetDefaultTemplate
	for rows.Next() {
		var t CoreToolResultBudgetDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description,
			&t.PerCallMaxTokens, &t.PerTurnMaxTokens, &t.PerRunMaxTokens,
			&t.ForceSummarizeOverTokens,
			&t.TargetUseCase, &t.RecommendedForModelFamily,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan tool_result_budget_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate tool_result_budget_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreToolResultBudgetDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreToolResultBudgetDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreToolResultBudgetDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreToolResultBudgetDefaultTemplate{}, false, nil
}

// LoadByUseCase filters by use case.
func (l *CoreToolResultBudgetDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CoreToolResultBudgetDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreToolResultBudgetDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreToolResultBudgetDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreToolResultBudgetDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreToolResultBudgetDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedTRBDTemplateSlugs is the closed canonical set.
var SeedExpectedTRBDTemplateSlugs = []string{
	"balanced-default",
	"small-context-tight",
	"research-heavy",
	"code-heavy",
	"cost-strict",
	"dev-debug",
}

// SeedExpectedTRBDTemplateUseCases is the closed use-case set.
var SeedExpectedTRBDTemplateUseCases = []string{
	"general", "research", "code",
}

// SeedExpectedTRBDTemplateModelFamilies is the closed model-family set.
var SeedExpectedTRBDTemplateModelFamilies = []string{
	"mid_tier", "small_local", "large_context", "dev_local",
}

// SeedRecommendedTRBDTemplateSlugs is the safe one-click subset
// (excludes dev-debug — production-unsafe).
var SeedRecommendedTRBDTemplateSlugs = []string{
	"balanced-default",
	"small-context-tight",
	"research-heavy",
	"code-heavy",
	"cost-strict",
}

// SeedAdminReviewTRBDTemplateSlugs lists templates requiring admin review.
var SeedAdminReviewTRBDTemplateSlugs = []string{
	"cost-strict",
}

// SeedExpectedTRBDTemplateRowCount = 6.
const SeedExpectedTRBDTemplateRowCount = 6
