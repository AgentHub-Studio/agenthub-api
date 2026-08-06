package chat

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a chat session or message cannot be found.
var ErrNotFound = errors.New("chat: not found")

// ErrRunAlreadyActive is returned when a concurrent run is already in progress for the session.
var ErrRunAlreadyActive = errors.New("chat: a run is already in progress for this session")

// ErrAgentNotFound é retornado quando o agentId em CreateSession
// não existe (FK 23503). Sem isso, FK violations vazavam como
// 422 com SQL state na mensagem.
var ErrAgentNotFound = errors.New("chat: agent not found")

// ErrSessionArchived é retornado quando há tentativa de POST run/message
// numa session com status=ARCHIVED. Bug 246/247: sessions arquivadas
// estavam aceitando novos runs e mensagens silenciosamente.
var ErrSessionArchived = errors.New("chat: session is archived; create a new session to continue")

// ErrMessageNotFound is returned when a clone boundary message does not belong
// to the source chat session.
var ErrMessageNotFound = errors.New("chat: message not found")

// ChatStatus represents the lifecycle status of a ChatSession.
type ChatStatus string

const (
	StatusActive   ChatStatus = "ACTIVE"
	StatusArchived ChatStatus = "ARCHIVED"
)

// ChatRunStatus represents the current state of a ChatRun.
type ChatRunStatus string

const (
	// P-C299-1: queued = published to RabbitMQ, not yet picked up by a worker.
	// active = worker has started processing. This distinction prevents clients
	// from seeing a run as "running" before the LLM loop has actually started,
	// and allows the stale-detection threshold to apply only to truly started runs.
	ChatRunStatusQueued    ChatRunStatus = "queued"
	ChatRunStatusActive    ChatRunStatus = "active"
	ChatRunStatusCompleted ChatRunStatus = "completed"
	ChatRunStatusFailed    ChatRunStatus = "failed"
	ChatRunStatusCancelled ChatRunStatus = "cancelled"
)

// ChatSession is the domain entity for a chat session.
type ChatSession struct {
	ID                     uuid.UUID       `db:"id"`
	AgentID                *uuid.UUID      `db:"agent_id"`
	Title                  string          `db:"title"`
	Status                 ChatStatus      `db:"status"`
	ClonedFromSessionID    *uuid.UUID      `db:"cloned_from_session_id"`
	ClonedFromSessionTitle *string         `db:"cloned_from_session_title"`
	SystemPromptSnapshot   *string         `db:"system_prompt_snapshot"`  // P-C115-1: snapshot at session creation
	ModelConfigSnapshot    json.RawMessage `db:"model_config_snapshot"`   // P-C330-1: snapshot at session creation
	SkillBindingsSnapshot  json.RawMessage `db:"skill_bindings_snapshot"` // P-C115-1: skill IDs at session creation
	AgentSnapshot          json.RawMessage `db:"agent_snapshot"`          // RT-01: canonical agent config snapshot
	AgentSnapshotHash      *string         `db:"agent_snapshot_hash"`     // RT-01: SHA-256 of AgentSnapshot
	ConfigHash             *string         `db:"config_hash"`             // P-C173-1: SHA-256 of modelConfig at last run
	CreatedAt              time.Time       `db:"created_at"`
	UpdatedAt              time.Time       `db:"updated_at"`
}

// SkillBindingsSnapshotData is the JSON structure stored in ChatSession.SkillBindingsSnapshot.
// P-C115-1: ensures the same tool set is available across all runs of the session.
type SkillBindingsSnapshotData struct {
	SkillIDs []uuid.UUID `json:"skillIds"`
}

// AgentSnapshotData is the canonical RT-01 JSON structure stored in
// ChatSession.AgentSnapshot.
type AgentSnapshotData struct {
	SystemPrompt string          `json:"systemPrompt"`
	ModelConfig  json.RawMessage `json:"modelConfig,omitempty"`
	SkillIDs     []uuid.UUID     `json:"skillIds,omitempty"`
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
	ID            uuid.UUID       `db:"id"`
	SessionID     uuid.UUID       `db:"session_id"`
	TenantID      string          `db:"tenant_id"`
	Status        ChatRunStatus   `db:"status"`
	LastEventID   *string         `db:"last_event_id"`
	Metadata      json.RawMessage `db:"metadata"`
	StartedAt     *time.Time      `db:"started_at"`
	CompletedAt   *time.Time      `db:"completed_at"`
	FailureReason *string         `db:"failure_reason"`
	CreatedAt     time.Time       `db:"created_at"`
}

