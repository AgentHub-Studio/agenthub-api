package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// AgentLoader returns agent configuration needed by the Runner.
type AgentLoader interface {
	GetAgentForRun(ctx context.Context, id uuid.UUID) (*AgentRunConfig, error)
}


// AgentRunConfig carries agent fields consumed by the agentic Runner.
type AgentRunConfig struct {
	ID               uuid.UUID
	SystemPrompt     string
	ModelConfig      json.RawMessage // raw JSON — passed to RunConfigFromModelConfig
	PermissionRules  json.RawMessage // raw JSON — {"allow":[],"deny":[],"confirm":[]}
	// EnableManagement controls whether the agenthub_manage builtin tool is included.
	// P-C184-2: both this flag AND the caller's admin role must be true.
	EnableManagement bool
	// Status is the agent's lifecycle status. Used to reject runs for DRAFT/ARCHIVED agents.
	// P-C178-1.
	Status string
}

// ErrAgentNotPublished is returned when an agent is not in PUBLISHED status.
var ErrAgentNotPublished = errors.New("agent is not published")

// ErrAgentArchived is returned when an agent has been archived.
var ErrAgentArchived = errors.New("agent is archived and no longer accepts new sessions")

// RunEvent is the envelope emitted by the agentic loop.
// Defined here (in the chat package) to avoid an import cycle:
// chat → chat/agentic → chat.
type RunEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// RunInput carries everything needed to start an agentic run.
type RunInput struct {
	RunID        uuid.UUID // ID of the persisted run
	SessionID    uuid.UUID
	AgentID      uuid.UUID
	UserMessage  string
	SystemPrompt string
	TenantID     string
	// UserMessageID is non-nil when the user message was already persisted by the
	// caller (typically chat.Service.RunSession). When set, the runner skips
	// its own user message persistence to avoid duplicates. P-C178-2.
	UserMessageID *uuid.UUID
	// SystemPromptSnapshot and ModelConfigSnapshot carry the values captured at
	// session creation time (P-C115-1 / P-C330-1). When set, the adapter uses
	// them instead of the agent's current values to ensure consistency throughout
	// the conversation even if the agent is updated between runs.
	SystemPromptSnapshot *string
	ModelConfigSnapshot  json.RawMessage
}

// ElicitationResponder routes a user's elicitation response to the active run.
// Implemented by agentic.SessionRunnerAdapter; no-op on other implementations.
type ElicitationResponder interface {
	RespondElicitation(sessionID, requestID string, result ElicitationResult) bool
}

// ElicitationResult mirrors agentic.ElicitationResult to avoid circular imports.
type ElicitationResult struct {
	Action  string                 `json:"action"`
	Content map[string]interface{} `json:"content,omitempty"`
}

// SessionRunner starts an agentic loop and returns a channel of RunEvents.
// The chat.Service calls this; the concrete implementation lives in
// chat/agentic and is injected via the server wiring.
type SessionRunner interface {
	RunSession(ctx context.Context, in RunInput) (<-chan RunEvent, error)
}

// Service provides business logic for chat operations.
type Service struct {
	repo        Repository
	runner      SessionRunner
	agentLoader AgentLoader // optional — used to capture agent snapshot at session creation
}

// NewService creates a new Service backed by the given Repository.
// runner may be nil (disables agentic features).
func NewService(repo Repository, runner SessionRunner) *Service {
	return &Service{repo: repo, runner: runner}
}

// WithAgentLoader wires an agent loader for capturing snapshots at session creation.
func (s *Service) WithAgentLoader(loader AgentLoader) *Service {
	s.agentLoader = loader
	return s
}

// GetActiveRun returns the active run for a session if any.
func (s *Service) GetActiveRun(ctx context.Context, sessionID uuid.UUID) (ChatRunResponse, bool, error) {
	run, found, err := s.repo.GetActiveRunBySession(ctx, sessionID)
	if err != nil {
		return ChatRunResponse{}, false, fmt.Errorf("chat service: get active run: %w", err)
	}
	if !found {
		return ChatRunResponse{}, false, nil
	}
	return RunResponseFrom(run), true, nil
}

