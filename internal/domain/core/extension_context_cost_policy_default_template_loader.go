package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreExtensionContextCostPolicyDefaultTemplate is a starter cost
// policy paired with EXT-010 ExtensionContextCostPolicy. Each row
// represents one of the 5 ExtensionContextCostCategory bands as a
// proven exemplar.
type CoreExtensionContextCostPolicyDefaultTemplate struct {
	ID                    uuid.UUID
	Slug                  string
	Category              string // matches EXT-010 enum byte-for-byte
	ExtensionKind         string
	StaticHeaderTokens    int
	PerTurnTokens         int
	PerToolCallTokens     int
	MaxBudgetPerSession   int
	Description           string
	ExampleUseCase        string
	IsRecommended         bool
	IsActive              bool
	SortOrder             int
}

// CoreExtensionContextCostPolicyDefaultTemplateLoader loads templates.
type CoreExtensionContextCostPolicyDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreExtensionContextCostPolicyDefaultTemplateLoader creates the loader.
func NewCoreExtensionContextCostPolicyDefaultTemplateLoader(pool *pgxpool.Pool) *CoreExtensionContextCostPolicyDefaultTemplateLoader {
	return &CoreExtensionContextCostPolicyDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by sort_order then slug.
func (l *CoreExtensionContextCostPolicyDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CoreExtensionContextCostPolicyDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, category, extension_kind,
		       static_header_tokens, per_turn_tokens,
		       per_tool_call_tokens, max_budget_per_session,
		       description, example_use_case, is_recommended,
		       is_active, sort_order
		  FROM ah_core.extension_context_cost_policy_default_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.extension_context_cost_policy_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query extension_context_cost_policy_default_template: %w", err)
	}
	defer rows.Close()

	var out []CoreExtensionContextCostPolicyDefaultTemplate
	for rows.Next() {
		var t CoreExtensionContextCostPolicyDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.Category, &t.ExtensionKind,
			&t.StaticHeaderTokens, &t.PerTurnTokens,
			&t.PerToolCallTokens, &t.MaxBudgetPerSession,
			&t.Description, &t.ExampleUseCase, &t.IsRecommended,
			&t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan extension_context_cost_policy_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate extension_context_cost_policy_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CoreExtensionContextCostPolicyDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreExtensionContextCostPolicyDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreExtensionContextCostPolicyDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreExtensionContextCostPolicyDefaultTemplate{}, false, nil
}

// FindByCategory returns the template for an EXT-010 category.
func (l *CoreExtensionContextCostPolicyDefaultTemplateLoader) FindByCategory(ctx context.Context, category string) (CoreExtensionContextCostPolicyDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreExtensionContextCostPolicyDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Category == category {
			return t, true, nil
		}
	}
	return CoreExtensionContextCostPolicyDefaultTemplate{}, false, nil
}

// SeedExpectedECPTemplateSlugs is the closed canonical set.
var SeedExpectedECPTemplateSlugs = []string{
	"micro-readonly-lookup",
	"small-curated-search",
	"medium-rag-bundle",
	"large-coding-suite",
	"heavy-multimodal-vision",
}

// SeedExpectedECPTemplateCategories — matches EXT-010
// ExtensionContextCostCategory enum byte-for-byte (5 bands).
var SeedExpectedECPTemplateCategories = []string{
	"micro", "small", "medium", "large", "heavy",
}

// SeedExpectedECPTemplateExtensionKinds — closed extension-kind set.
var SeedExpectedECPTemplateExtensionKinds = []string{
	"readonly_lookup", "curated_search", "rag_bundle",
	"coding_suite", "multimodal_vision",
}

// SeedRecommendedECPTemplateSlugs — all 5 are recommended.
var SeedRecommendedECPTemplateSlugs = SeedExpectedECPTemplateSlugs

// SeedExpectedECPTemplateRowCount = 5.
const SeedExpectedECPTemplateRowCount = 5

// Band boundaries replicate EXT-010 constants for cross-validation.
const (
	SeedEXT010MicroMaxPerTurn  = 100
	SeedEXT010SmallMaxPerTurn  = 500
	SeedEXT010MediumMaxPerTurn = 2000
	SeedEXT010LargeMaxPerTurn  = 8000
)
