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

// ErrDuplicateName é retornado quando já existe um trigger com o
// mesmo name para o agente. Mapeado para 409 no handler.
var ErrDuplicateName = errors.New("trigger: a trigger with this name already exists for the agent")

// ErrNotificationWebhookNotFound is returned when the optional notification
// webhook configured for a trigger does not exist.
var ErrNotificationWebhookNotFound = errors.New("trigger: notification webhook not found")

// NullableUUID captures whether a JSON UUID field was omitted or explicitly
// provided as null. Used by PATCH/PUT payloads that must support clearing
// nullable foreign keys.
type NullableUUID struct {
	Set   bool
	Value *uuid.UUID
}

// UnmarshalJSON records field presence and decodes either a UUID or null.
func (n *NullableUUID) UnmarshalJSON(data []byte) error {
	n.Set = true
	if string(data) == "null" {
		n.Value = nil
		return nil
	}
	var id uuid.UUID
	if err := json.Unmarshal(data, &id); err != nil {
		return err
	}
	n.Value = &id
	return nil
}

// MarshalJSON exists for tests and typed clients. A zero NullableUUID still
// serializes as null if explicitly marshaled, so clients should usually omit it.
func (n NullableUUID) MarshalJSON() ([]byte, error) {
	if n.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*n.Value)
}

// RunStatus represents the state of a trigger run.
type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

// AgentTrigger represents a scheduled execution of an agent.
type AgentTrigger struct {
	ID                    uuid.UUID       `json:"id"`
	AgentID               uuid.UUID       `json:"agentId"`
	Name                  string          `json:"name"`
	CronExpression        string          `json:"cronExpression"`
	Enabled               bool            `json:"enabled"`
	InputTemplate         json.RawMessage `json:"inputTemplate,omitempty"`
	NotificationWebhookID *uuid.UUID      `json:"notificationWebhookId,omitempty"`
	LastRunAt             *time.Time      `json:"lastRunAt,omitempty"`
	NextRunAt             *time.Time      `json:"nextRunAt,omitempty"`
	RunCount              int             `json:"runCount"`
	CreatedAt             time.Time       `json:"createdAt"`
	UpdatedAt             time.Time       `json:"updatedAt"`
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
	Name                  string          `json:"name"`
	CronExpression        string          `json:"cronExpression"`
	Enabled               *bool           `json:"enabled,omitempty"`
	InputTemplate         json.RawMessage `json:"inputTemplate,omitempty"`
	NotificationWebhookID *uuid.UUID      `json:"notificationWebhookId,omitempty"`
}

// UpdateTriggerRequest is the payload for updating a trigger.
type UpdateTriggerRequest struct {
	Name                  *string          `json:"name,omitempty"`
	CronExpression        *string          `json:"cronExpression,omitempty"`
	Enabled               *bool            `json:"enabled,omitempty"`
	InputTemplate         *json.RawMessage `json:"inputTemplate,omitempty"`
	NotificationWebhookID NullableUUID     `json:"notificationWebhookId,omitempty"`
}
