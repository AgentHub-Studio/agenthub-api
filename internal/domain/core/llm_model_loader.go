package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreLLMModel represents a platform-managed LLM model catalog entry.
// Tenants opt-in by selecting a slug as their agent's model_config.model.
// ah_core stores the catalog; API keys live in tenant settings.
//
// Inspired by PDF arXiv:2604.14228v1 §4 (context window + tool support
// + streaming as first-class agent capabilities); §11.4 (cost-aware
// model selection feeds evaluator dimension).
type CoreLLMModel struct {
	ID                        uuid.UUID
	Slug                      string
	DisplayName               string
	Description               string
	Provider                  string
	ModelName                 string
	ContextWindow             int
	SupportsTools             bool
	SupportsVision            bool
	SupportsStreaming         bool
	DefaultTemperature        float64
	DefaultMaxTokens          int
	CostPer1MInputTokensUSD   float64
	CostPer1MOutputTokensUSD  float64
	IsRecommended             bool
	IsActive                  bool
	SortOrder                 int
}

// CoreLLMModelLoader loads platform-managed LLM model catalog.
// Like other core loaders, non-fatal when schema missing.
type CoreLLMModelLoader struct {
	pool *pgxpool.Pool
}

// NewCoreLLMModelLoader creates a CoreLLMModelLoader.
func NewCoreLLMModelLoader(pool *pgxpool.Pool) *CoreLLMModelLoader {
	return &CoreLLMModelLoader{pool: pool}
}

// LoadAll returns all active models, ordered by sort_order then slug.
func (l *CoreLLMModelLoader) LoadAll(ctx context.Context) ([]CoreLLMModel, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, display_name, description,
		       provider, model_name, context_window,
		       supports_tools, supports_vision, supports_streaming,
		       default_temperature, default_max_tokens,
		       cost_per_1m_input_tokens_usd, cost_per_1m_output_tokens_usd,
		       is_recommended, is_active, sort_order
		  FROM ah_core.llm_model
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.llm_model not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query llm_model: %w", err)
	}
	defer rows.Close()

	var models []CoreLLMModel
	for rows.Next() {
		var m CoreLLMModel
		if err := rows.Scan(
			&m.ID, &m.Slug, &m.DisplayName, &m.Description,
			&m.Provider, &m.ModelName, &m.ContextWindow,
			&m.SupportsTools, &m.SupportsVision, &m.SupportsStreaming,
			&m.DefaultTemperature, &m.DefaultMaxTokens,
			&m.CostPer1MInputTokensUSD, &m.CostPer1MOutputTokensUSD,
			&m.IsRecommended, &m.IsActive, &m.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan llm_model: %w", err)
		}
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate llm_model: %w", err)
	}
	return models, nil
}

// FindBySlug returns one model by slug.
func (l *CoreLLMModelLoader) FindBySlug(ctx context.Context, slug string) (CoreLLMModel, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreLLMModel{}, false, err
	}
	for _, m := range all {
		if m.Slug == slug {
			return m, true, nil
		}
	}
	return CoreLLMModel{}, false, nil
}

// LoadByProvider returns active models for a specific provider.
func (l *CoreLLMModelLoader) LoadByProvider(ctx context.Context, provider string) ([]CoreLLMModel, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreLLMModel
	for _, m := range all {
		if m.Provider == provider {
			matched = append(matched, m)
		}
	}
	return matched, nil
}

// LoadRecommended returns models flagged is_recommended (one per provider tier).
// UI uses this to populate the "Suggested" picker section.
func (l *CoreLLMModelLoader) LoadRecommended(ctx context.Context) ([]CoreLLMModel, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreLLMModel
	for _, m := range all {
		if m.IsRecommended {
			matched = append(matched, m)
		}
	}
	return matched, nil
}

// SeedExpectedLLMModelSlugs is the canonical list of slugs the seed
// migration 000014_seed_llm_models installs.
var SeedExpectedLLMModelSlugs = []string{
	// anthropic (2)
	"anthropic/claude-haiku-4-5",
	"anthropic/claude-sonnet-4-6",
	// openai (2)
	"openai/gpt-4o-mini",
	"openai/gpt-4o",
	// openrouter (2)
	"openrouter/mistralai/mistral-nemo",
	"openrouter/meta-llama/llama-3-8b-instruct",
	// ollama (1)
	"ollama/llama3.2",
}

// SeedExpectedLLMModelProviders is the closed set of providers seeded.
var SeedExpectedLLMModelProviders = []string{
	"anthropic",
	"openai",
	"openrouter",
	"ollama",
}

// SeedRecommendedLLMModelSlugs is the curated "suggested" subset (one per
// provider tier).
var SeedRecommendedLLMModelSlugs = []string{
	"anthropic/claude-haiku-4-5",
	"openai/gpt-4o-mini",
	"openrouter/mistralai/mistral-nemo",
	"ollama/llama3.2",
}
