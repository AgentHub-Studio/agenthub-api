package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreQueryPipelineStepTemplate is a platform-managed preset for one of the nine §4.1 pipeline steps.
type CoreQueryPipelineStepTemplate struct {
	ID             uuid.UUID
	Slug           string
	Label          string
	Description    string
	StepOrder      int    // 1–9, canonical execution order per §4.1
	Phase          string // setup|context|reasoning|execution|termination
	CanBlock       bool
	IsRetryable    bool
	IsPerIteration bool
	SortOrder      int
}

// SeedExpectedQueryPipelineStepSlugs is the canonical closed set from §4.1.
var SeedExpectedQueryPipelineStepSlugs = []string{
	"settings_resolution",
	"mutable_state_init",
	"context_assembly",
	"pre_model_shapers",
	"model_call",
	"tool_use_dispatch",
	"permission_gate",
	"tool_execution",
	"stop_condition",
}

// SeedExpectedQueryPipelineStepRowCount matches the migration INSERT count.
const SeedExpectedQueryPipelineStepRowCount = 9

// SeedQueryPipelinePhases is the closed set of phase values in execution order.
var SeedQueryPipelinePhases = []string{
	"setup", "context", "reasoning", "execution", "termination",
}

// SeedQueryPipelineBlockingStepSlugs are the steps that can halt the pipeline.
var SeedQueryPipelineBlockingStepSlugs = []string{
	"pre_model_shapers", "model_call", "permission_gate", "stop_condition",
}

// SeedQueryPipelineRetryableStepSlugs are the steps the recovery mechanism may re-attempt.
var SeedQueryPipelineRetryableStepSlugs = []string{
	"model_call", "tool_execution",
}

// CoreQueryPipelineStepDefaultTemplateLoader loads pipeline step presets from ah_core.
type CoreQueryPipelineStepDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreQueryPipelineStepDefaultTemplateLoader creates a loader.
func NewCoreQueryPipelineStepDefaultTemplateLoader(pool *pgxpool.Pool) *CoreQueryPipelineStepDefaultTemplateLoader {
	return &CoreQueryPipelineStepDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all pipeline step templates ordered by step_order.
func (l *CoreQueryPipelineStepDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreQueryPipelineStepTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, description,
		       step_order, phase, can_block, is_retryable, is_per_iteration, sort_order
		  FROM ah_core.query_pipeline_step_template
		 ORDER BY step_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.query_pipeline_step_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query query_pipeline_step_template: %w", err)
	}
	defer rows.Close()

	var steps []CoreQueryPipelineStepTemplate
	for rows.Next() {
		var t CoreQueryPipelineStepTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.StepOrder, &t.Phase, &t.CanBlock, &t.IsRetryable, &t.IsPerIteration, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan query_pipeline_step_template: %w", err)
		}
		steps = append(steps, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate query_pipeline_step_template: %w", err)
	}
	return steps, nil
}

// FindBySlug returns one step template by slug.
func (l *CoreQueryPipelineStepDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreQueryPipelineStepTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreQueryPipelineStepTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreQueryPipelineStepTemplate{}, false, nil
}

// LoadBlockingSteps returns only the steps that can halt the pipeline.
func (l *CoreQueryPipelineStepDefaultTemplateLoader) LoadBlockingSteps(ctx context.Context) ([]CoreQueryPipelineStepTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreQueryPipelineStepTemplate
	for _, t := range all {
		if t.CanBlock {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRetryableSteps returns only the steps the recovery mechanism may re-attempt.
func (l *CoreQueryPipelineStepDefaultTemplateLoader) LoadRetryableSteps(ctx context.Context) ([]CoreQueryPipelineStepTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreQueryPipelineStepTemplate
	for _, t := range all {
		if t.IsRetryable {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadStepsInPhase returns all steps belonging to the given phase, in step_order.
func (l *CoreQueryPipelineStepDefaultTemplateLoader) LoadStepsInPhase(ctx context.Context, phase string) ([]CoreQueryPipelineStepTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreQueryPipelineStepTemplate
	for _, t := range all {
		if t.Phase == phase {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
