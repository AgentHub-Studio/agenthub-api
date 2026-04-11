package analytics

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements AnalyticsStore by querying the agent_metrics table
// that is populated by the agentic runner after every run.
// This closes the interface-only gap without requiring a ClickHouse deployment.
// A ClickHouseStore can be swapped in via dependency injection once ClickHouse is provisioned.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates an AnalyticsStore backed by PostgreSQL.
func NewPostgresStore(pool *pgxpool.Pool) AnalyticsStore {
	return &PostgresStore{pool: pool}
}

// AgentUsage aggregates run counts, token totals, cost, and latency for an agent.
func (s *PostgresStore) AgentUsage(ctx context.Context, agentID uuid.UUID, tr TimeRange) (UsageSummary, error) {
	q := `
		SELECT
			COUNT(*)                                         AS total_runs,
			COALESCE(SUM(total_tokens),        0)           AS total_tokens,
			COALESCE(SUM(prompt_tokens),       0)           AS prompt_tokens,
			COALESCE(SUM(completion_tokens),   0)           AS completion_tokens,
			COALESCE(SUM(estimated_cost_usd),  0)           AS total_cost_usd,
			COALESCE(AVG(latency_ms),          0)           AS avg_duration_ms,
			0::float8                                        AS avg_turns
		FROM agent_metrics
		WHERE agent_id    = $1
		  AND created_at >= $2
		  AND created_at <= $3`

	var sum UsageSummary
	sum.AgentID = agentID
	err := s.pool.QueryRow(ctx, q, agentID, tr.From, tr.To).Scan(
		&sum.TotalRuns,
		&sum.TotalTokens,
		&sum.PromptTokens,
		&sum.CompletionTokens,
		&sum.TotalCostUSD,
		&sum.AvgDurationMs,
		&sum.AvgTurns,
	)
	if err != nil {
		return UsageSummary{}, fmt.Errorf("analytics: agent usage: %w", err)
	}
	return sum, nil
}

// AgentCosts breaks down cost by provider + model for the given agent and time range.
func (s *PostgresStore) AgentCosts(ctx context.Context, agentID uuid.UUID, tr TimeRange) ([]CostSummary, error) {
	q := `
		SELECT
			COALESCE(provider,    'unknown')                AS provider,
			COALESCE(model_name,  'unknown')                AS model,
			COUNT(*)                                        AS runs,
			COALESCE(SUM(total_tokens),       0)            AS total_tokens,
			COALESCE(SUM(estimated_cost_usd), 0)            AS cost_usd
		FROM agent_metrics
		WHERE agent_id    = $1
		  AND created_at >= $2
		  AND created_at <= $3
		GROUP BY provider, model_name
		ORDER BY cost_usd DESC`

	rows, err := s.pool.Query(ctx, q, agentID, tr.From, tr.To)
	if err != nil {
		return nil, fmt.Errorf("analytics: agent costs: %w", err)
	}
	defer rows.Close()

	var out []CostSummary
	for rows.Next() {
		var c CostSummary
		if err := rows.Scan(&c.Provider, &c.Model, &c.Runs, &c.TotalTokens, &c.CostUSD); err != nil {
			return nil, fmt.Errorf("analytics: agent costs scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// TopTools returns the most frequently used tools ranked by execution count.
func (s *PostgresStore) TopTools(ctx context.Context, tr TimeRange, limit int) ([]TopTool, error) {
	q := `
		SELECT
			tool_name,
			COUNT(*)                                                              AS executions,
			COALESCE(AVG(duration_ms),     0)                                   AS avg_duration_ms,
			COALESCE(
				SUM(CASE WHEN is_error THEN 1 ELSE 0 END)::float8 / COUNT(*),
				0
			)                                                                     AS error_rate
		FROM tool_execution
		WHERE created_at >= $1
		  AND created_at <= $2
		GROUP BY tool_name
		ORDER BY executions DESC
		LIMIT $3`

	rows, err := s.pool.Query(ctx, q, tr.From, tr.To, limit)
	if err != nil {
		return nil, fmt.Errorf("analytics: top tools: %w", err)
	}
	defer rows.Close()

	var out []TopTool
	for rows.Next() {
		var t TopTool
		if err := rows.Scan(&t.ToolName, &t.Executions, &t.AvgDuration, &t.ErrorRate); err != nil {
			return nil, fmt.Errorf("analytics: top tools scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

var _ AnalyticsStore = (*PostgresStore)(nil)
