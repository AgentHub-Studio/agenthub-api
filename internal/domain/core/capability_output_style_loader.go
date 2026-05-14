package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityOutputStyleLoader loads capability-specific output styles from
// ah_core.output_style. These 3 styles (sort_order 100-102) are response-formatting
// templates designed for the three capability agents introduced in migration 000091:
//
//   - research-report  — structured web research output (core-researcher)
//   - analysis-brief   — document analysis with evidence base (core-analyst)
//   - task-checklist   — numbered task plan with checkboxes (core-planner)
//
// Seeded by migration 000095. Distinct from the 8 platform styles seeded in
// migration 000011 (sort_order 10-80).
//
// Non-fatal when the ah_core schema or output_style table is missing — supports
// fresh deployments where 000011 has not yet run.
//
// Reuses [CoreOutputStyle] from output_style_loader.go (same package).
type CoreCapabilityOutputStyleLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityOutputStyleLoader creates a CoreCapabilityOutputStyleLoader
// backed by pool.
func NewCoreCapabilityOutputStyleLoader(pool *pgxpool.Pool) *CoreCapabilityOutputStyleLoader {
	return &CoreCapabilityOutputStyleLoader{pool: pool}
}

// LoadCapabilityStyles returns all active capability output styles from
// ah_core.output_style WHERE slug = ANY($1) AND is_active = true,
// ordered by sort_order.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityOutputStyleLoader) LoadCapabilityStyles(ctx context.Context) ([]CoreOutputStyle, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, name, slug,
		       COALESCE(description, '') AS description,
		       prompt_template, output_format, max_words,
		       audience, is_default, is_active, sort_order
		  FROM ah_core.output_style
		 WHERE slug = ANY($1)
		   AND is_active = true
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityOutputStyleSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.output_style not accessible, capability output styles unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability output styles: %w", err)
	}
	defer rows.Close()

	var styles []CoreOutputStyle
	for rows.Next() {
		var s CoreOutputStyle
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Slug, &s.Description,
			&s.PromptTemplate, &s.OutputFormat, &s.MaxWords,
			&s.Audience, &s.IsDefault, &s.IsActive, &s.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability output style: %w", err)
		}
		styles = append(styles, s)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.output_style not accessible (post-iter), capability output styles unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability output styles: %w", err)
	}
	return styles, nil
}

// FindBySlug returns a single capability output style by slug.
// Returns (CoreOutputStyle{}, false, nil) when not found.
func (l *CoreCapabilityOutputStyleLoader) FindBySlug(ctx context.Context, slug string) (CoreOutputStyle, bool, error) {
	all, err := l.LoadCapabilityStyles(ctx)
	if err != nil {
		return CoreOutputStyle{}, false, err
	}
	for _, s := range all {
		if s.Slug == slug {
			return s, true, nil
		}
	}
	return CoreOutputStyle{}, false, nil
}

// ============================================================
// Seed catalog constants — migration 000095 (2026-05-11).
// ============================================================

// SeedCapabilityOutputStyleSlugs is the canonical closed set of capability
// output style slugs seeded in migration 000095. One style per capability agent:
// research-report (researcher), analysis-brief (analyst), task-checklist (planner).
var SeedCapabilityOutputStyleSlugs = []string{
	"research-report",
	"analysis-brief",
	"task-checklist",
}

// SeedCapabilityOutputStyleCount is the expected row count after migration 000095.
const SeedCapabilityOutputStyleCount = 3

// SeedResearchOutputStyleSlug is the output style slug for the core-researcher
// capability agent. Produces structured web research reports with citations.
const SeedResearchOutputStyleSlug = "research-report"

// SeedAnalysisOutputStyleSlug is the output style slug for the core-analyst
// capability agent. Produces structured document analysis briefs.
const SeedAnalysisOutputStyleSlug = "analysis-brief"

// SeedPlannerOutputStyleSlug is the output style slug for the core-planner
// capability agent. Produces numbered task plans with checkboxes.
const SeedPlannerOutputStyleSlug = "task-checklist"

// SeedCapabilityOutputStyleFormat is the output_format value used by all 3
// capability styles. All use markdown — structured sections with headers and lists.
const SeedCapabilityOutputStyleFormat = "markdown"
