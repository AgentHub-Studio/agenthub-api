package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreCapabilityRateLimitLoader loads per-capability agent rate limits
// from ah_core.capability_rate_limit. These 9 rows (seeded by migration
// 000105) define cost-protection guardrails for each capability agent,
// preventing runaway LLM expenses for new tenants:
//
//   - core-researcher: requests_per_minute=10, tokens_per_minute=50000, max_concurrent_runs=2
//   - core-analyst:    requests_per_minute=8,  tokens_per_minute=80000, max_concurrent_runs=2
//   - core-planner:    requests_per_minute=5,  tokens_per_minute=20000, max_concurrent_runs=3
//
// Non-fatal when the ah_core schema or the capability_rate_limit table
// is missing — supports fresh deployments where migration 000105 has not yet run.
type CoreCapabilityRateLimitLoader struct {
	pool *pgxpool.Pool
}

// NewCoreCapabilityRateLimitLoader creates a CoreCapabilityRateLimitLoader
// backed by pool.
func NewCoreCapabilityRateLimitLoader(pool *pgxpool.Pool) *CoreCapabilityRateLimitLoader {
	return &CoreCapabilityRateLimitLoader{pool: pool}
}

// CoreCapabilityRateLimit is a single per-agent rate limit definition.
// Captures the slug, the agent it belongs to, the limit key, the numeric
// limit value, the window duration in seconds (0 for concurrency limits),
// and an optional human-readable description.
type CoreCapabilityRateLimit struct {
	Slug          string
	AgentSlug     string
	LimitKey      string
	LimitValue    int
	WindowSeconds int
	Description   string
}

// LoadCapabilityRateLimits returns all capability rate limit rows from
// ah_core.capability_rate_limit WHERE agent_slug = ANY($1), ordered by
// agent_slug then limit_key. Returns nil, nil when the table is not
// accessible (non-fatal).
func (l *CoreCapabilityRateLimitLoader) LoadCapabilityRateLimits(ctx context.Context) ([]CoreCapabilityRateLimit, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, limit_key, limit_value, window_seconds, COALESCE(description, '')
		  FROM ah_core.capability_rate_limit
		 WHERE agent_slug = ANY($1)
		 ORDER BY agent_slug, limit_key`

	rows, err := conn.Query(ctx, query, SeedCapabilityRateLimitAgentSlugs)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_rate_limit not accessible, capability rate limits unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query capability rate limits: %w", err)
	}
	defer rows.Close()

	var limits []CoreCapabilityRateLimit
	for rows.Next() {
		var rl CoreCapabilityRateLimit
		if err := rows.Scan(
			&rl.Slug, &rl.AgentSlug, &rl.LimitKey, &rl.LimitValue, &rl.WindowSeconds, &rl.Description,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability rate limit: %w", err)
		}
		limits = append(limits, rl)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_rate_limit not accessible (post-iter), capability rate limits unavailable", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate capability rate limits: %w", err)
	}
	return limits, nil
}

// LoadRateLimitsForAgent returns all capability rate limit rows for a single
// agent from ah_core.capability_rate_limit WHERE agent_slug = $1, ordered by
// limit_key. Returns nil, nil when the table is not accessible (non-fatal).
func (l *CoreCapabilityRateLimitLoader) LoadRateLimitsForAgent(ctx context.Context, agentSlug string) ([]CoreCapabilityRateLimit, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT slug, agent_slug, limit_key, limit_value, window_seconds, COALESCE(description, '')
		  FROM ah_core.capability_rate_limit
		 WHERE agent_slug = $1
		 ORDER BY limit_key`

	rows, err := conn.Query(ctx, query, agentSlug)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_rate_limit not accessible, load for agent unavailable", "err", err, "agent_slug", agentSlug)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query rate limits for agent: %w", err)
	}
	defer rows.Close()

	var limits []CoreCapabilityRateLimit
	for rows.Next() {
		var rl CoreCapabilityRateLimit
		if err := rows.Scan(
			&rl.Slug, &rl.AgentSlug, &rl.LimitKey, &rl.LimitValue, &rl.WindowSeconds, &rl.Description,
		); err != nil {
			return nil, fmt.Errorf("core: scan capability rate limit for agent: %w", err)
		}
		limits = append(limits, rl)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.capability_rate_limit not accessible (post-iter), load for agent unavailable", "err", err, "agent_slug", agentSlug)
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate rate limits for agent: %w", err)
	}
	return limits, nil
}

