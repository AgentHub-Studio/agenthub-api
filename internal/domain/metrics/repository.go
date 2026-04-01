package metrics

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Repository handles persistence for AgentMetrics.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ListByAgent returns a paginated list of metrics for a specific agent.
func (r *Repository) ListByAgent(ctx context.Context, tenantID string, agentID uuid.UUID, pr pagination.PageRequest) ([]AgentMetrics, int, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var total int
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_metrics WHERE agent_id = $1`, agentID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("metrics: count: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, agent_id, agent_execution_id, session_id, model_name, provider,
		        prompt_tokens, completion_tokens, total_tokens, estimated_cost_usd, latency_ms, created_at
		 FROM agent_metrics
		 WHERE agent_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		agentID, pr.Size, pr.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("metrics: list by agent: %w", err)
	}
	defer rows.Close()

	var items []AgentMetrics
	for rows.Next() {
		var m AgentMetrics
		if err := rows.Scan(
			&m.ID, &m.AgentID, &m.AgentExecutionID, &m.SessionID, &m.ModelName, &m.Provider,
			&m.PromptTokens, &m.CompletionTokens, &m.TotalTokens, &m.EstimatedCostUSD,
			&m.LatencyMs, &m.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("metrics: scan: %w", err)
		}
		items = append(items, m)
	}
	return items, total, rows.Err()
}

// Record inserts a new metrics entry.
func (r *Repository) Record(ctx context.Context, tenantID string, m AgentMetrics) (AgentMetrics, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return AgentMetrics{}, err
	}
	defer release()

	var created AgentMetrics
	err = conn.QueryRow(ctx,
		`INSERT INTO agent_metrics
		 (agent_id, agent_execution_id, session_id, model_name, provider,
		  prompt_tokens, completion_tokens, total_tokens, estimated_cost_usd, latency_ms)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, agent_id, agent_execution_id, session_id, model_name, provider,
		           prompt_tokens, completion_tokens, total_tokens, estimated_cost_usd, latency_ms, created_at`,
		m.AgentID, m.AgentExecutionID, m.SessionID, m.ModelName, m.Provider,
		m.PromptTokens, m.CompletionTokens, m.TotalTokens, m.EstimatedCostUSD, m.LatencyMs,
	).Scan(
		&created.ID, &created.AgentID, &created.AgentExecutionID, &created.SessionID,
		&created.ModelName, &created.Provider, &created.PromptTokens, &created.CompletionTokens,
		&created.TotalTokens, &created.EstimatedCostUSD, &created.LatencyMs, &created.CreatedAt,
	)
	return created, err
}

// GetAgentSummary returns aggregated metrics for a specific agent.
func (r *Repository) GetAgentSummary(ctx context.Context, tenantID string, agentID uuid.UUID) (MetricsSummary, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return MetricsSummary{}, err
	}
	defer release()

	var s MetricsSummary
	err = conn.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(total_tokens),0), COALESCE(SUM(estimated_cost_usd),0),
		        COALESCE(AVG(latency_ms),0)
		 FROM agent_metrics WHERE agent_id = $1`,
		agentID,
	).Scan(&s.TotalExecutions, &s.TotalTokens, &s.TotalCostUSD, &s.AvgLatencyMs)
	return s, err
}

// GetTenantSummary returns aggregated metrics for the entire tenant.
func (r *Repository) GetTenantSummary(ctx context.Context, tenantID string) (MetricsSummary, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return MetricsSummary{}, err
	}
	defer release()

	var s MetricsSummary
	err = conn.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(total_tokens),0), COALESCE(SUM(estimated_cost_usd),0),
		        COALESCE(AVG(latency_ms),0)
		 FROM agent_metrics`,
	).Scan(&s.TotalExecutions, &s.TotalTokens, &s.TotalCostUSD, &s.AvgLatencyMs)
	return s, err
}
