// Package metrics provides token and latency tracking for agent executions.
package metrics

import (
	"errors"
	"strings"
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

// AgentUsage holds usage totals for a single agent.
type AgentUsage struct {
	AgentID     uuid.UUID `json:"agentId"`
	Executions  int       `json:"executions"`
	TotalTokens int64     `json:"totalTokens"`
	TotalCostUSD float64  `json:"totalCostUsd"`
}

// CostBreakdownEntry holds cost aggregated by provider and model.
type CostBreakdownEntry struct {
	Provider     string  `json:"provider"`
	ModelName    string  `json:"modelName"`
	Executions   int     `json:"executions"`
	TotalTokens  int64   `json:"totalTokens"`
	TotalCostUSD float64 `json:"totalCostUsd"`
}

// RecordRequest is the payload for recording agent metrics.
// EstimatedCostUSD is optional; if omitted (zero), cost is auto-calculated
// from Provider and ModelName using built-in per-token pricing.
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

// modelCostPerToken maps "provider:model" to USD cost per 1K tokens.
// Values are averaged (input+output)/2 for simplicity; update as pricing changes.
var modelCostPerToken = map[string]float64{
	"openai:gpt-4o":          0.0075 / 1000,
	"openai:gpt-4o-mini":     0.00015 / 1000,
	"openai:gpt-4-turbo":     0.015 / 1000,
	"openai:gpt-3.5-turbo":   0.001 / 1000,
	"anthropic:claude-opus-4-6":    0.015 / 1000,
	"anthropic:claude-sonnet-4-6":  0.003 / 1000,
	"anthropic:claude-haiku-4-5":   0.00025 / 1000,
	"google:gemini-1.5-pro":  0.00875 / 1000,
	"google:gemini-1.5-flash": 0.000375 / 1000,
}

// EstimateCost returns the estimated cost in USD for the given provider, model, and token count.
// Returns 0 if the model is not in the pricing table.
func EstimateCost(provider, modelName string, totalTokens int) float64 {
	key := strings.ToLower(provider) + ":" + strings.ToLower(modelName)
	rate, ok := modelCostPerToken[key]
	if !ok {
		return 0
	}
	return rate * float64(totalTokens)
}
