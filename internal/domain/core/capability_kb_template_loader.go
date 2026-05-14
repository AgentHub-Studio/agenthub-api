package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityKBTemplateLoader loads capability-specific knowledge base templates
// from ah_core.knowledge_base_template. These 3 templates (sort_order 100-102) are
// designed for the three capability agents introduced in migration 000091:
//
//   - research-collection-template — web research findings (core-researcher)
//   - analysis-workspace-template  — document analysis workspace (core-analyst)
//   - project-notes-template       — project documentation (core-planner)
//
// Seeded by migration 000096. Distinct from the 7 platform templates seeded in
// migration 000017 (sort_order 10-70). All 3 use new template_kind values
// (research_collection, analysis_workspace, project_notes) that do not overlap
// with the platform kinds.
//
// Non-fatal when the ah_core schema or knowledge_base_template table is missing —
// supports fresh deployments where migration 000017 has not yet run.
//
// Reuses [CoreKnowledgeBaseTemplate] from kb_template_loader.go (same package).
type CoreCapabilityKBTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityKBTemplateLoader creates a CoreCapabilityKBTemplateLoader
// backed by pool.
func NewCoreCapabilityKBTemplateLoader(pool *pgxpool.Pool) *CoreCapabilityKBTemplateLoader {
	return &CoreCapabilityKBTemplateLoader{pool: pool}
}

// LoadCapabilityKBTemplates returns all active capability KB templates from
// ah_core.knowledge_base_template WHERE slug = ANY($1) AND is_active = true,
// ordered by sort_order.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityKBTemplateLoader) LoadCapabilityKBTemplates(ctx context.Context) ([]CoreKnowledgeBaseTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, display_name, description, template_kind,
		       default_embedding_provider_slug,
		       chunk_strategy, chunk_size_tokens, chunk_overlap_tokens,
		       recommended_top_k, supported_doc_types,
		       requires_admin_review, is_recommended, is_active, sort_order
		  FROM ah_core.knowledge_base_template
		 WHERE slug = ANY($1)
		   AND is_active = true
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityKBTemplateSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.knowledge_base_template not accessible, capability KB templates unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability KB templates: %w", err)
	}
	defer rows.Close()

	var templates []CoreKnowledgeBaseTemplate
	for rows.Next() {
		var t CoreKnowledgeBaseTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.DisplayName, &t.Description, &t.TemplateKind,
			&t.DefaultEmbeddingProviderSlug,
			&t.ChunkStrategy, &t.ChunkSizeTokens, &t.ChunkOverlapTokens,
			&t.RecommendedTopK, &t.SupportedDocTypes,
			&t.RequiresAdminReview, &t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability KB template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.knowledge_base_template not accessible (post-iter), capability KB templates unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability KB templates: %w", err)
	}
	return templates, nil
}

// FindBySlug returns a single capability KB template by slug.
// Returns (CoreKnowledgeBaseTemplate{}, false, nil) when not found.
func (l *CoreCapabilityKBTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreKnowledgeBaseTemplate, bool, error) {
	all, err := l.LoadCapabilityKBTemplates(ctx)
	if err != nil {
		return CoreKnowledgeBaseTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreKnowledgeBaseTemplate{}, false, nil
}

// ============================================================
// Seed catalog constants — migration 000096 (2026-05-11).
// ============================================================

// SeedCapabilityKBTemplateSlugs is the canonical closed set of capability
// KB template slugs seeded in migration 000096. One template per capability
// agent: research-collection-template (researcher), analysis-workspace-template
// (analyst), project-notes-template (planner).
var SeedCapabilityKBTemplateSlugs = []string{
	"research-collection-template",
	"analysis-workspace-template",
	"project-notes-template",
}

// SeedCapabilityKBTemplateCount is the expected row count after migration 000096.
const SeedCapabilityKBTemplateCount = 3

// SeedCapabilityKBTemplateKinds is the closed set of new template_kind values
// introduced by migration 000096. These are distinct from the 7 platform kinds
// (faq, documentation, internal_wiki, chat_history, api_reference, regulatory,
// customer_support) seeded in migration 000017.
var SeedCapabilityKBTemplateKinds = []string{
	"research_collection",
	"analysis_workspace",
	"project_notes",
}

// SeedResearchKBTemplateSlug is the KB template slug for the core-researcher
// capability agent. Organizes web research findings and reference materials.
const SeedResearchKBTemplateSlug = "research-collection-template"

// SeedAnalysisKBTemplateSlug is the KB template slug for the core-analyst
// capability agent. Accepts documents for analysis and synthesis.
const SeedAnalysisKBTemplateSlug = "analysis-workspace-template"

// SeedPlannerKBTemplateSlug is the KB template slug for the core-planner
// capability agent. Stores project documentation and decision records.
const SeedPlannerKBTemplateSlug = "project-notes-template"