// RunSessionOptions carries request-scoped execution overrides. These values
// are intended for preview surfaces such as Agent Studio and must not mutate
// the source Agent record.
type RunSessionOptions struct {
	SystemPromptOverride *string
	// AgentConfig is an already-authorized configuration for this request. It is
	// reused only when it belongs to the session agent, avoiding a duplicate
	// repository load without changing the session snapshot contract.
	AgentConfig *AgentRunConfig
}

// ChatSessionResponse is the DTO for a chat session.
type ChatSessionResponse struct {
	ID                     uuid.UUID  `json:"id"`
	AgentID                *uuid.UUID `json:"agentId,omitempty"`
	Title                  string     `json:"title"`
	Status                 ChatStatus `json:"status"`
	ClonedFromSessionID    *uuid.UUID `json:"clonedFromSessionId,omitempty"`
	ClonedFromSessionTitle *string    `json:"clonedFromSessionTitle,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

// ChatMessageResponse is the DTO for a chat message.
type ChatMessageResponse struct {
	ID               uuid.UUID       `json:"id"`
	SessionID        uuid.UUID       `json:"sessionId"`
	Role             string          `json:"role"`
	Content          string          `json:"content"`
	MessageType      MessageType     `json:"messageType"`
	ToolCalls        json.RawMessage `json:"toolCalls,omitempty"`
	ToolCallID       *string         `json:"toolCallId,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	OriginalContent  *string         `json:"originalContent,omitempty"`
	ProcessedContent *string         `json:"processedContent,omitempty"`
	TokenUsage       json.RawMessage `json:"tokenUsage,omitempty"`
	FinishReason     *string         `json:"finishReason,omitempty"`
	TurnIndex        int             `json:"turnIndex"`
	RunID            *uuid.UUID      `json:"runId,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
}

// ChatRunResponse is the DTO for a chat run.
type ChatRunResponse struct {
	ID            uuid.UUID       `json:"id"`
	SessionID     uuid.UUID       `json:"sessionId"`
	Status        ChatRunStatus   `json:"status"`
	LastEventID   *string         `json:"lastEventId,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	StartedAt     *time.Time      `json:"startedAt,omitempty"`
	CompletedAt   *time.Time      `json:"completedAt,omitempty"`
	FailureReason *string         `json:"failureReason,omitempty"`
}

// PromptIdentity is the request-scoped identity used to render dynamic prompt
// placeholders without calling an LLM.
type PromptIdentity struct {
	UserID     string
	UserEmail  string
	Username   string
	Roles      []string
	TenantID   string
	TenantName string
}

// EffectivePromptResponse is returned by GET /api/chat/sessions/{id}/effective-prompt.
type EffectivePromptResponse struct {
	SessionID    uuid.UUID  `json:"sessionId"`
	AgentID      *uuid.UUID `json:"agentId,omitempty"`
	SystemPrompt string     `json:"systemPrompt"`
	Warnings     []string   `json:"warnings"`
}

// EffectiveToolResponse describes one callable tool exposed for the current
// request identity.
type EffectiveToolResponse struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Builtin     bool   `json:"builtin"`
	SkillSlug   string `json:"skillSlug,omitempty"`
	Deferred    bool   `json:"deferred,omitempty"`
}

// EffectiveToolsResponse is returned by GET /api/chat/sessions/{id}/effective-tools.
type EffectiveToolsResponse struct {
	SessionID uuid.UUID               `json:"sessionId"`
	AgentID   *uuid.UUID              `json:"agentId,omitempty"`
	Tools     []EffectiveToolResponse `json:"tools"`
	Warnings  []string                `json:"warnings"`
}

// ChatSessionListStamp is a lightweight signature for detecting list changes
// without reloading the full session page on every poll cycle.
type ChatSessionListStamp struct {
	Count           int64     `json:"count"`
	LatestUpdatedAt time.Time `json:"latestUpdatedAt"`
}

// SessionResponseFrom maps a ChatSession entity to a ChatSessionResponse DTO.
func SessionResponseFrom(s ChatSession) ChatSessionResponse {
	return ChatSessionResponse{
		ID:                     s.ID,
		AgentID:                s.AgentID,
		Title:                  s.Title,
		Status:                 s.Status,
		ClonedFromSessionID:    s.ClonedFromSessionID,
		ClonedFromSessionTitle: s.ClonedFromSessionTitle,
		CreatedAt:              s.CreatedAt,
		UpdatedAt:              s.UpdatedAt,
	}
}

