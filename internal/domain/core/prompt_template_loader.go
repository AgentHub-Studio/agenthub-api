package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePromptTemplate represents a platform-managed prompt template
// catalog entry. Tenants opt-in by creating an agent from a template;
// the system_prompt becomes the agent's initial prompt.
type CorePromptTemplate struct {
	ID                     uuid.UUID
	Slug                   string
	DisplayName            string
	Description            string
	TemplateKind           string // assistant/coder/analyst/researcher/writer/translator/customer_support/data_extractor
	SystemPrompt           string
	RecommendedTemperature float64
	RecommendedMaxTokens   int
	RequiresTools          string // comma-separated skill slugs
	Placeholders           string // comma-separated {{...}} names
	IsRecommended          bool
	IsActive               bool
	SortOrder              int
}

// RequiresToolsList returns parsed tool slugs.
func (t CorePromptTemplate) RequiresToolsList() []string {
	return parseCommaList(t.RequiresTools)
}

// PlaceholdersList returns parsed placeholder names.
func (t CorePromptTemplate) PlaceholdersList() []string {
	return parseCommaList(t.Placeholders)
}

// CorePromptTemplateLoader loads platform prompt templates.
type CorePromptTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePromptTemplateLoader creates a CorePromptTemplateLoader.
func NewCorePromptTemplateLoader(pool *pgxpool.Pool) *CorePromptTemplateLoader {
	return &CorePromptTemplateLoader{pool: pool}
}

// LoadAll returns all active templates.
func (l *CorePromptTemplateLoader) LoadAll(ctx context.Context) ([]CorePromptTemplate, error) {
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
		  FROM ah_core.core_prompt_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.core_prompt_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query prompt_template: %w", err)
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
			return nil, fmt.Errorf("core: scan prompt_template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate prompt_template: %w", err)
	}
	return templates, nil
}

// FindBySlug returns one template by slug.
func (l *CorePromptTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePromptTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
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

// LoadByKind returns active templates for a kind.
func (l *CorePromptTemplateLoader) LoadByKind(ctx context.Context, kind string) ([]CorePromptTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePromptTemplate
	for _, t := range all {
		if t.TemplateKind == kind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns recommended templates.
func (l *CorePromptTemplateLoader) LoadRecommended(ctx context.Context) ([]CorePromptTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePromptTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// RenderSystemPrompt substitutes placeholders into the system prompt.
// Replaces {{key}} with values[key]. Missing keys remain literal.
func (t CorePromptTemplate) RenderSystemPrompt(values map[string]string) string {
	out := t.SystemPrompt
	for k, v := range values {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

// SeedExpectedPromptTemplateSlugs is the canonical list.
var SeedExpectedPromptTemplateSlugs = []string{
	"general-assistant",
	"code-reviewer",
	"data-analyst",
	"researcher",
	"technical-writer",
	"translator",
	"customer-support",
	"data-extractor",
}

// SeedExpectedPromptTemplateKinds is the closed set.
var SeedExpectedPromptTemplateKinds = []string{
	"assistant", "coder", "analyst", "researcher",
	"writer", "translator", "customer_support", "data_extractor",
}

// SeedRecommendedPromptTemplateSlugs is the curated subset.
var SeedRecommendedPromptTemplateSlugs = []string{
	"general-assistant",
	"code-reviewer",
	"researcher",
	"customer-support",
}
