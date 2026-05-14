package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityModelConfig is a single per-agent LLM model configuration row
// loaded from ah_core.capability_model_config. Each row encodes one config key
// (e.g. "default_model", "temperature", "max_tokens") and its value for a
// specific core capability agent. These act as sane defaults the orchestrator
// uses when no tenant override is configured.
type CoreCapabilityModelConfig struct {
	ID          int64
	AgentSlug   string
	ConfigKey   string
	ConfigValue string
	Description string
	CreatedAt   time.Time
}

// ============================================================
// Seed catalog constants — migration 000120 (2026-05-11).
// ============================================================

// SeedModelConfigCount is the total number of capability_model_config rows
// seeded by migration 000120: 9 rows (3 config keys × 3 agents).
const SeedModelConfigCount = 9

// SeedModelConfigAgentCount is the number of capability agents that have
// seeded model configs in migration 000120 (researcher, analyst, planner).
const SeedModelConfigAgentCount = 3

// Config key constants for capability_model_config.config_key values.
const (
	// SeedModelConfigKeyModel is the config key for the default LLM model ID.
	SeedModelConfigKeyModel = "default_model"
	// SeedModelConfigKeyTemperature is the config key for the sampling temperature.
	SeedModelConfigKeyTemperature = "temperature"
	// SeedModelConfigKeyMaxTokens is the config key for the maximum output token budget.
	SeedModelConfigKeyMaxTokens = "max_tokens"
)

// Per-agent default model ID constants.
const (
	// SeedResearcherModel is the default model for core-researcher:
	// fast and cost-effective for search-heavy workloads.
	SeedResearcherModel = "claude-haiku-4-5-20251001"
	// SeedAnalystModel is the default model for core-analyst:
	// balanced capability for complex analysis tasks.
	SeedAnalystModel = "claude-sonnet-4-6"
	// SeedPlannerModel is the default model for core-planner:
	// capable reasoning for complex multi-step planning.
	SeedPlannerModel = "claude-sonnet-4-6"
)

// Per-agent temperature string constants (as seeded in config_value).
const (
	// SeedResearcherTemperature is the temperature seeded for core-researcher (0.3 — factual accuracy).
	SeedResearcherTemperature = "0.3"
	// SeedAnalystTemperature is the temperature seeded for core-analyst (0.2 — deterministic analysis).
	SeedAnalystTemperature = "0.2"
	// SeedPlannerTemperature is the temperature seeded for core-planner (0.5 — creative planning).
	SeedPlannerTemperature = "0.5"
)

// CoreCapabilityModelConfigLoader loads per-agent default model configuration
// rows from ah_core.capability_model_config (seeded by migration 000120).
// All methods are non-fatal when the ah_core schema or the
// capability_model_config table is not yet accessible — supports fresh
// deployments where migration 000120 has not yet run.
type CoreCapabilityModelConfigLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityModelConfigLoader creates a CoreCapabilityModelConfigLoader
// backed by pool.
func NewCoreCapabilityModelConfigLoader(pool *pgxpool.Pool) *CoreCapabilityModelConfigLoader {
	return &CoreCapabilityModelConfigLoader{pool: pool}
}

// LoadCapabilityModelConfigs returns all rows from ah_core.capability_model_config
// ordered by agent_slug, config_key. Returns nil, nil when the table is not
// accessible (non-fatal).
func (l *CoreCapabilityModelConfigLoader) LoadCapabilityModelConfigs(ctx context.Context) ([]CoreCapabilityModelConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, config_key, config_value, description, created_at
		  FROM ah_core.capability_model_config
		 ORDER BY agent_slug, config_key`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_model_config not accessible, model configs unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_model_config: %w", err)
	}
	defer rows.Close()

	var configs []CoreCapabilityModelConfig
	for rows.Next() {
		var c CoreCapabilityModelConfig
		if err := rows.Scan(
			&c.ID, &c.AgentSlug, &c.ConfigKey, &c.ConfigValue, &c.Description, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_model_config: %w", err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_model_config not accessible (post-iter), model configs unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability_model_config: %w", err)
	}
	return configs, nil
}

// LoadModelConfigsForAgent returns all config rows for the given agentSlug
// from ah_core.capability_model_config, ordered by config_key.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityModelConfigLoader) LoadModelConfigsForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityModelConfig, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, config_key, config_value, description, created_at
		  FROM ah_core.capability_model_config
		 WHERE agent_slug = $1
		 ORDER BY config_key`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_model_config not accessible, model configs unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_model_config for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var configs []CoreCapabilityModelConfig
	for rows.Next() {
		var c CoreCapabilityModelConfig
		if err := rows.Scan(
			&c.ID, &c.AgentSlug, &c.ConfigKey, &c.ConfigValue, &c.Description, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_model_config for agent %q: %w", agentSlug, err)
		}
		configs = append(configs, c)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_model_config not accessible (post-iter), model configs unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability_model_config for agent %q: %w", agentSlug, err)
	}
	return configs, nil
}

// GetModelConfigValue returns the config_value for (agentSlug, configKey) from
// ah_core.capability_model_config. The second return value is true when the row
// was found. Non-fatal when the table is missing — returns ("", false, nil) in
// that case.
func (l *CoreCapabilityModelConfigLoader) GetModelConfigValue(ctx context.Context, agentSlug, configKey string) (string, bool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return "", false, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT config_value
		  FROM ah_core.capability_model_config
		 WHERE agent_slug = $1
		   AND config_key = $2
		 LIMIT 1`

	var value string
	err = conn.QueryRow(ctx, query, agentSlug, configKey).Scan(&value)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_model_config not accessible, config value unavailable",
				"agent_slug", agentSlug, "config_key", configKey, "err", err)
			return "", false, nil
		}
		// pgx returns pgx.ErrNoRows when there is no matching row — map to not-found.
		if err.Error() == "no rows in result set" {
			return "", false, nil
		}
		return "", false, fmt.Errorf("core: get model config value for agent %q key %q: %w", agentSlug, configKey, err)
	}
	return value, true, nil
}