// ============================================================
// Seed catalog constants — migration 000105 (2026-05-11).
// ============================================================

// SeedCapabilityRateLimitCount is the expected total row count after
// migration 000105. Nine rate limit rows — three limit types for each of
// the three capability agents (researcher, analyst, planner).
const SeedCapabilityRateLimitCount = 9

// SeedCapabilityRateLimitAgentSlugs is the canonical closed set of agent
// slugs that have rate limit rows seeded by migration 000105.
var SeedCapabilityRateLimitAgentSlugs = []string{
	"core-researcher",
	"core-analyst",
	"core-planner",
}

// Per-agent rate limit count constants — each agent has exactly 3 limits.

// SeedResearcherRateLimitCount is the number of rate limit rows for the
// researcher capability agent (requests_per_minute / tokens_per_minute /
// max_concurrent_runs).
const SeedResearcherRateLimitCount = 3

// SeedAnalystRateLimitCount is the number of rate limit rows for the analyst
// capability agent.
const SeedAnalystRateLimitCount = 3

// SeedPlannerRateLimitCount is the number of rate limit rows for the planner
// capability agent.
const SeedPlannerRateLimitCount = 3

// Limit key constants — the three distinct keys seeded per agent.

// SeedRateLimitKeyRequestsPerMinute is the limit key for max LLM requests
// per minute within a 60 s time window.
const SeedRateLimitKeyRequestsPerMinute = "requests_per_minute"

// SeedRateLimitKeyTokensPerMinute is the limit key for max tokens consumed
// per minute within a 60 s time window.
const SeedRateLimitKeyTokensPerMinute = "tokens_per_minute"

// SeedRateLimitKeyMaxConcurrentRuns is the limit key for max simultaneous
// agent runs. window_seconds=0 because concurrency is not time-windowed.
const SeedRateLimitKeyMaxConcurrentRuns = "max_concurrent_runs"

// Notable limit values — used in tests and runtime enforcement logic.

// SeedResearcherRequestsPerMinute is the requests_per_minute limit for the
// researcher agent (10) — highest of the three agents; active web research
// loops require more frequent LLM calls.
const SeedResearcherRequestsPerMinute = 10

// SeedAnalystRequestsPerMinute is the requests_per_minute limit for the
// analyst agent (8) — document analysis is deliberate; fewer calls than
// the researcher.
const SeedAnalystRequestsPerMinute = 8

// SeedPlannerRequestsPerMinute is the requests_per_minute limit for the
// planner agent (5) — structured planning uses the fewest LLM requests.
const SeedPlannerRequestsPerMinute = 5

// SeedResearcherTokensPerMinute is the tokens_per_minute limit for the
// researcher agent (50 000) — web search results are typically concise.
const SeedResearcherTokensPerMinute = 50000

// SeedAnalystTokensPerMinute is the tokens_per_minute limit for the analyst
// agent (80 000) — highest token budget because it processes large documents.
const SeedAnalystTokensPerMinute = 80000

// SeedPlannerTokensPerMinute is the tokens_per_minute limit for the planner
// agent (20 000) — task decomposition uses smaller context windows.
const SeedPlannerTokensPerMinute = 20000

// SeedResearcherMaxConcurrentRuns is the max_concurrent_runs limit for the
// researcher agent (2) — two parallel research tasks without overloading quota.
const SeedResearcherMaxConcurrentRuns = 2

// SeedAnalystMaxConcurrentRuns is the max_concurrent_runs limit for the
// analyst agent (2) — heavy document processing limits safe concurrency.
const SeedAnalystMaxConcurrentRuns = 2

// SeedPlannerMaxConcurrentRuns is the max_concurrent_runs limit for the
// planner agent (3) — highest concurrency of the three; lightweight tasks.
const SeedPlannerMaxConcurrentRuns = 3

// SeedConcurrentRunsWindowSeconds is the window_seconds value for all
// max_concurrent_runs rows (0) — concurrency limits are not time-windowed.
const SeedConcurrentRunsWindowSeconds = 0

// SeedTimedLimitWindowSeconds is the window_seconds value for
// requests_per_minute and tokens_per_minute rows (60 s sliding window).
const SeedTimedLimitWindowSeconds = 60
