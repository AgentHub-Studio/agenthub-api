package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreKnowledgeBaseTemplate represents a platform-managed KB template
// catalog entry. Tenants opt-in by creating a KB from a template; the
// template's defaults become the KB's initial config.
//
// Inspired by PDF arXiv:2604.14228v1 §4 (RAG pipeline) + CLAUDE.md
// (RAG pipeline: extract → chunk → embed → index → search).
type CoreKnowledgeBaseTemplate struct {
	ID                            uuid.UUID
	Slug                          string
	DisplayName                   string
	Description                   string
	TemplateKind                  string // faq / documentation / internal_wiki / chat_history / api_reference / regulatory / customer_support
	DefaultEmbeddingProviderSlug  string // FK to ah_core.embedding_provider.slug (app-level)
	ChunkStrategy                 string // fixed_size / semantic / sentence / paragraph
	ChunkSizeTokens               int
	ChunkOverlapTokens            int
	RecommendedTopK               int
	SupportedDocTypes             string // comma-separated
	RequiresAdminReview           bool
	IsRecommended                 bool
	IsActive                      bool
	SortOrder                     int
}

// SupportedDocTypesList returns the parsed list of doc types.
func (t CoreKnowledgeBaseTemplate) SupportedDocTypesList() []string {
	parts := strings.Split(t.SupportedDocTypes, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// CoreKnowledgeBaseTemplateLoader loads platform KB templates.
// Like other core loaders, non-fatal when schema missing.
type CoreKnowledgeBaseTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreKnowledgeBaseTemplateLoader creates a CoreKnowledgeBaseTemplateLoader.
func NewCoreKnowledgeBaseTemplateLoader(pool *pgxpool.Pool) *CoreKnowledgeBaseTemplateLoader {
	return &CoreKnowledgeBaseTemplateLoader{pool: pool}
}

// LoadAll returns all active templates, ordered by sort_order then slug.
func (l *CoreKnowledgeBaseTemplateLoader) LoadAll(ctx context.Context) ([]CoreKnowledgeBaseTemplate, error) {
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
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.knowledge_base_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query knowledge_base_template: %w", err)
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
			return nil, fmt.Errorf("core: scan knowledge_base_template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate knowledge_base_template: %w", err)
	}
	return templates, nil
}

// FindBySlug returns one template by slug.
func (l *CoreKnowledgeBaseTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreKnowledgeBaseTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
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

// LoadByKind returns active templates for a specific kind.
func (l *CoreKnowledgeBaseTemplateLoader) LoadByKind(ctx context.Context, kind string) ([]CoreKnowledgeBaseTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreKnowledgeBaseTemplate
	for _, t := range all {
		if t.TemplateKind == kind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns templates flagged is_recommended.
func (l *CoreKnowledgeBaseTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreKnowledgeBaseTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreKnowledgeBaseTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedKBTemplateSlugs is the canonical list of slugs.
var SeedExpectedKBTemplateSlugs = []string{
	"faq-template",
	"documentation-template",
	"internal-wiki-template",
	"chat-history-template",
	"api-reference-template",
	"regulatory-template",
	"customer-support-template",
}

// SeedExpectedKBTemplateKinds is the closed set.
var SeedExpectedKBTemplateKinds = []string{
	"faq",
	"documentation",
	"internal_wiki",
	"chat_history",
	"api_reference",
	"regulatory",
	"customer_support",
}

// SeedExpectedKBTemplateChunkStrategies is the closed set.
var SeedExpectedKBTemplateChunkStrategies = []string{
	"fixed_size",
	"semantic",
	"sentence",
	"paragraph",
}

// SeedRecommendedKBTemplateSlugs is the curated subset.
var SeedRecommendedKBTemplateSlugs = []string{
	"faq-template",
	"documentation-template",
	"api-reference-template",
	"customer-support-template",
}

// SeedAdminReviewRequiredKBTemplateSlugs lists templates that need
// tenant admin approval before creation.
var SeedAdminReviewRequiredKBTemplateSlugs = []string{
	"regulatory-template",
}