// MessageResponseFrom maps a ChatMessage entity to a ChatMessageResponse DTO.
func MessageResponseFrom(m ChatMessage) ChatMessageResponse {
	resp := ChatMessageResponse{
		ID:           m.ID,
		SessionID:    m.SessionID,
		Role:         m.Role,
		Content:      m.Content,
		MessageType:  m.MessageType,
		ToolCalls:    redactPublicToolCalls(m.ToolCalls),
		ToolCallID:   m.ToolCallID,
		Metadata:     m.Metadata,
		TokenUsage:   m.TokenUsage,
		FinishReason: m.FinishReason,
		TurnIndex:    m.TurnIndex,
		RunID:        m.RunID,
		CreatedAt:    m.CreatedAt,
	}
	var metadata struct {
		OriginalContent  *string `json:"originalContent"`
		ProcessedContent *string `json:"processedContent"`
	}
	if len(m.Metadata) > 0 && json.Unmarshal(m.Metadata, &metadata) == nil {
		resp.OriginalContent = metadata.OriginalContent
		resp.ProcessedContent = metadata.ProcessedContent
	}
	return resp
}

// redactPublicToolCalls produces a safe public copy of persisted tool-call
// payloads. Tool arguments can contain external credentials; the SSE boundary
// already redacts them, so history responses must provide the same guarantee
// without changing the payload used for run resumption.
func redactPublicToolCalls(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return json.RawMessage(strconv.Quote(redactAsyncExternalDiagnosticText(string(raw))))
	}

	redacted, err := json.Marshal(redactPublicToolCallValue(value))
	if err != nil {
		return json.RawMessage(strconv.Quote(redactAsyncExternalDiagnosticText(string(raw))))
	}
	return redacted
}

func redactPublicToolCallValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			if isAsyncLogSensitiveKey(key) {
				continue
			}
			result[key] = redactPublicToolCallValue(child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = redactPublicToolCallValue(child)
		}
		return result
	case string:
		var embedded any
		if json.Unmarshal([]byte(typed), &embedded) == nil {
			if redacted, err := json.Marshal(redactPublicToolCallValue(embedded)); err == nil {
				return string(redacted)
			}
		}
		return redactAsyncLogText(typed)
	default:
		return value
	}
}

// RunResponseFrom maps a ChatRun entity to a ChatRunResponse DTO.
func RunResponseFrom(r ChatRun) ChatRunResponse {
	failureReason := r.FailureReason
	if failureReason != nil {
		redacted := redactAsyncExternalDiagnosticText(*failureReason)
		failureReason = &redacted
	}
	metadata := r.Metadata
	if len(metadata) > 0 {
		redacted := redactAsyncExternalDiagnosticText(string(metadata))
		if json.Valid([]byte(redacted)) {
			metadata = json.RawMessage(redacted)
		} else {
			metadata = json.RawMessage(strconv.Quote(redacted))
		}
	}

	return ChatRunResponse{
		ID:            r.ID,
		SessionID:     r.SessionID,
		Status:        r.Status,
		LastEventID:   r.LastEventID,
		Metadata:      metadata,
		StartedAt:     r.StartedAt,
		CompletedAt:   r.CompletedAt,
		FailureReason: failureReason,
	}
}

// CreateSessionRequest is the payload for creating a chat session.
type CreateSessionRequest struct {
	AgentID *uuid.UUID `json:"agentId,omitempty"`
	Title   string     `json:"title"`
	// AgentConfig is an internal, request-scoped configuration supplied by a
	// caller that already loaded the target agent. It is never accepted from or
	// exposed to the public API.
	AgentConfig *AgentRunConfig `json:"-"`
}

// CloneSessionRequest is the payload for creating a branched copy of a session.
// UntilMessageID is inclusive when provided.
type CloneSessionRequest struct {
	UntilMessageID *uuid.UUID `json:"untilMessageId,omitempty"`
	Title          string     `json:"title,omitempty"`
}

// PendingElicitationInfo describes an unresolved ask_user elicitation request.
// P-C101-1: returned by GetPendingElicitations so async (RabbitMQ) callers can
// discover pending user-input requests when the original SSE stream is gone.
type PendingElicitationInfo struct {
	RequestID string      `json:"requestId"`
	Payload   interface{} `json:"payload,omitempty"`
	CreatedAt time.Time   `json:"createdAt"`
}

// AgentRoutingInfo carries lightweight agent metadata used by the smart router
// to select the best agent for a given user message without loading full configs.
type AgentRoutingInfo struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	Description string
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
