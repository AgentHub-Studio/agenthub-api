package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityPromptTemplateLoader loads capability-specific prompt templates
// from ah_core.prompt_template. These 5 templates (kind="capability") are
// user-facing starters that help users compose requests to the three capability
// agents introduced in migration 000091 (core-researcher, core-analyst,
// core-planner). Seeded by migration 000094.
//
// Distinct from CorePromptTemplateLoader (which loads the full platform baseline
// of 8 generic templates seeded in migration 000019). This loader targets only
// the capability-kind templates and provides sub-group accessors grouped by the
// underlying skill they require (web research, doc analysis, task workflow).
//
// Non-fatal when the ah_core schema or prompt_template table is missing —
// supports fresh deployments where 000019 has not yet run.
type CoreCapabilityPromptTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityPromptTemplateLoader creates a CoreCapabilityPromptTemplateLoader
// backed by pool.
func NewCoreCapabilityPromptTemplateLoader(pool *pgxpool.Pool) *CoreCapabilityPromptTemplateLoader {
	return &CoreCapabilityPromptTemplateLoader{pool: pool}
}

// LoadCapabilityTemplates returns all recommended capability-kind prompt templates
// from ah_core.prompt_template, ordered by slug.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityPromptTemplateLoader) LoadCapabilityTemplates(ctx context.Context) ([]CorePromptTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, display_name, description, template_kind,
		       system_prompt, recommended_temperature, recommended_max_tokens,
		       requires_tools, placeholders,
		       is_recommended, is_active, sort_order
		  FROM ah_core.prompt_template
		 WHERE template_kind = 'capability'
		   AND is_recommended = true
		 ORDER BY slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.prompt_template not accessible, capability templates unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability prompt templates: %w", err)
	}
	defer rows.Close()

	var templates []CorePromptTemplate
	for rows.Next() {
		var t CorePromptTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.DisplayName, &t.Description, &t.TemplateKind,
			&t.SystemPrompt, &t.RecommendedTemperature, &t.RecommendedMaxTokens,
			&t.RequiresTools, &t.Placeholders,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability prompt template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.prompt_template not accessible (post-iter), capability templates unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability prompt templates: %w", err)
	}
	return templates, nil
}

// FindBySlug returns a single capability prompt template by slug.
// Returns (CorePromptTemplate{}, false, nil) when not found.
func (l *CoreCapabilityPromptTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePromptTemplate, bool, error) {
	all, err := l.LoadCapabilityTemplates(ctx)
	if err != nil {
		return CorePromptTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePromptTemplate{}, false, nil
}

// ============================================================
// Seed catalog constants — migration 000094 (2026-05-11).
// ============================================================

// SeedCapabilityPromptTemplateSlugs is the canonical closed set of capability
// prompt template slugs seeded in migration 000094. Each template targets a
// distinct capability: web research (2), doc analysis (2), task workflow (1).
var SeedCapabilityPromptTemplateSlugs = []string{
	"capability-competitive-research",
	"capability-doc-analysis-summary",
	"capability-knowledge-synthesis",
	"capability-task-breakdown",
	"capability-web-research-brief",
}

// SeedCapabilityPromptTemplateCount is the expected row count after migration 000094.
const SeedCapabilityPromptTemplateCount = 5

// SeedCapabilityPromptTemplateKind is the template_kind value used by all 5
// capability templates. Distinct from the platform kinds (assistant, coder, etc.)
// so the capability layer can be queried independently.
const SeedCapabilityPromptTemplateKind = "capability"

// SeedWebResearchPromptSlugs are the two templates that use the core-web-research
// skill: a general brief and a competitive research report.
var SeedWebResearchPromptSlugs = []string{
	"capability-web-research-brief",
	"capability-competitive-research",
}

// SeedDocAnalysisPromptSlugs are the two templates that use the core-doc-analysis
// skill: a document summary and a knowledge base synthesis.
var SeedDocAnalysisPromptSlugs = []string{
	"capability-doc-analysis-summary",
	"capability-knowledge-synthesis",
}

// SeedTaskWorkflowPromptSlugs contains the single template that uses the
// core-task-workflow skill: a goal-to-task breakdown plan.
var SeedTaskWorkflowPromptSlugs = []string{
	"capability-task-breakdown",
}
