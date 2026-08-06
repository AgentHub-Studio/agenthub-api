package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

const (
	ActionInvoke = "invoke"

	defaultPairLimit       = 100
	defaultPairLimitWindow = time.Minute
	defaultAuditTimeout    = 2 * time.Second
)

var (
	ErrValidation  = errors.New("a2a: validation failed")
	ErrForbidden   = errors.New("a2a: forbidden")
	ErrRateLimited = errors.New("a2a: rate limited")
	ErrNotFound    = errors.New("a2a: target not found")
)

type Grant struct {
	ID            uuid.UUID `json:"id" db:"id"`
	SubjectTenant string    `json:"subjectTenant" db:"subject_tenant"`
	AgentID       uuid.UUID `json:"agentId" db:"agent_id"`
	Actions       []string  `json:"actions" db:"actions"`
	CreatedAt     time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt     time.Time `json:"updatedAt" db:"updated_at"`
}

type InvokeResult struct {
	SessionID    uuid.UUID
	TargetTenant string
	AgentID      uuid.UUID
	Events       <-chan chat.RunEvent
	Preparation  InvokePreparationTimings
	cancel       context.CancelFunc
}

// Cancel ends the target execution when the SSE connection is no longer active.
func (r InvokeResult) Cancel() {
	if r.cancel != nil {
		r.cancel()
	}
}

// InvokeStartedEvent is the first SSE event emitted for a successful A2A
// invocation. The subsequent events are the standard chat run events.
type InvokeStartedEvent struct {
	SessionID    uuid.UUID                `json:"sessionId"`
	TargetTenant string                   `json:"targetTenant"`
	AgentID      uuid.UUID                `json:"agentId"`
	Preparation  InvokePreparationTimings `json:"preparation"`
}

// InvokePreparationTimings provides coarse, non-sensitive timing for the
// synchronous work that must complete before an A2A SSE stream can start.
// Values are elapsed milliseconds and do not disclose tenant data or inputs.
type InvokePreparationTimings struct {
	GrantCheckMS    int64 `json:"grantCheckMs"`
	RateLimitMS     int64 `json:"rateLimitMs"`
	AgentLoadMS     int64 `json:"agentLoadMs"`
	SessionCreateMS int64 `json:"sessionCreateMs"`
	RunPrepareMS    int64 `json:"runPrepareMs"`
	TotalMS         int64 `json:"totalMs"`
}

func (r InvokeResult) StartedEventData() json.RawMessage {
	data, err := json.Marshal(InvokeStartedEvent{
		SessionID:    r.SessionID,
		TargetTenant: r.TargetTenant,
		AgentID:      r.AgentID,
		Preparation:  r.Preparation,
	})
	if err != nil {
		return json.RawMessage("null")
	}
	return data
}
