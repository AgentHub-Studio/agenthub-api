package chat_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockChatSvc satisfies the private chatService interface in chat.Handler.
type mockChatSvc struct {
	sessions map[uuid.UUID]chat.ChatSession
	messages map[uuid.UUID][]chat.ChatMessage
}

func newMockChatSvc() *mockChatSvc {
	return &mockChatSvc{
		sessions: make(map[uuid.UUID]chat.ChatSession),
		messages: make(map[uuid.UUID][]chat.ChatMessage),
	}
}

func (m *mockChatSvc) ListSessions(_ context.Context, req pagination.PageRequest) (pagination.Page[chat.ChatSessionResponse], error) {
	items := make([]chat.ChatSessionResponse, 0, len(m.sessions))
	for _, s := range m.sessions {
		items = append(items, chat.SessionResponseFrom(s))
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockChatSvc) CreateSession(_ context.Context, req chat.CreateSessionRequest) (chat.ChatSessionResponse, error) {
	id := uuid.New()
	s := chat.ChatSession{
		ID:      id,
		AgentID: req.AgentID,
		Title:   req.Title,
		Status:  chat.StatusActive,
	}
	m.sessions[id] = s
	return chat.SessionResponseFrom(s), nil
}

func (m *mockChatSvc) GetSession(_ context.Context, id uuid.UUID) (chat.ChatSessionResponse, error) {
	s, ok := m.sessions[id]
	if !ok {
		return chat.ChatSessionResponse{}, chat.ErrNotFound
	}
	return chat.SessionResponseFrom(s), nil
}

func (m *mockChatSvc) DeleteSession(_ context.Context, id uuid.UUID) error {
	if _, ok := m.sessions[id]; !ok {
		return chat.ErrNotFound
	}
	delete(m.sessions, id)
	return nil
}

func (m *mockChatSvc) ArchiveSession(_ context.Context, id uuid.UUID) (chat.ChatSessionResponse, error) {
	s, ok := m.sessions[id]
	if !ok {
		return chat.ChatSessionResponse{}, chat.ErrNotFound
	}
	s.Status = chat.StatusArchived
	m.sessions[id] = s
	return chat.SessionResponseFrom(s), nil
}

func (m *mockChatSvc) ListMessages(_ context.Context, sessionID uuid.UUID, req pagination.PageRequest) (pagination.Page[chat.ChatMessageResponse], error) {
	msgs := m.messages[sessionID]
	items := make([]chat.ChatMessageResponse, len(msgs))
	for i, msg := range msgs {
		items[i] = chat.MessageResponseFrom(msg)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockChatSvc) AddMessage(_ context.Context, sessionID uuid.UUID, req chat.CreateMessageRequest) (chat.ChatMessageResponse, error) {
	msg := chat.ChatMessage{
		ID:        uuid.New(),
		SessionID: sessionID,
		Role:      req.Role,
		Content:   req.Content,
	}
	m.messages[sessionID] = append(m.messages[sessionID], msg)
	return chat.MessageResponseFrom(msg), nil
}

func setupChat() (*chi.Mux, *mockChatSvc) {
	svc := newMockChatSvc()
	h := chat.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestChatHandler_ListSessions_Success(t *testing.T) {
	r, svc := setupChat()
	id := uuid.New()
	svc.sessions[id] = chat.ChatSession{ID: id, AgentID: uuid.New(), Title: "Session 1", Status: chat.StatusActive}

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[chat.ChatSessionResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestChatHandler_CreateSession_Success(t *testing.T) {
	r, _ := setupChat()
	body, _ := json.Marshal(chat.CreateSessionRequest{AgentID: uuid.New(), Title: "My Chat"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp chat.ChatSessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "My Chat", resp.Title)
}

func TestChatHandler_CreateSession_InvalidBody(t *testing.T) {
	r, _ := setupChat()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChatHandler_GetSession_NotFound(t *testing.T) {
	r, _ := setupChat()
	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestChatHandler_DeleteSession_Success(t *testing.T) {
	r, svc := setupChat()
	id := uuid.New()
	svc.sessions[id] = chat.ChatSession{ID: id, AgentID: uuid.New(), Title: "To Delete", Status: chat.StatusActive}

	req := httptest.NewRequest(http.MethodDelete, "/api/chat/sessions/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestChatHandler_DeleteSession_NotFound(t *testing.T) {
	r, _ := setupChat()
	req := httptest.NewRequest(http.MethodDelete, "/api/chat/sessions/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestChatHandler_AddMessage_Success(t *testing.T) {
	r, svc := setupChat()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Chat", Status: chat.StatusActive}

	body, _ := json.Marshal(chat.CreateMessageRequest{Role: "user", Content: "Hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestChatHandler_ListMessages_Success(t *testing.T) {
	r, svc := setupChat()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Chat", Status: chat.StatusActive}

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/messages", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
