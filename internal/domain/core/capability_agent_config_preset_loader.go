package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityAgentConfigPresetLoader loads capability-specific agent config
// presets from ah_core.agent_config_preset. These 3 presets (sort_order 100-102)
// provide recommended LLM configuration profiles for the three capability agents
// introduced in migration 000091:
//
//   - capability-research-config  — Anthropic claude-sonnet-4-6, temperature 0.3, 8192 tokens
//   - capability-analysis-config  — Anthropic claude-sonnet-4-6, temperature 0.1, 16384 tokens
//   - capability-planning-config  — Anthropic claude-sonnet-4-6, temperature 0.5, 4096 tokens
//
// Seeded by migration 000098. The table ah_core.agent_config_preset is also
// created by that migration.
//
// Temperature tuning per agent purpose:
//
//   Research  — moderate (0.3): broad exploration with enough determinism for
//               source attribution; large context (8192) for multi-source summaries.
//   Analysis  — lowest (0.1): consistent, reproducible structured findings;
//               maximum context (16384) for large document corpora.
//   Planning  — higher (0.5): creative task decomposition; lean context (4096)
//               keeps plans concise.
//
// Non-fatal when the ah_core schema or agent_config_preset table is missing —
// supports fresh deployments where migration 000098 has not yet run.
type CoreCapabilityAgentConfigPresetLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityAgentConfigPresetLoader creates a
// CoreCapabilityAgentConfigPresetLoader backed by pool.
func NewCoreCapabilityAgentConfigPresetLoader(pool *pgxpool.Pool) *CoreCapabilityAgentConfigPresetLoader {
	return &CoreCapabilityAgentConfigPresetLoader{pool: pool}
}

// CoreAgentConfigPreset is a platform-managed LLM configuration preset for a
// capability agent. Captures model provider, model identifier, temperature, and
// max token budget.
type CoreAgentConfigPreset struct {
	Slug          string
	Name          string
	Description   string
	ModelProvider string
	ModelID       string
	Temperature   float64
	MaxTokens     int
	IsRecommended bool
	SortOrder     int
}

// LoadCapabilityAgentConfigPresets returns all capability agent config presets
// from ah_core.agent_config_preset WHERE slug = ANY($1), ordered by sort_order.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityAgentConfigPresetLoader) LoadCapabilityAgentConfigPresets(ctx context.Context) ([]CoreAgentConfigPreset, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, name, description, model_provider, model_id,
		       temperature::float8,
		       max_tokens, is_recommended, sort_order
		  FROM ah_core.agent_config_preset
		 WHERE slug = ANY($1)
		 ORDER BY sort_order`

	rows, err := conn.Query(ctx, query, SeedCapabilityAgentConfigPresetSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.agent_config_preset not accessible, capability agent config presets unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability agent config presets: %w", err)
	}
	defer rows.Close()

	var presets []CoreAgentConfigPreset
	for rows.Next() {
		var p CoreAgentConfigPreset
		if err := rows.Scan(
			&p.Slug, &p.Name, &p.Description, &p.ModelProvider, &p.ModelID,
			&p.Temperature,
			&p.MaxTokens, &p.IsRecommended, &p.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability agent config preset: %w", err)
		}
		presets = append(presets, p)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.agent_config_preset not accessible (post-iter), capability agent config presets unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability agent config presets: %w", err)
	}
	return presets, nil
}

// FindBySlug returns a single capability agent config preset by slug.
// Returns (CoreAgentConfigPreset{}, false, nil) when not found.
func (l *CoreCapabilityAgentConfigPresetLoader) FindBySlug(ctx context.Context, slug string) (CoreAgentConfigPreset, bool, error) {
	all, err := l.LoadCapabilityAgentConfigPresets(ctx)
	if err != nil {
		return CoreAgentConfigPreset{}, false, err
	}
	for _, p := range all {
		if p.Slug == slug {
			return p, true, nil
		}
	}
	return CoreAgentConfigPreset{}, false, nil
}

// ============================================================
// Seed catalog constants — migration 000098 (2026-05-11).
// ============================================================

// SeedCapabilityAgentConfigPresetSlugs is the canonical closed set of capability
// agent config preset slugs seeded in migration 000098. One preset per capability
// agent: research-config (researcher), analysis-config (analyst),
// planning-config (planner).
var SeedCapabilityAgentConfigPresetSlugs = []string{
	"capability-research-config",
	"capability-analysis-config",
	"capability-planning-config",
}

// SeedCapabilityAgentConfigPresetCount is the expected row count after migration 000098.
const SeedCapabilityAgentConfigPresetCount = 3

// SeedResearchAgentConfigSlug is the agent config preset slug for the
// core-researcher capability agent. Temperature 0.3, max_tokens 8192.
const SeedResearchAgentConfigSlug = "capability-research-config"

// SeedAnalysisAgentConfigSlug is the agent config preset slug for the
// core-analyst capability agent. Temperature 0.1 (lowest), max_tokens 16384.
const SeedAnalysisAgentConfigSlug = "capability-analysis-config"

// SeedPlanningAgentConfigSlug is the agent config preset slug for the
// core-planner capability agent. Temperature 0.5, max_tokens 4096.
const SeedPlanningAgentConfigSlug = "capability-planning-config"

// SeedCapabilityAgentConfigModelProvider is the LLM provider used for all
// 3 capability agent config presets seeded in migration 000098.
const SeedCapabilityAgentConfigModelProvider = "anthropic"

// SeedCapabilityAgentConfigModelID is the model identifier used for all
// 3 capability agent config presets seeded in migration 000098.
const SeedCapabilityAgentConfigModelID = "claude-sonnet-4-6"

// SeedCapabilityAgentConfigTemperatures is the map of slug → temperature for
// the 3 capability agent config presets. Used for tests and validation.
var SeedCapabilityAgentConfigTemperatures = map[string]float64{
	"capability-research-config": 0.3,
	"capability-analysis-config": 0.1,
	"capability-planning-config": 0.5,
}

// SeedCapabilityAgentConfigMaxTokens is the map of slug → max_tokens for
// the 3 capability agent config presets. Used for tests and validation.
var SeedCapabilityAgentConfigMaxTokens = map[string]int{
	"capability-research-config": 8192,
	"capability-analysis-config": 16384,
	"capability-planning-config": 4096,
}
