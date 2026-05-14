package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreContextCollapserStrategyDefaultTemplate is a platform-managed
// blueprint paired with CTX-012 ContextCollapser. Each row encodes a
// (strategy + consumer_kind) preset tenants pick to drive read-time
// projection.
type CoreContextCollapserStrategyDefaultTemplate struct {
	ID                             uuid.UUID
	Slug                           string
	Name                           string
	Description                    string
	CollapseStrategy               string
	ConsumerKind                   string
	PreserveErrorsAlways           bool
	PreserveCompactSummaryAlways   bool
	TargetUseCase                  string
	RecommendedForTenantKind       string
	RequiresAdminReview            bool
	IsRecommended                  bool
	IsActive                       bool
	SortOrder                      int
}

// CoreContextCollapserStrategyDefaultTemplateLoader loads collapser strategy templates.
type CoreContextCollapserStrategyDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreContextCollapserStrategyDefaultTemplateLoader creates the loader.
func NewCoreContextCollapserStrategyDefaultTemplateLoader(pool *pgxpool.Pool) *CoreContextCollapserStrategyDefaultTemplateLoader {
	return &CoreContextCollapserStrategyDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreContextCollapserStrategyDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreContextCollapserStrategyDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, collapse_strategy, consumer_kind,
		       preserve_errors_always, preserve_compact_summary_always,
		       target_use_case, recommended_for_tenant_kind,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.context_collapser_strategy_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.context_collapser_strategy_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query context_collapser_strategy_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreContextCollapserStrategyDefaultTemplate
	for rows.Next() {
		var t CoreContextCollapserStrategyDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.CollapseStrategy, &t.ConsumerKind,
			&t.PreserveErrorsAlways, &t.PreserveCompactSummaryAlways,
			&t.TargetUseCase, &t.RecommendedForTenantKind,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan context_collapser_strategy_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate context_collapser_strategy_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreContextCollapserStrategyDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreContextCollapserStrategyDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreContextCollapserStrategyDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreContextCollapserStrategyDefaultTemplate{}, false, nil
}

// LoadByStrategy filters by CTX-012 strategy label.
func (l *CoreContextCollapserStrategyDefaultTemplateLoader) LoadByStrategy(ctx context.Context, strategy string) ([]CoreContextCollapserStrategyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreContextCollapserStrategyDefaultTemplate
	for _, t := range all {
		if t.CollapseStrategy == strategy {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByConsumerKind filters by consumer kind.
func (l *CoreContextCollapserStrategyDefaultTemplateLoader) LoadByConsumerKind(ctx context.Context, kind string) ([]CoreContextCollapserStrategyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreContextCollapserStrategyDefaultTemplate
	for _, t := range all {
		if t.ConsumerKind == kind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreContextCollapserStrategyDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreContextCollapserStrategyDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreContextCollapserStrategyDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedCCSDTemplateSlugs is the closed canonical set.
var SeedExpectedCCSDTemplateSlugs = []string{
	"llm-renderer-balanced",
	"replay-debug-verbose",
	"cost-dashboard-aggressive",
	"audit-export-minimal",
	"ui-summary-aggressive",
	"dev-debug-verbose-no-recommendations",
}

// SeedExpectedCCSDTemplateStrategies matches CTX-012 CollapseStrategy enum.
var SeedExpectedCCSDTemplateStrategies = []string{
	"minimal", "balanced", "aggressive",
}

// SeedExpectedCCSDTemplateConsumerKinds is the closed consumer-kind set.
var SeedExpectedCCSDTemplateConsumerKinds = []string{
	"llm_renderer", "debugger", "cost_dashboard",
	"audit_exporter", "ui_summary",
}

// SeedExpectedCCSDTemplateUseCases is the closed use-case set.
var SeedExpectedCCSDTemplateUseCases = []string{
	"agent_runtime", "incident_replay", "analytics",
	"compliance_export", "user_facing_ui",
}

// SeedExpectedCCSDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedCCSDTemplateTenantKinds = []string{
	"general", "regulated", "dev_local",
}

// SeedRecommendedCCSDTemplateSlugs is the safe one-click subset.
var SeedRecommendedCCSDTemplateSlugs = []string{
	"llm-renderer-balanced",
	"replay-debug-verbose",
	"cost-dashboard-aggressive",
	"audit-export-minimal",
	"ui-summary-aggressive",
}

// SeedAdminReviewCCSDTemplateSlugs is the admin-review set.
var SeedAdminReviewCCSDTemplateSlugs = []string{
	"audit-export-minimal",
}

// SeedExpectedCCSDTemplateRowCount = 6.
const SeedExpectedCCSDTemplateRowCount = 6
