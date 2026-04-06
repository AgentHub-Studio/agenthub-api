package chat

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a chat session or message cannot be found.
var ErrNotFound = errors.New("chat: not found")

// ChatStatus represents the lifecycle status of a ChatSession.
type ChatStatus string

const (
	StatusActive   ChatStatus = "ACTIVE"
	StatusArchived ChatStatus = "ARCHIVED"
)

// ChatRunStatus represents the current state of a ChatRun.
type ChatRunStatus string

const (
	ChatRunStatusActive    ChatRunStatus = "active"
	ChatRunStatusCompleted ChatRunStatus = "completed"
	ChatRunStatusFailed    ChatRunStatus = "failed"
	ChatRunStatusCancelled ChatRunStatus = "cancelled"
)

// ChatSession is the domain entity for a chat session.
type ChatSession struct {
	ID        uuid.UUID  `db:"id"`
	AgentID   *uuid.UUID `db:"agent_id"`
	Title     string     `db:"title"`
	Status    ChatStatus `db:"status"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

// MessageType represents the kind of chat message in the agentic loop.
type MessageType string

const (
	MessageTypeText           MessageType = "text"
	MessageTypeToolUse        MessageType = "tool_use"
	MessageTypeToolResult     MessageType = "tool_result"
	MessageTypeSystem         MessageType = "system"
	MessageTypeCompactSummary MessageType = "compact_summary"
)

// ChatMessage is the domain entity for a message within a session.
type ChatMessage struct {
	ID           uuid.UUID       `db:"id"`
	SessionID    uuid.UUID       `db:"session_id"`
	Role         string          `db:"role"`
	Content      string          `db:"content"`
	MessageType  MessageType     `db:"message_type"`
	ToolCalls    json.RawMessage `db:"tool_calls"`
	ToolCallID   *string         `db:"tool_call_id"`
	Metadata     json.RawMessage `db:"metadata"`
	TokenUsage   json.RawMessage `db:"token_usage"`
	FinishReason *string         `db:"finish_reason"`
	TurnIndex    int             `db:"turn_index"`
	RunID        *uuid.UUID      `db:"run_id"`
	CreatedAt    time.Time       `db:"created_at"`
}

// ChatRun represents a background agentic run.
type ChatRun struct {
	ID          uuid.UUID       `db:"id"`
	SessionID   uuid.UUID       `db:"session_id"`
	TenantID    string          `db:"tenant_id"`
	Status      ChatRunStatus   `db:"status"`
	LastEventID string          `db:"last_event_id"`
	Metadata    json.RawMessage `db:"metadata"`
	StartedAt   time.Time       `db:"started_at"`
	CompletedAt *time.Time      `db:"completed_at"`
}

// ChatSessionResponse is the DTO for a chat session.
type ChatSessionResponse struct {
	ID        uuid.UUID  `json:"id"`
	AgentID   *uuid.UUID `json:"agentId,omitempty"`
	Title     string     `json:"title"`
	Status    ChatStatus `json:"status"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// ChatMessageResponse is the DTO for a chat message.
type ChatMessageResponse struct {
	ID           uuid.UUID       `json:"id"`
	SessionID    uuid.UUID       `json:"sessionId"`
	Role         string          `json:"role"`
	Content      string          `json:"content"`
	MessageType  MessageType     `json:"messageType"`
	ToolCalls    json.RawMessage `json:"toolCalls,omitempty"`
	ToolCallID   *string         `json:"toolCallId,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	TokenUsage   json.RawMessage `json:"tokenUsage,omitempty"`
	FinishReason *string         `json:"finishReason,omitempty"`
	TurnIndex    int             `json:"turnIndex"`
	RunID        *uuid.UUID      `json:"runId,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
}

// ChatRunResponse is the DTO for a chat run.
type ChatRunResponse struct {
	ID          uuid.UUID       `json:"id"`
	SessionID   uuid.UUID       `json:"sessionId"`
	Status      ChatRunStatus   `json:"status"`
	LastEventID string          `json:"lastEventId,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	StartedAt   time.Time       `json:"startedAt"`
	CompletedAt *time.Time      `json:"completedAt,omitempty"`
}

// SessionResponseFrom maps a ChatSession entity to a ChatSessionResponse DTO.
func SessionResponseFrom(s ChatSession) ChatSessionResponse {
	return ChatSessionResponse(s)
}

// MessageResponseFrom maps a ChatMessage entity to a ChatMessageResponse DTO.
func MessageResponseFrom(m ChatMessage) ChatMessageResponse {
	return ChatMessageResponse(m)
}

// RunResponseFrom maps a ChatRun entity to a ChatRunResponse DTO.
func RunResponseFrom(r ChatRun) ChatRunResponse {
	return ChatRunResponse{
		ID:          r.ID,
		SessionID:   r.SessionID,
		Status:      r.Status,
		LastEventID: r.LastEventID,
		Metadata:    r.Metadata,
		StartedAt:   r.StartedAt,
		CompletedAt: r.CompletedAt,
	}
}

// CreateSessionRequest is the payload for creating a chat session.
type CreateSessionRequest struct {
	AgentID *uuid.UUID `json:"agentId,omitempty"`
	Title   string     `json:"title"`
}

// CreateMessageRequest is the payload for adding a message to a session.
type CreateMessageRequest struct {
	Role         string          `json:"role"`
	Content      string          `json:"content"`
	MessageType  MessageType     `json:"messageType,omitempty"`
	ToolCalls    json.RawMessage `json:"toolCalls,omitempty"`
	ToolCallID   *string         `json:"toolCallId,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	TokenUsage   json.RawMessage `json:"tokenUsage,omitempty"`
	FinishReason *string         `json:"finishReason,omitempty"`
	TurnIndex    int             `json:"turnIndex,omitempty"`
	RunID        *uuid.UUID      `json:"runId,omitempty"`
}