// ListSessions returns a paginated list of chat sessions.
func (s *Service) ListSessions(ctx context.Context, req pagination.PageRequest) (pagination.Page[ChatSessionResponse], error) {
	items, total, err := s.repo.FindSessions(ctx, req)
	if err != nil {
		return pagination.Page[ChatSessionResponse]{}, fmt.Errorf("chat service: list sessions: %w", err)
	}

	responses := make([]ChatSessionResponse, len(items))
	for i, item := range items {
		responses[i] = SessionResponseFrom(item)
	}

	return pagination.NewPage(responses, total, req), nil
}

// GetSession returns a single chat session by ID.
func (s *Service) GetSession(ctx context.Context, id uuid.UUID) (ChatSessionResponse, error) {
	session, err := s.repo.GetSessionByID(ctx, id)
	if err != nil {
		return ChatSessionResponse{}, err
	}
	return SessionResponseFrom(session), nil
}

// CreateSession creates a new chat session.
func (s *Service) CreateSession(ctx context.Context, req CreateSessionRequest) (ChatSessionResponse, error) {
	if req.Title == "" {
		return ChatSessionResponse{}, fmt.Errorf("chat service: title is required")
	}

	session := ChatSession{
		AgentID: req.AgentID,
		Title:   req.Title,
		Status:  StatusActive,
	}

	// P-C115-1 / P-C330-1: capture agent snapshot at session creation so that
	// the persona and model config remain consistent throughout the conversation
	// even if the agent is updated between turns.
	if req.AgentID != nil && s.agentLoader != nil {
		if agentCfg, err := s.agentLoader.GetAgentForRun(ctx, *req.AgentID); err == nil {
			if agentCfg.SystemPrompt != "" {
				snapshot := agentCfg.SystemPrompt
				session.SystemPromptSnapshot = &snapshot
			}
			if len(agentCfg.ModelConfig) > 2 {
				session.ModelConfigSnapshot = agentCfg.ModelConfig
			}
		}
		// Snapshot failure is non-fatal: session creation proceeds without snapshot.
	}

	created, err := s.repo.CreateSession(ctx, session)
	if err != nil {
		return ChatSessionResponse{}, fmt.Errorf("chat service: create session: %w", err)
	}

	return SessionResponseFrom(created), nil
}

// ArchiveSession sets a session's status to ARCHIVED.
func (s *Service) ArchiveSession(ctx context.Context, id uuid.UUID) (ChatSessionResponse, error) {
	session, err := s.repo.UpdateSessionStatus(ctx, id, StatusArchived)
	if err != nil {
		return ChatSessionResponse{}, err
	}
	return SessionResponseFrom(session), nil
}

// RenameSession updates a session's title.
func (s *Service) RenameSession(ctx context.Context, id uuid.UUID, title string) (ChatSessionResponse, error) {
	session, err := s.repo.UpdateSessionTitle(ctx, id, title)
	if err != nil {
		return ChatSessionResponse{}, err
	}
	return SessionResponseFrom(session), nil
}

// DeleteSession removes a chat session by ID.
func (s *Service) DeleteSession(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.DeleteSession(ctx, id); err != nil {
		return fmt.Errorf("chat service: delete session: %w", err)
	}
	return nil
}

// ListMessages returns a paginated list of messages for a session.
func (s *Service) ListMessages(ctx context.Context, sessionID uuid.UUID, req pagination.PageRequest) (pagination.Page[ChatMessageResponse], error) {
	items, total, err := s.repo.FindMessages(ctx, sessionID, req)
	if err != nil {
		return pagination.Page[ChatMessageResponse]{}, fmt.Errorf("chat service: list messages: %w", err)
	}

	responses := make([]ChatMessageResponse, len(items))
	for i, item := range items {
		responses[i] = MessageResponseFrom(item)
	}

	return pagination.NewPage(responses, total, req), nil
}

