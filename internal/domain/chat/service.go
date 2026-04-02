package chat

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// agentRunner delegates user messages to the orchestrator for agent-based sessions.
type agentRunner interface {
	RunAgentExecution(ctx context.Context, bearerToken string, agentID string, userMessage string) (string, error)
}

// Service provides business logic for chat operations.
type Service struct {
	repo         Repository
	orchestrator agentRunner
}

// NewService creates a new Service backed by the given Repository.
// Pass a non-nil orchestrator to enable AI responses for agent-based sessions.
func NewService(repo Repository, orchestrator agentRunner) *Service {
	return &Service{repo: repo, orchestrator: orchestrator}
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
// For user messages on agent-bound sessions, it delegates to the orchestrator and
// stores the assistant reply. The user message response is returned; the assistant
// reply (if any) is stored asynchronously and the frontend can poll for it.
func (s *Service) AddMessage(ctx context.Context, r *http.Request, sessionID uuid.UUID, req CreateMessageRequest) (ChatMessageResponse, error) {
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

	// For user messages on agent sessions, trigger orchestrator and store assistant reply.
	if req.Role == "user" && s.orchestrator != nil {
		session, sesErr := s.repo.GetSessionByID(ctx, sessionID)
		if sesErr == nil && session.AgentID != nil {
			bearerToken := r.Header.Get("Authorization")
			go func() {
				bgCtx := context.Background()
				output, runErr := s.orchestrator.RunAgentExecution(bgCtx, bearerToken, session.AgentID.String(), req.Content)
				if runErr != nil {
					slog.Warn("chat service: orchestrator execution failed", "sessionId", sessionID, "err", runErr)
					return
				}
				if output == "" {
					return
				}
				assistantMsg := ChatMessage{
					SessionID: sessionID,
					Role:      "assistant",
					Content:   output,
				}
				if _, storeErr := s.repo.CreateMessage(bgCtx, assistantMsg); storeErr != nil {
					slog.Warn("chat service: failed to store assistant message", "sessionId", sessionID, "err", storeErr)
				}
			}()
		}
	}

	return MessageResponseFrom(created), nil
}
