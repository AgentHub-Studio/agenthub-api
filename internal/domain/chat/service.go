package chat

import (
	"context"
	"encoding/json"
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
	ID           uuid.UUID
	SystemPrompt string
	ModelConfig  json.RawMessage // raw JSON — passed to RunConfigFromModelConfig
}

// RunEvent is the envelope emitted by the agentic loop.
// Defined here (in the chat package) to avoid an import cycle:
// chat → chat/agentic → chat.
type RunEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// RunInput carries everything needed to start an agentic run.
type RunInput struct {
	SessionID    uuid.UUID
	AgentID      uuid.UUID
	UserMessage  string
	SystemPrompt string
	TenantID     string
}

// SessionRunner starts an agentic loop and returns a channel of RunEvents.
// The chat.Service calls this; the concrete implementation lives in
// chat/agentic and is injected via the server wiring.
type SessionRunner interface {
	RunSession(ctx context.Context, in RunInput) (<-chan RunEvent, error)
}

// Service provides business logic for chat operations.
type Service struct {
	repo   Repository
	runner SessionRunner
}

// NewService creates a new Service backed by the given Repository.
// runner may be nil (disables agentic features).
func NewService(repo Repository, runner SessionRunner) *Service {
	return &Service{repo: repo, runner: runner}
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
		return nil, fmt.Errorf("chat service: session has no agent")
	}

	return s.runner.RunSession(ctx, RunInput{
		SessionID:   sessionID,
		AgentID:     *session.AgentID,
		UserMessage: userMessage,
		TenantID:    tenantID,
	})
}
