// Package metrics provides token and latency tracking for agent executions.
package metrics

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a metrics record is not found.
var ErrNotFound = errors.New("metrics not found")

// AgentMetrics records usage and performance data for a single agent execution.
type AgentMetrics struct {
	ID               uuid.UUID `db:"id"`
	AgentID          uuid.UUID `db:"agent_id"`
	AgentExecutionID uuid.UUID `db:"agent_execution_id"`
	SessionID        string    `db:"session_id"`
	ModelName        string    `db:"model_name"`
	Provider         string    `db:"provider"`
	PromptTokens     int       `db:"prompt_tokens"`
	CompletionTokens int       `db:"completion_tokens"`
	TotalTokens      int       `db:"total_tokens"`
	EstimatedCostUSD float64   `db:"estimated_cost_usd"`
	LatencyMs        int64     `db:"latency_ms"`
	CreatedAt        time.Time `db:"created_at"`
}

// AgentMetricsResponse is the public DTO for AgentMetrics.
type AgentMetricsResponse struct {
	ID               uuid.UUID `json:"id"`
	AgentID          uuid.UUID `json:"agentId"`
	AgentExecutionID uuid.UUID `json:"agentExecutionId"`
	SessionID        string    `json:"sessionId"`
	ModelName        string    `json:"modelName"`
	Provider         string    `json:"provider"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	TotalTokens      int       `json:"totalTokens"`
	EstimatedCostUSD float64   `json:"estimatedCostUsd"`
	LatencyMs        int64     `json:"latencyMs"`
	CreatedAt        time.Time `json:"createdAt"`
}

// ResponseFrom converts AgentMetrics to its public DTO.
func ResponseFrom(m AgentMetrics) AgentMetricsResponse { return AgentMetricsResponse(m) }

// MetricsSummary aggregates metrics for reporting.
type MetricsSummary struct {
	TotalExecutions int     `json:"totalExecutions"`
	TotalTokens     int64   `json:"totalTokens"`
	TotalCostUSD    float64 `json:"totalCostUsd"`
	AvgLatencyMs    float64 `json:"avgLatencyMs"`
}

// RecordRequest is the payload for recording agent metrics.
type RecordRequest struct {
	AgentID          uuid.UUID `json:"agentId"`
	AgentExecutionID uuid.UUID `json:"agentExecutionId"`
	SessionID        string    `json:"sessionId"`
	ModelName        string    `json:"modelName"`
	Provider         string    `json:"provider"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	TotalTokens      int       `json:"totalTokens"`
	EstimatedCostUSD float64   `json:"estimatedCostUsd"`
	LatencyMs        int64     `json:"latencyMs"`
}
