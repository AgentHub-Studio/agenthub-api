package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityInteractionMode is a single per-agent interaction mode row
// loaded from ah_core.capability_interaction_mode. Each row encodes one
// dimension of how a core capability agent prefers to interact: whether it
// favours single-turn vs. multi-turn dialogue, how proactively it asks
// clarifying questions, and what response style it applies.
type CoreCapabilityInteractionMode struct {
	ID          int64
	AgentSlug   string
	ModeKey     string
	ModeValue   string
	Description string
	CreatedAt   time.Time
}

// ============================================================
// Seed catalog constants — migration 000122 (2026-05-11).
// ============================================================

// SeedInteractionModeCount is the total number of capability_interaction_mode
// rows seeded by migration 000122: 9 rows (3 mode dimensions × 3 agents).
const SeedInteractionModeCount = 9

// SeedInteractionModeAgentCount is the number of capability agents that have
// seeded interaction mode rows in migration 000122 (researcher, analyst, planner).
const SeedInteractionModeAgentCount = 3

// Mode key constants for well-known capability_interaction_mode.mode_key values.
const (
	// SeedModeKeyPrimary is the mode_key that encodes the agent's fundamental
	// interaction pattern: single-turn Q&A, multi-turn dialogue, or task execution.
	SeedModeKeyPrimary = "primary_mode"

	// SeedModeKeyProactiveQ is the mode_key that encodes how proactively the
	// agent asks clarifying questions (low or high).
	SeedModeKeyProactiveQ = "proactive_questions"

	// SeedModeKeyResponseStyle is the mode_key that encodes the structural
	// format the agent uses when presenting its output.
	SeedModeKeyResponseStyle = "response_style"
)

// Primary mode value constants for capability_interaction_mode.mode_value
// when mode_key = SeedModeKeyPrimary.
const (
	// SeedPrimaryModeSingleTurn indicates the agent responds in one comprehensive
	// reply per query without expecting a follow-up exchange.
	SeedPrimaryModeSingleTurn = "single_turn"

	// SeedPrimaryModeMultiTurn indicates the agent engages in a back-and-forth
	// dialogue: gathering information, presenting results, and refining.
	SeedPrimaryModeMultiTurn = "multi_turn"

	// SeedPrimaryModeTaskExecution indicates the agent focuses on decomposing a
	// goal into steps and tracking execution rather than conversing.
	SeedPrimaryModeTaskExecution = "task_execution"
)

// Proactive questions value constants for capability_interaction_mode.mode_value
// when mode_key = SeedModeKeyProactiveQ.
const (
	// SeedProactiveQLow indicates the agent proceeds with reasonable assumptions
	// and asks few clarifying questions.
	SeedProactiveQLow = "low"

	// SeedProactiveQHigh indicates the agent actively asks for clarification when
	// data or goals are ambiguous before proceeding.
	SeedProactiveQHigh = "high"
)

// Per-agent primary mode aliases — convenience constants that name which
// primary mode each seeded core capability agent uses.
const (
	// SeedResearcherPrimaryMode is the primary_mode value for core-researcher.
	SeedResearcherPrimaryMode = SeedPrimaryModeSingleTurn

	// SeedAnalystPrimaryMode is the primary_mode value for core-analyst.
	SeedAnalystPrimaryMode = SeedPrimaryModeMultiTurn

	// SeedPlannerPrimaryMode is the primary_mode value for core-planner.
	SeedPlannerPrimaryMode = SeedPrimaryModeTaskExecution
)

// CoreCapabilityInteractionModeLoader loads per-agent interaction mode rows
// from ah_core.capability_interaction_mode (seeded by migration 000122).
// All methods are non-fatal when the ah_core schema or the
// capability_interaction_mode table is not yet accessible — supports fresh
// deployments where migration 000122 has not yet run.
type CoreCapabilityInteractionModeLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityInteractionModeLoader creates a
// CoreCapabilityInteractionModeLoader backed by pool.
func NewCoreCapabilityInteractionModeLoader(pool *pgxpool.Pool) *CoreCapabilityInteractionModeLoader {
	return &CoreCapabilityInteractionModeLoader{pool: pool}
}

// LoadCapabilityInteractionModes returns all rows from
// ah_core.capability_interaction_mode ordered by agent_slug, mode_key.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityInteractionModeLoader) LoadCapabilityInteractionModes(ctx context.Context) ([]CoreCapabilityInteractionMode, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, mode_key, mode_value, description, created_at
		  FROM ah_core.capability_interaction_mode
		 ORDER BY agent_slug, mode_key`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_interaction_mode not accessible, interaction modes unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_interaction_mode: %w", err)
	}
	defer rows.Close()

	var modes []CoreCapabilityInteractionMode
	for rows.Next() {
		var m CoreCapabilityInteractionMode
		if err := rows.Scan(
			&m.ID, &m.AgentSlug, &m.ModeKey, &m.ModeValue, &m.Description, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_interaction_mode: %w", err)
		}
		modes = append(modes, m)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_interaction_mode not accessible (post-iter), interaction modes unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability_interaction_mode: %w", err)
	}
	return modes, nil
}

// LoadInteractionModesForAgent returns all interaction mode rows for the given
// agentSlug from ah_core.capability_interaction_mode, ordered by mode_key.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityInteractionModeLoader) LoadInteractionModesForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityInteractionMode, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, mode_key, mode_value, description, created_at
		  FROM ah_core.capability_interaction_mode
		 WHERE agent_slug = $1
		 ORDER BY mode_key`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_interaction_mode not accessible, interaction modes unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_interaction_mode for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var modes []CoreCapabilityInteractionMode
	for rows.Next() {
		var m CoreCapabilityInteractionMode
		if err := rows.Scan(
			&m.ID, &m.AgentSlug, &m.ModeKey, &m.ModeValue, &m.Description, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_interaction_mode for agent %q: %w", agentSlug, err)
		}
		modes = append(modes, m)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_interaction_mode not accessible (post-iter), interaction modes unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability_interaction_mode for agent %q: %w", agentSlug, err)
	}
	return modes, nil
}

// GetInteractionModeValue returns the mode_value for (agentSlug, modeKey)
// from ah_core.capability_interaction_mode. Returns ("", false, nil) when the
// row is not found or the table is not accessible (non-fatal).
func (l *CoreCapabilityInteractionModeLoader) GetInteractionModeValue(ctx context.Context, agentSlug, modeKey string) (string, bool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return "", false, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT mode_value
		  FROM ah_core.capability_interaction_mode
		 WHERE agent_slug = $1
		   AND mode_key   = $2
		 LIMIT 1`

	var value string
	err = conn.QueryRow(ctx, query, agentSlug, modeKey).Scan(&value)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_interaction_mode not accessible, interaction mode value unavailable",
				"agent_slug", agentSlug, "mode_key", modeKey, "err", err)
			return "", false, nil
		}
		// pgx returns "no rows in result set" when there is no matching row — map to not-found.
		if err.Error() == "no rows in result set" {
			return "", false, nil
		}
		return "", false, fmt.Errorf("core: get interaction mode value for agent %q key %q: %w", agentSlug, modeKey, err)
	}
	return value, true, nil
}
