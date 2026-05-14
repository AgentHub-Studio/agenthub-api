package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityFallbackBehavior is a single per-agent fallback behavior row
// loaded from ah_core.capability_fallback_behavior. Each row encodes what a
// specific core capability agent does when a primary tool or capability fails:
// the trigger condition, the recovery action, the user-facing message, and
// the number of automatic retries attempted before the action fires.
type CoreCapabilityFallbackBehavior struct {
	ID              int64
	AgentSlug       string
	BehaviorKey     string
	Trigger         string
	Action          string
	FallbackMessage string
	RetryCount      int
	CreatedAt       time.Time
}

// ============================================================
// Seed catalog constants — migration 000121 (2026-05-11).
// ============================================================

// SeedFallbackBehaviorCount is the total number of capability_fallback_behavior
// rows seeded by migration 000121: 9 rows (3 behaviors × 3 agents).
const SeedFallbackBehaviorCount = 9

// SeedFallbackBehaviorAgentCount is the number of capability agents that have
// seeded fallback behaviors in migration 000121 (researcher, analyst, planner).
const SeedFallbackBehaviorAgentCount = 3

// SeedZeroRetryCount is the number of fallback behaviors with retry_count=0,
// meaning the agent falls back immediately without attempting any retry.
// These are: doc_unavailable (analyst), ambiguous_data (analyst), no_goal (planner).
const SeedZeroRetryCount = 3

// Action constants for capability_fallback_behavior.action values.
// Each constant matches one of the 7 allowed values in the CHECK constraint.
const (
	// SeedActionGracefulDegrade signals the agent to answer from training knowledge
	// or provide a friendly degradation message instead of using the failed tool.
	SeedActionGracefulDegrade = "graceful_degrade"

	// SeedActionRetryWithCache signals the agent to retry the operation using
	// cached or alternative data sources.
	SeedActionRetryWithCache = "retry_with_cache"

	// SeedActionRephraseRetry signals the agent to rephrase the original query
	// or request and retry the operation.
	SeedActionRephraseRetry = "rephrase_and_retry"

	// SeedActionRetrySimpler signals the agent to decompose the prompt into
	// simpler sub-requests and retry with reduced complexity.
	SeedActionRetrySimpler = "retry_with_simpler_prompt"

	// SeedActionAskClarification signals the agent to pause and ask the user
	// for additional information before proceeding.
	SeedActionAskClarification = "ask_clarification"

	// SeedActionRetryDirect signals the agent to bypass delegation to a sub-agent
	// and handle the step directly.
	SeedActionRetryDirect = "retry_direct"

	// SeedActionDecompose signals the agent to break the task into smaller phases
	// and retry each phase individually.
	SeedActionDecompose = "decompose_and_retry"
)

// BehaviorKey constants for well-known capability_fallback_behavior.behavior_key values.
const (
	// SeedBehaviorKeySearchFailure is the behavior_key for the core-researcher
	// fallback when the web search tool is unavailable.
	SeedBehaviorKeySearchFailure = "search_failure"

	// SeedBehaviorKeyDocUnavailable is the behavior_key for the core-analyst
	// fallback when the knowledge base is empty or inaccessible.
	SeedBehaviorKeyDocUnavailable = "doc_unavailable"

	// SeedBehaviorKeySubagentFailure is the behavior_key for the core-planner
	// fallback when sub-agent delegation fails.
	SeedBehaviorKeySubagentFailure = "subagent_failure"
)

// CoreCapabilityFallbackBehaviorLoader loads per-agent fallback behavior rows
// from ah_core.capability_fallback_behavior (seeded by migration 000121).
// All methods are non-fatal when the ah_core schema or the
// capability_fallback_behavior table is not yet accessible — supports fresh
// deployments where migration 000121 has not yet run.
type CoreCapabilityFallbackBehaviorLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityFallbackBehaviorLoader creates a
// CoreCapabilityFallbackBehaviorLoader backed by pool.
func NewCoreCapabilityFallbackBehaviorLoader(pool *pgxpool.Pool) *CoreCapabilityFallbackBehaviorLoader {
	return &CoreCapabilityFallbackBehaviorLoader{pool: pool}
}

