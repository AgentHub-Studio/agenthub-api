package chat_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockChatRepo struct {
	sessions map[uuid.UUID]chat.ChatSession
	messages []chat.ChatMessage
}

func uuidPtr() *uuid.UUID { id := uuid.New(); return &id }

func newMockRepo() *mockChatRepo {
	return &mockChatRepo{sessions: make(map[uuid.UUID]chat.ChatSession)}
}

func (m *mockChatRepo) FindSessions(_ context.Context, _ pagination.PageRequest) ([]chat.ChatSession, int64, error) {
	out := make([]chat.ChatSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out, int64(len(out)), nil
}

func (m *mockChatRepo) GetSessionByID(_ context.Context, id uuid.UUID) (chat.ChatSession, error) {
	s, ok := m.sessions[id]
	if !ok {
		return chat.ChatSession{}, chat.ErrNotFound
	}
	return s, nil
}

func (m *mockChatRepo) CreateSession(_ context.Context, s chat.ChatSession) (chat.ChatSession, error) {
	s.ID = uuid.New()
	s.Status = chat.StatusActive
	m.sessions[s.ID] = s
	return s, nil
}

func (m *mockChatRepo) UpdateSessionStatus(_ context.Context, id uuid.UUID, status chat.ChatStatus) (chat.ChatSession, error) {
	s, ok := m.sessions[id]
	if !ok {
		return chat.ChatSession{}, chat.ErrNotFound
	}
	s.Status = status
	m.sessions[id] = s
	return s, nil
}

func (m *mockChatRepo) DeleteSession(_ context.Context, id uuid.UUID) error {
	if _, ok := m.sessions[id]; !ok {
		return chat.ErrNotFound
	}
	delete(m.sessions, id)
	return nil
}

func (m *mockChatRepo) FindMessages(_ context.Context, sessionID uuid.UUID, _ pagination.PageRequest) ([]chat.ChatMessage, int64, error) {
	var out []chat.ChatMessage
	for _, msg := range m.messages {
		if msg.SessionID == sessionID {
			out = append(out, msg)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockChatRepo) CreateMessage(_ context.Context, msg chat.ChatMessage) (chat.ChatMessage, error) {
	msg.ID = uuid.New()
	m.messages = append(m.messages, msg)
	return msg, nil
}

func (m *mockChatRepo) GetLatestAssistantMessage(_ context.Context, sessionID uuid.UUID, after time.Time) (chat.ChatMessage, bool, error) {
	for _, msg := range m.messages {
		if msg.SessionID == sessionID && msg.Role == "assistant" && msg.CreatedAt.After(after) {
			return msg, true, nil
		}
	}
	return chat.ChatMessage{}, false, nil
}

func (m *mockChatRepo) GetLatestCompactSummary(_ context.Context, sessionID uuid.UUID) (chat.ChatMessage, bool, error) {
	for i := len(m.messages) - 1; i >= 0; i-- {
		msg := m.messages[i]
		if msg.SessionID == sessionID && msg.MessageType == chat.MessageTypeCompactSummary {
			return msg, true, nil
		}
	}
	return chat.ChatMessage{}, false, nil
}

func TestChatService_CreateSession_Success(t *testing.T) {
	svc := chat.NewService(newMockRepo(), nil)
	agentID := uuid.New()
	s, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "Support Chat",
	})
	require.NoError(t, err)
	assert.Equal(t, "Support Chat", s.Title)
	assert.NotEqual(t, uuid.Nil, s.ID)
	assert.Equal(t, chat.StatusActive, s.Status)
}

func TestChatService_GetSession_NotFound(t *testing.T) {
	svc := chat.NewService(newMockRepo(), nil)
	_, err := svc.GetSession(context.Background(), uuid.New())
	require.ErrorIs(t, err, chat.ErrNotFound)
}

func TestChatService_ArchiveSession(t *testing.T) {
	svc := chat.NewService(newMockRepo(), nil)
	created, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{AgentID: uuidPtr(), Title:"test"})
	require.NoError(t, err)
	archived, err := svc.ArchiveSession(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, chat.StatusArchived, archived.Status)
}

func TestChatService_AddMessage(t *testing.T) {
	svc := chat.NewService(newMockRepo(), nil)
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{AgentID: uuidPtr(), Title:"q&a"})
	require.NoError(t, err)
	msg, err := svc.AddMessage(context.Background(), nil, session.ID, chat.CreateMessageRequest{
		Role:    "user",
		Content: "Hello!",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello!", msg.Content)
	assert.NotEqual(t, uuid.Nil, msg.ID)
}

func TestChatService_ListMessages(t *testing.T) {
	svc := chat.NewService(newMockRepo(), nil)
	session, err := svc.CreateSession(context.Background(), chat.CreateSessionRequest{AgentID: uuidPtr(), Title:"q&a"})
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err = svc.AddMessage(context.Background(), nil, session.ID, chat.CreateMessageRequest{Role: "user", Content: "msg"})
		require.NoError(t, err)
	}
	page, err := svc.ListMessages(context.Background(), session.ID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
	assert.Len(t, page.Content, 3)
}
