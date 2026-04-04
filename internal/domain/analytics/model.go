// Package analytics provides query models for agent execution analytics.
package analytics

import (
	"time"

	"github.com/google/uuid"
)

// UsageSummary aggregates agent usage over a time range.
type UsageSummary struct {
	AgentID          uuid.UUID `json:"agentId"`
	TotalRuns        int       `json:"totalRuns"`
	TotalTokens      int64     `json:"totalTokens"`
	PromptTokens     int64     `json:"promptTokens"`
	CompletionTokens int64     `json:"completionTokens"`
	TotalCostUSD     float64   `json:"totalCostUsd"`
	AvgDurationMs    float64   `json:"avgDurationMs"`
	AvgTurns         float64   `json:"avgTurns"`
}

// CostSummary aggregates costs by provider/model.
type CostSummary struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Runs        int     `json:"runs"`
	TotalTokens int64   `json:"totalTokens"`
	CostUSD     float64 `json:"costUsd"`
}

// TopTool represents a frequently used tool.
type TopTool struct {
	ToolName    string  `json:"toolName"`
	Executions  int     `json:"executions"`
	AvgDuration float64 `json:"avgDurationMs"`
	ErrorRate   float64 `json:"errorRate"`
}

// TimeRange represents a query time range.
type TimeRange struct {
	From time.Time
	To   time.Time
}