// GetLatestAssistantMessage delegates to the repository to find the newest assistant
// message created after the given time, returning it as a DTO.
func (s *Service) GetLatestAssistantMessage(ctx context.Context, sessionID uuid.UUID, after time.Time) (ChatMessageResponse, bool, error) {
	msg, found, err := s.repo.GetLatestAssistantMessage(ctx, sessionID, after)
	if err != nil {
		return ChatMessageResponse{}, false, fmt.Errorf("chat service: get latest assistant message: %w", err)
	}
	if !found {
		return ChatMessageResponse{}, false, nil
	}
	return MessageResponseFrom(msg), true, nil
}

// AddMessage adds a message to a chat session.
// Returns the persisted user message DTO. For agent-bound sessions the agentic
// loop is NOT started here — use RunSession instead.
func (s *Service) AddMessage(ctx context.Context, sessionID uuid.UUID, req CreateMessageRequest) (ChatMessageResponse, error) {
	if req.Role == "" {
		return ChatMessageResponse{}, fmt.Errorf("chat service: role is required")
	}
	if req.Content == "" {
		return ChatMessageResponse{}, fmt.Errorf("chat service: content is required")
	}

	m := ChatMessage{
		SessionID: sessionID,
		Role:      req.Role,
		Content:   req.Content,
	}

	created, err := s.repo.CreateMessage(ctx, m)
	if err != nil {
		return ChatMessageResponse{}, fmt.Errorf("chat service: add message: %w", err)
	}

	return MessageResponseFrom(created), nil
}

// RespondElicitation routes a user response to an active elicitation request.
// Returns false when the session has no active run or the requestID is not found.
func (s *Service) RespondElicitation(sessionID, requestID string, result ElicitationResult) bool {
	if r, ok := s.runner.(ElicitationResponder); ok {
		return r.RespondElicitation(sessionID, requestID, result)
	}
	return false
}

// RunSession starts an agentic run for the given session.
// It loads the session, validates it has an agent, then delegates to the SessionRunner.
// The caller (SSE handler) consumes the returned channel for streaming.
func (s *Service) RunSession(ctx context.Context, sessionID uuid.UUID, userMessage, tenantID string) (<-chan RunEvent, error) {
	if s.runner == nil {
		return nil, fmt.Errorf("chat service: agentic features not configured")
	}

	session, err := s.repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("chat service: get session: %w", err)
	}
	if session.AgentID == nil {
		defaultID, err := s.repo.FindDefaultAgentID(ctx)
		if err != nil {
			return nil, fmt.Errorf("chat service: find default agent: %w", err)
		}
		if defaultID == nil {
			return nil, fmt.Errorf("chat service: session has no agent and no published agent exists")
		}
		if err := s.repo.UpdateSessionAgent(ctx, sessionID, *defaultID); err != nil {
			return nil, fmt.Errorf("chat service: bind default agent: %w", err)
		}
		session.AgentID = defaultID
	}

	// P-C178-2: persist user message BEFORE starting the run so it is never lost
	// even if the run fails to initialise (e.g. agent not published, model error).
	var userMsgID *uuid.UUID
	if userMessage != "" {
		msg := ChatMessage{
			SessionID: sessionID,
			Role:      "user",
			Content:   userMessage,
		}
		persisted, err := s.repo.CreateMessage(ctx, msg)
		if err != nil {
			return nil, fmt.Errorf("chat service: persist user message: %w", err)
		}
		userMsgID = &persisted.ID
	}

	return s.runner.RunSession(ctx, RunInput{
		SessionID:            sessionID,
		AgentID:              *session.AgentID,
		UserMessage:          userMessage,
		TenantID:             tenantID,
		UserMessageID:        userMsgID,
		SystemPromptSnapshot: session.SystemPromptSnapshot,
		ModelConfigSnapshot:  session.ModelConfigSnapshot,
	})
}
