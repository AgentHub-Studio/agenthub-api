package trigger

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a trigger cannot be found.
var ErrNotFound = errors.New("trigger: not found")

// ErrAgentNotFound é retornado quando o agentId em
// POST /api/agents/{agentId}/triggers não existe (FK 23503).
var ErrAgentNotFound = errors.New("trigger: agent not found")

// RunStatus represents the state of a trigger run.
type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

// AgentTrigger represents a scheduled execution of an agent.
type AgentTrigger struct {
	ID             uuid.UUID       `json:"id"`
	AgentID        uuid.UUID       `json:"agentId"`
	Name           string          `json:"name"`
	CronExpression string          `json:"cronExpression"`
	Enabled        bool            `json:"enabled"`
	InputTemplate  json.RawMessage `json:"inputTemplate,omitempty"`
	LastRunAt      *time.Time      `json:"lastRunAt,omitempty"`
	NextRunAt      *time.Time      `json:"nextRunAt,omitempty"`
	RunCount       int             `json:"runCount"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// AgentTriggerRun represents a single execution of a trigger.
type AgentTriggerRun struct {
	ID          uuid.UUID  `json:"id"`
	TriggerID   uuid.UUID  `json:"triggerId"`
	SessionID   uuid.UUID  `json:"sessionId"`
	Status      RunStatus  `json:"status"`
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	TotalTurns  *int       `json:"totalTurns,omitempty"`
	TotalTokens *int       `json:"totalTokens,omitempty"`
	Error       *string    `json:"error,omitempty"`
}

// CreateTriggerRequest is the payload for creating a trigger.
type CreateTriggerRequest struct {
	Name           string          `json:"name"`
	CronExpression string          `json:"cronExpression"`
	Enabled        *bool           `json:"enabled,omitempty"`
	InputTemplate  json.RawMessage `json:"inputTemplate,omitempty"`
}

// UpdateTriggerRequest is the payload for updating a trigger.
type UpdateTriggerRequest struct {
	Name           *string          `json:"name,omitempty"`
	CronExpression *string          `json:"cronExpression,omitempty"`
	Enabled        *bool            `json:"enabled,omitempty"`
	InputTemplate  *json.RawMessage `json:"inputTemplate,omitempty"`
}
