// Package experiment provides A/B testing for agent prompts.
package experiment

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a PromptExperiment is not found.
var ErrNotFound = errors.New("experiment not found")

// ExperimentStatus represents the lifecycle state of an experiment.
type ExperimentStatus string

const (
	ExperimentStatusDraft     ExperimentStatus = "DRAFT"
	ExperimentStatusActive    ExperimentStatus = "ACTIVE"
	ExperimentStatusPaused    ExperimentStatus = "PAUSED"
	ExperimentStatusCompleted ExperimentStatus = "COMPLETED"
)

// PromptExperiment defines an A/B test for agent prompt variants.
type PromptExperiment struct {
	ID           uuid.UUID        `db:"id"`
	AgentID      uuid.UUID        `db:"agent_id"`
	Name         string           `db:"name"`
	Status       ExperimentStatus `db:"status"`
	TrafficSplit string           `db:"traffic_split"`
	Variants     string           `db:"variants"`
	StartDate    time.Time        `db:"start_date"`
	EndDate      time.Time        `db:"end_date"`
	CreatedAt    time.Time        `db:"created_at"`
}

// PromptExperimentResponse is the public DTO for PromptExperiment.
type PromptExperimentResponse struct {
	ID           uuid.UUID        `json:"id"`
	AgentID      uuid.UUID        `json:"agentId"`
	Name         string           `json:"name"`
	Status       ExperimentStatus `json:"status"`
	TrafficSplit string           `json:"trafficSplit"`
	Variants     string           `json:"variants"`
	StartDate    time.Time        `json:"startDate"`
	EndDate      time.Time        `json:"endDate"`
	CreatedAt    time.Time        `json:"createdAt"`
}

// ResponseFrom converts a PromptExperiment to its public DTO.
func ResponseFrom(e PromptExperiment) PromptExperimentResponse { return PromptExperimentResponse(e) }

// ExperimentResult records an observation for a variant in an experiment.
type ExperimentResult struct {
	ID           uuid.UUID `db:"id"`
	ExperimentID uuid.UUID `db:"experiment_id"`
	VariantKey   string    `db:"variant_key"`
	SessionID    string    `db:"session_id"`
	UserFeedback int       `db:"user_feedback"`
	LatencyMs    int64     `db:"latency_ms"`
	TokenCount   int       `db:"token_count"`
	CreatedAt    time.Time `db:"created_at"`
}

// ExperimentResultResponse is the public DTO for ExperimentResult.
type ExperimentResultResponse struct {
	ID           uuid.UUID `json:"id"`
	ExperimentID uuid.UUID `json:"experimentId"`
	VariantKey   string    `json:"variantKey"`
	SessionID    string    `json:"sessionId"`
	UserFeedback int       `json:"userFeedback"`
	LatencyMs    int64     `json:"latencyMs"`
	TokenCount   int       `json:"tokenCount"`
	CreatedAt    time.Time `json:"createdAt"`
}

// ResultResponseFrom converts an ExperimentResult to its public DTO.
func ResultResponseFrom(r ExperimentResult) ExperimentResultResponse {
	return ExperimentResultResponse(r)
}

// CreateRequest is the payload for creating a PromptExperiment.
type CreateRequest struct {
	AgentID      uuid.UUID `json:"agentId"`
	Name         string    `json:"name"`
	TrafficSplit string    `json:"trafficSplit"`
	Variants     string    `json:"variants"`
	StartDate    time.Time `json:"startDate"`
	EndDate      time.Time `json:"endDate"`
}

// RecordResultRequest is the payload for recording an experiment result.
type RecordResultRequest struct {
	VariantKey   string `json:"variantKey"`
	SessionID    string `json:"sessionId"`
	UserFeedback int    `json:"userFeedback"`
	LatencyMs    int64  `json:"latencyMs"`
	TokenCount   int    `json:"tokenCount"`
}