// LoadCapabilityFallbackBehaviors returns all rows from
// ah_core.capability_fallback_behavior ordered by agent_slug, behavior_key.
// Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityFallbackBehaviorLoader) LoadCapabilityFallbackBehaviors(ctx context.Context) ([]CoreCapabilityFallbackBehavior, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, behavior_key, trigger, action, fallback_message, retry_count, created_at
		  FROM ah_core.capability_fallback_behavior
		 ORDER BY agent_slug, behavior_key`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_fallback_behavior not accessible, fallback behaviors unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_fallback_behavior: %w", err)
	}
	defer rows.Close()

	var behaviors []CoreCapabilityFallbackBehavior
	for rows.Next() {
		var b CoreCapabilityFallbackBehavior
		if err := rows.Scan(
			&b.ID, &b.AgentSlug, &b.BehaviorKey, &b.Trigger, &b.Action,
			&b.FallbackMessage, &b.RetryCount, &b.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_fallback_behavior: %w", err)
		}
		behaviors = append(behaviors, b)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_fallback_behavior not accessible (post-iter), fallback behaviors unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability_fallback_behavior: %w", err)
	}
	return behaviors, nil
}

// LoadFallbackBehaviorsForAgent returns all fallback behavior rows for the
// given agentSlug from ah_core.capability_fallback_behavior, ordered by
// behavior_key. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityFallbackBehaviorLoader) LoadFallbackBehaviorsForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityFallbackBehavior, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, behavior_key, trigger, action, fallback_message, retry_count, created_at
		  FROM ah_core.capability_fallback_behavior
		 WHERE agent_slug = $1
		 ORDER BY behavior_key`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_fallback_behavior not accessible, fallback behaviors unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability_fallback_behavior for agent %q: %w", agentSlug, err)
	}
	defer rows.Close()

	var behaviors []CoreCapabilityFallbackBehavior
	for rows.Next() {
		var b CoreCapabilityFallbackBehavior
		if err := rows.Scan(
			&b.ID, &b.AgentSlug, &b.BehaviorKey, &b.Trigger, &b.Action,
			&b.FallbackMessage, &b.RetryCount, &b.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability_fallback_behavior for agent %q: %w", agentSlug, err)
		}
		behaviors = append(behaviors, b)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_fallback_behavior not accessible (post-iter), fallback behaviors unavailable",
				"agent_slug", agentSlug, "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability_fallback_behavior for agent %q: %w", agentSlug, err)
	}
	return behaviors, nil
}

// GetFallbackBehavior returns the fallback behavior row for (agentSlug, behaviorKey)
// from ah_core.capability_fallback_behavior. Returns nil, nil when the row is
// not found or the table is not accessible (non-fatal).
func (l *CoreCapabilityFallbackBehaviorLoader) GetFallbackBehavior(ctx context.Context, agentSlug, behaviorKey string) (*CoreCapabilityFallbackBehavior, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, agent_slug, behavior_key, trigger, action, fallback_message, retry_count, created_at
		  FROM ah_core.capability_fallback_behavior
		 WHERE agent_slug = $1
		   AND behavior_key = $2
		 LIMIT 1`

	var b CoreCapabilityFallbackBehavior
	err = conn.QueryRow(ctx, query, agentSlug, behaviorKey).Scan(
		&b.ID, &b.AgentSlug, &b.BehaviorKey, &b.Trigger, &b.Action,
		&b.FallbackMessage, &b.RetryCount, &b.CreatedAt,
	)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_fallback_behavior not accessible, fallback behavior unavailable",
				"agent_slug", agentSlug, "behavior_key", behaviorKey, "err", err)
			return nil, nil
		}
		// pgx returns "no rows in result set" when there is no matching row — map to not-found.
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("core: get fallback behavior for agent %q key %q: %w", agentSlug, behaviorKey, err)
	}
	return &b, nil
}
