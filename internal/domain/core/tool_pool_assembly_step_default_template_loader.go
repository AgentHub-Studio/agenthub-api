package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreToolPoolAssemblyStepTemplate is a platform-managed preset for one of the
// five §6.2 tool pool assembly steps.
type CoreToolPoolAssemblyStepTemplate struct {
	ID               uuid.UUID
	Slug             string
	Label            string
	Description      string
	StepOrder        int  // 1–5, canonical assembly order per §6.2
	IsAlwaysActive   bool // false = skipped when the relevant source is absent
	CanFilterTools   bool // true = step may remove tools from the pool
	AlwaysPrecedesMCP bool // true = step finishes before MCP tools are merged
	SortOrder        int
}

// SeedExpectedToolPoolAssemblyStepSlugs is the canonical closed set from §6.2.
var SeedExpectedToolPoolAssemblyStepSlugs = []string{
	"base_tool_enumeration",
	"mode_filtering",
	"deny_rule_prefiltering",
	"mcp_tool_integration",
	"deduplication",
}

// SeedExpectedToolPoolAssemblyStepRowCount matches the migration INSERT count.
const SeedExpectedToolPoolAssemblyStepRowCount = 5

// SeedToolPoolAssemblyFilteringStepSlugs are steps that may remove tools from the pool.
var SeedToolPoolAssemblyFilteringStepSlugs = []string{
	"mode_filtering",
	"deny_rule_prefiltering",
	"deduplication",
}

// SeedToolPoolAssemblyPreMCPStepSlugs are steps guaranteed to complete before MCP integration.
var SeedToolPoolAssemblyPreMCPStepSlugs = []string{
	"base_tool_enumeration",
	"mode_filtering",
	"deny_rule_prefiltering",
}

// SeedToolPoolAssemblyConditionalStepSlugs are steps not always active.
var SeedToolPoolAssemblyConditionalStepSlugs = []string{
	"mcp_tool_integration",
}

// CoreToolPoolAssemblyStepDefaultTemplateLoader loads assembly step presets from ah_core.
type CoreToolPoolAssemblyStepDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreToolPoolAssemblyStepDefaultTemplateLoader creates a loader.
func NewCoreToolPoolAssemblyStepDefaultTemplateLoader(pool *pgxpool.Pool) *CoreToolPoolAssemblyStepDefaultTemplateLoader {
	return &CoreToolPoolAssemblyStepDefaultTemplateLoader{pool: pool}
}

// LoadAll returns all assembly steps ordered by step_order.
func (l *CoreToolPoolAssemblyStepDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreToolPoolAssemblyStepTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, label, description,
		       step_order, is_always_active, can_filter_tools, always_precedes_mcp, sort_order
		  FROM ah_core.tool_pool_assembly_step_template
		 ORDER BY step_order`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.tool_pool_assembly_step_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query tool_pool_assembly_step_template: %w", err)
	}
	defer rows.Close()

	var steps []CoreToolPoolAssemblyStepTemplate
	for rows.Next() {
		var t CoreToolPoolAssemblyStepTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Label, &t.Description,
			&t.StepOrder, &t.IsAlwaysActive, &t.CanFilterTools, &t.AlwaysPrecedesMCP, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan tool_pool_assembly_step_template: %w", err)
		}
		steps = append(steps, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate tool_pool_assembly_step_template: %w", err)
	}
	return steps, nil
}

// FindBySlug returns one assembly step template by slug.
func (l *CoreToolPoolAssemblyStepDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreToolPoolAssemblyStepTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreToolPoolAssemblyStepTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreToolPoolAssemblyStepTemplate{}, false, nil
}

// LoadFilteringSteps returns steps that may remove tools from the pool, in step_order.
func (l *CoreToolPoolAssemblyStepDefaultTemplateLoader) LoadFilteringSteps(ctx context.Context) ([]CoreToolPoolAssemblyStepTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreToolPoolAssemblyStepTemplate
	for _, t := range all {
		if t.CanFilterTools {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadPreMCPSteps returns steps guaranteed to complete before MCP tool integration.
func (l *CoreToolPoolAssemblyStepDefaultTemplateLoader) LoadPreMCPSteps(ctx context.Context) ([]CoreToolPoolAssemblyStepTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreToolPoolAssemblyStepTemplate
	for _, t := range all {
		if t.AlwaysPrecedesMCP {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadAlwaysActive returns steps that run unconditionally.
func (l *CoreToolPoolAssemblyStepDefaultTemplateLoader) LoadAlwaysActive(ctx context.Context) ([]CoreToolPoolAssemblyStepTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreToolPoolAssemblyStepTemplate
	for _, t := range all {
		if t.IsAlwaysActive {
			matched = append(matched, t)
		}
	}
	return matched, nil
}
