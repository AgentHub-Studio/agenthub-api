package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreMultiAgentCoordinationPlanDefaultTemplate is a platform-managed
// blueprint paired with SUB-011 MultiAgentCoordinator. Each template
// captures a proven strategy × failure_policy × parallelism shape.
type CoreMultiAgentCoordinationPlanDefaultTemplate struct {
	ID                       uuid.UUID
	Slug                     string
	Name                     string
	Description              string
	TargetStrategy           string
	TargetFailurePolicy      string
	TargetUseCase            string
	MaxParallelism           int
	SampleTaskCount          int
	HasDagDependencies       bool
	RecommendedForTenantKind string
	RequiresAdminReview      bool
	IsRecommended            bool
	IsActive                 bool
	SortOrder                int
}

// CoreMultiAgentCoordinationPlanDefaultTemplateLoader loads templates.
type CoreMultiAgentCoordinationPlanDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader creates the loader.
func NewCoreMultiAgentCoordinationPlanDefaultTemplateLoader(pool *pgxpool.Pool) *CoreMultiAgentCoordinationPlanDefaultTemplateLoader {
	return &CoreMultiAgentCoordinationPlanDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreMultiAgentCoordinationPlanDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreMultiAgentCoordinationPlanDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, name, description, target_strategy, target_failure_policy,
		       target_use_case, max_parallelism, sample_task_count, has_dag_dependencies,
		       recommended_for_tenant_kind, requires_admin_review,
		       is_recommended, is_active, sort_order
		  FROM ah_core.multi_agent_coordination_plan_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.multi_agent_coordination_plan_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query multi_agent_coordination_plan_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreMultiAgentCoordinationPlanDefaultTemplate
	for rows.Next() {
		var t CoreMultiAgentCoordinationPlanDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Name, &t.Description, &t.TargetStrategy, &t.TargetFailurePolicy,
			&t.TargetUseCase, &t.MaxParallelism, &t.SampleTaskCount, &t.HasDagDependencies,
			&t.RecommendedForTenantKind, &t.RequiresAdminReview,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan multi_agent_coordination_plan_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate multi_agent_coordination_plan_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreMultiAgentCoordinationPlanDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreMultiAgentCoordinationPlanDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreMultiAgentCoordinationPlanDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreMultiAgentCoordinationPlanDefaultTemplate{}, false, nil
}

// LoadByStrategy filters by SUB-011 MultiAgentStrategy.
func (l *CoreMultiAgentCoordinationPlanDefaultTemplateLoader) LoadByStrategy(ctx context.Context, strategy string) ([]CoreMultiAgentCoordinationPlanDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreMultiAgentCoordinationPlanDefaultTemplate
	for _, t := range all {
		if t.TargetStrategy == strategy {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByFailurePolicy filters by SUB-011 MultiAgentFailurePolicy.
func (l *CoreMultiAgentCoordinationPlanDefaultTemplateLoader) LoadByFailurePolicy(ctx context.Context, policy string) ([]CoreMultiAgentCoordinationPlanDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreMultiAgentCoordinationPlanDefaultTemplate
	for _, t := range all {
		if t.TargetFailurePolicy == policy {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByUseCase filters by use_case.
func (l *CoreMultiAgentCoordinationPlanDefaultTemplateLoader) LoadByUseCase(ctx context.Context, useCase string) ([]CoreMultiAgentCoordinationPlanDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreMultiAgentCoordinationPlanDefaultTemplate
	for _, t := range all {
		if t.TargetUseCase == useCase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended=TRUE.
func (l *CoreMultiAgentCoordinationPlanDefaultTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreMultiAgentCoordinationPlanDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreMultiAgentCoordinationPlanDefaultTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedMACPDTemplateSlugs is the closed canonical set.
var SeedExpectedMACPDTemplateSlugs = []string{
	"sequential-pipeline",
	"parallel-fanout",
	"dag-build-test-deploy",
	"pipeline-extract-summarize",
	"dag-with-skip-downstream",
}

// SeedExpectedMACPDTemplateStrategies matches SUB-011 MultiAgentStrategy
// enum byte-for-byte (4 strategies; dag appears twice — diamond + general).
var SeedExpectedMACPDTemplateStrategies = []string{
	"sequential", "parallel", "pipeline", "dag",
}

// SeedExpectedMACPDTemplateFailurePolicies matches SUB-011
// MultiAgentFailurePolicy enum byte-for-byte.
var SeedExpectedMACPDTemplateFailurePolicies = []string{
	"abort_on_failure", "continue_on_failure", "skip_downstream_on_failure",
}

// SeedExpectedMACPDTemplateUseCases is the closed use-case set.
var SeedExpectedMACPDTemplateUseCases = []string{
	"linear_workflow", "independent_research",
	"cicd_pipeline", "data_pipeline", "complex_orchestration",
}

// SeedExpectedMACPDTemplateTenantKinds is the closed tenant-kind set.
var SeedExpectedMACPDTemplateTenantKinds = []string{"general"}

// SeedRecommendedMACPDTemplateSlugs — all 5 are recommended.
var SeedRecommendedMACPDTemplateSlugs = []string{
	"sequential-pipeline",
	"parallel-fanout",
	"dag-build-test-deploy",
	"pipeline-extract-summarize",
	"dag-with-skip-downstream",
}

// SeedAdminReviewMACPDTemplateSlugs — DAG templates require admin review
// (non-trivial topology + skip_downstream policy implications).
var SeedAdminReviewMACPDTemplateSlugs = []string{
	"dag-build-test-deploy",
	"dag-with-skip-downstream",
}

// SeedExpectedMACPDTemplateRowCount = 5.
const SeedExpectedMACPDTemplateRowCount = 5
