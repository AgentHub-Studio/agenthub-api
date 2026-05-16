package chat_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/task"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockChatSvc satisfies the private chatService interface in chat.Handler.
type mockChatSvc struct {
	sessions               map[uuid.UUID]chat.ChatSession
	messages               map[uuid.UUID][]chat.ChatMessage
	runEvents              []chat.RunEvent
	lastClientState        chat.ClientStatePatch
	lastClientStateSession uuid.UUID
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

func (m *mockChatSvc) RenameSession(_ context.Context, id uuid.UUID, title string) (chat.ChatSessionResponse, error) {
	s, ok := m.sessions[id]
	if !ok {
		return chat.ChatSessionResponse{}, chat.ErrNotFound
	}
	s.Title = title
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

func (m *mockChatSvc) RunSession(_ context.Context, sessionID uuid.UUID, userMessage, tenantID string) (<-chan chat.RunEvent, error) {
	if _, ok := m.sessions[sessionID]; !ok {
		return nil, chat.ErrNotFound
	}
	ch := make(chan chat.RunEvent, 10)
	go func() {
		defer close(ch)
		if len(m.runEvents) > 0 {
			for _, ev := range m.runEvents {
				ch <- ev
			}
			return
		}
		ch <- chat.RunEvent{Type: "text_delta", Data: json.RawMessage(`{"content":"Hello from agent"}`)}
		ch <- chat.RunEvent{Type: "run_complete", Data: json.RawMessage(`{"totalTurns":1,"totalTokens":50}`)}
	}()
	return ch, nil
}

func (m *mockChatSvc) GetActiveRun(_ context.Context, _ uuid.UUID) (chat.ChatRunResponse, bool, error) {
	return chat.ChatRunResponse{}, false, nil
}

func (m *mockChatSvc) RespondElicitation(sessionID, requestID string, result chat.ElicitationResult) bool {
	return false // no active runs in tests
}

func (m *mockChatSvc) ApplyClientState(sessionID uuid.UUID, patch chat.ClientStatePatch) {
	m.lastClientState = patch
	m.lastClientStateSession = sessionID
}

func setupChat() (*chi.Mux, *mockChatSvc) {
	svc := newMockChatSvc()
	h := chat.NewHandler(svc, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestChatHandler_ListSessions_Success(t *testing.T) {
	r, svc := setupChat()
	id := uuid.New()
	agentID := uuid.New()
	svc.sessions[id] = chat.ChatSession{ID: id, AgentID: &agentID, Title: "Session 1", Status: chat.StatusActive}

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
	agentID := uuid.New()
	body, _ := json.Marshal(chat.CreateSessionRequest{AgentID: &agentID, Title: "My Chat"})
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
	delAgentID := uuid.New()
	svc.sessions[id] = chat.ChatSession{ID: id, AgentID: &delAgentID, Title: "To Delete", Status: chat.StatusActive}

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

func TestChatHandler_ArchiveSession_Success(t *testing.T) {
	r, svc := setupChat()
	id := uuid.New()
	svc.sessions[id] = chat.ChatSession{ID: id, Title: "Active Chat", Status: chat.StatusActive}

	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+id.String()+"/archive", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp chat.ChatSessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, chat.StatusArchived, resp.Status)
}

func TestChatHandler_ArchiveSession_NotFound(t *testing.T) {
	r, _ := setupChat()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+uuid.New().String()+"/archive", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestChatHandler_ArchiveSession_InvalidID(t *testing.T) {
	r, _ := setupChat()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/not-a-uuid/archive", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- POST /api/chat/sessions/{id}/run (SSE) ---

func TestChatHandler_RunSession_Success(t *testing.T) {
	r, svc := setupChat()

	// Create a session with an agent.
	agentID := uuid.New()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, AgentID: &agentID, Title: "test", Status: chat.StatusActive}

	body, _ := json.Marshal(map[string]string{"message": "Hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))

	// X-Run-ID header must be present for reconnection.
	runID := w.Header().Get("X-Run-ID")
	assert.NotEmpty(t, runID, "X-Run-ID header must be set")

	// Parse SSE events from response body.
	responseBody := w.Body.String()
	assert.Contains(t, responseBody, "event: text_delta\n")
	assert.Contains(t, responseBody, "event: run_complete\n")
	assert.Contains(t, responseBody, `"content":"Hello from agent"`)

	// SSE events must include id fields.
	assert.Contains(t, responseBody, "id: "+runID+":1\n")
	assert.Contains(t, responseBody, "id: "+runID+":2\n")
}

func TestChatHandler_RunSession_SessionNotFound(t *testing.T) {
	r, _ := setupChat()
	body, _ := json.Marshal(map[string]string{"message": "Hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+uuid.New().String()+"/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestChatHandler_RunSession_EmptyMessage(t *testing.T) {
	r, _ := setupChat()
	body, _ := json.Marshal(map[string]string{"message": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+uuid.New().String()+"/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChatHandler_RunSession_InvalidBody(t *testing.T) {
	r, _ := setupChat()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+uuid.New().String()+"/run", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChatHandler_RunSession_InvalidSessionID(t *testing.T) {
	r, _ := setupChat()
	body, _ := json.Marshal(map[string]string{"message": "Hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/not-a-uuid/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- GET /api/chat/sessions/{id}/run/{runId}/resume (SSE reconnection) ---

func TestChatHandler_ResumeSession_ReplayAll(t *testing.T) {
	r, svc := setupChat()

	agentID := uuid.New()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, AgentID: &agentID, Title: "test", Status: chat.StatusActive}

	// First, run a session to populate the buffer.
	body, _ := json.Marshal(map[string]string{"message": "Hello"})
	runReq := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/run", bytes.NewReader(body))
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()
	r.ServeHTTP(runW, runReq)

	require.Equal(t, http.StatusOK, runW.Code)
	runID := runW.Header().Get("X-Run-ID")
	require.NotEmpty(t, runID)

	// Resume with no Last-Event-ID — should replay all events.
	resumeCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	resumeReq := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/run/"+runID+"/resume", nil).WithContext(resumeCtx)
	resumeW := httptest.NewRecorder()
	r.ServeHTTP(resumeW, resumeReq)

	assert.Equal(t, http.StatusOK, resumeW.Code)
	assert.Equal(t, "text/event-stream", resumeW.Header().Get("Content-Type"))

	resumeBody := resumeW.Body.String()
	assert.Contains(t, resumeBody, "event: text_delta\n")
	assert.Contains(t, resumeBody, "event: run_complete\n")
	assert.Contains(t, resumeBody, "id: "+runID+":1\n")
	assert.Contains(t, resumeBody, "id: "+runID+":2\n")
}

func TestChatHandler_ResumeSession_ReplayPartial(t *testing.T) {
	r, svc := setupChat()

	agentID := uuid.New()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, AgentID: &agentID, Title: "test", Status: chat.StatusActive}

	body, _ := json.Marshal(map[string]string{"message": "Hello"})
	runReq := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/run", bytes.NewReader(body))
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()
	r.ServeHTTP(runW, runReq)

	require.Equal(t, http.StatusOK, runW.Code)
	runID := runW.Header().Get("X-Run-ID")

	// Resume with Last-Event-ID = 1 — should only get event 2.
	resumeReq := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/run/"+runID+"/resume", nil)
	resumeReq.Header.Set("Last-Event-ID", runID+":1")
	resumeW := httptest.NewRecorder()
	r.ServeHTTP(resumeW, resumeReq)

	assert.Equal(t, http.StatusOK, resumeW.Code)
	resumeBody := resumeW.Body.String()
	assert.NotContains(t, resumeBody, "id: "+runID+":1\n")
	assert.Contains(t, resumeBody, "id: "+runID+":2\n")
}

func TestChatHandler_ResumeSession_QueryParamLastEventID(t *testing.T) {
	r, svc := setupChat()

	agentID := uuid.New()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, AgentID: &agentID, Title: "test", Status: chat.StatusActive}

	body, _ := json.Marshal(map[string]string{"message": "Hello"})
	runReq := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/run", bytes.NewReader(body))
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()
	r.ServeHTTP(runW, runReq)

	runID := runW.Header().Get("X-Run-ID")

	// Use query param instead of header.
	resumeReq := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/run/"+runID+"/resume?lastEventId="+runID+":1", nil)
	resumeW := httptest.NewRecorder()
	r.ServeHTTP(resumeW, resumeReq)

	assert.Equal(t, http.StatusOK, resumeW.Code)
	resumeBody := resumeW.Body.String()
	assert.NotContains(t, resumeBody, "id: "+runID+":1\n")
	assert.Contains(t, resumeBody, "id: "+runID+":2\n")
}

func TestChatHandler_ResumeSession_FiltersResolvedInputRequest(t *testing.T) {
	t.Skip("covered by internal package test for filterReplayableEvents")
}

func TestChatHandler_ResumeSession_RunNotFound(t *testing.T) {
	r, _ := setupChat()
	sessionID := uuid.New()
	resumeReq := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/run/nonexistent-run/resume", nil)
	resumeW := httptest.NewRecorder()
	r.ServeHTTP(resumeW, resumeReq)

	assert.Equal(t, http.StatusNotFound, resumeW.Code)
}

func TestChatHandler_ResumeSession_InvalidSessionID(t *testing.T) {
	r, _ := setupChat()
	resumeReq := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/not-a-uuid/run/some-run/resume", nil)
	resumeW := httptest.NewRecorder()
	r.ServeHTTP(resumeW, resumeReq)

	assert.Equal(t, http.StatusBadRequest, resumeW.Code)
}

// --- GET /api/chat/runs/{id} (TR-01-TASK-27, P-C325-1) ---

// mockRunLookup satisfies chat.RunLookup for unit tests.
type mockRunLookup struct {
	runs map[uuid.UUID]chat.ChatRun
}

func (m *mockRunLookup) GetRunByID(_ context.Context, id uuid.UUID) (chat.ChatRun, error) {
	if r, ok := m.runs[id]; ok {
		return r, nil
	}
	return chat.ChatRun{}, chat.ErrNotFound
}

func setupChatWithRuns(rl chat.RunLookup) (*chi.Mux, *mockChatSvc) {
	svc := newMockChatSvc()
	h := chat.NewHandler(svc, nil).WithRunLookup(rl)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestGetRun_Success(t *testing.T) {
	runID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	rl := &mockRunLookup{runs: map[uuid.UUID]chat.ChatRun{
		runID: {
			ID:        runID,
			SessionID: sessionID,
			Status:    chat.ChatRunStatusCompleted,
			StartedAt: &now,
		},
	}}
	r, _ := setupChatWithRuns(rl)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/runs/"+runID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp chat.ChatRunResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, runID, resp.ID)
	assert.Equal(t, sessionID, resp.SessionID)
	assert.Equal(t, chat.ChatRunStatusCompleted, resp.Status)
}

func TestGetRun_NotFound(t *testing.T) {
	rl := &mockRunLookup{runs: map[uuid.UUID]chat.ChatRun{}}
	r, _ := setupChatWithRuns(rl)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/runs/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetRun_InvalidID(t *testing.T) {
	rl := &mockRunLookup{runs: map[uuid.UUID]chat.ChatRun{}}
	r, _ := setupChatWithRuns(rl)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/runs/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetRun_NoLookupConfigured(t *testing.T) {
	// Handler created without any executor or runLookup.
	svc := newMockChatSvc()
	h := chat.NewHandler(svc, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/runs/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- IMPROVEMENT-TASK-03: task list endpoints ---

// mockHandlerTaskRepo is a simple in-memory task.Repository for handler tests.
type mockHandlerTaskRepo struct {
	tasks []task.Task
}

func (m *mockHandlerTaskRepo) CreateTask(_ context.Context, t task.Task) error {
	m.tasks = append(m.tasks, t)
	return nil
}
func (m *mockHandlerTaskRepo) UpdateTask(_ context.Context, _ task.Task) error { return nil }
func (m *mockHandlerTaskRepo) GetTask(_ context.Context, _ string) (task.Task, error) {
	return task.Task{}, nil
}
func (m *mockHandlerTaskRepo) ListBySession(_ context.Context, _ uuid.UUID, req pagination.PageRequest) ([]task.Task, int64, error) {
	return m.tasks, int64(len(m.tasks)), nil
}
func (m *mockHandlerTaskRepo) CreateNotification(_ context.Context, _ task.Notification) error {
	return nil
}
func (m *mockHandlerTaskRepo) ListNotificationsByTask(_ context.Context, _ string) ([]task.Notification, error) {
	return nil, nil
}

var _ task.Repository = (*mockHandlerTaskRepo)(nil)

func setupChatWithTasks(repo task.Repository) (*chi.Mux, *mockChatSvc) {
	svc := newMockChatSvc()
	h := chat.NewHandler(svc, nil).WithTaskRepository(repo)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestHandler_GetTasks_200(t *testing.T) {
	sessionID := uuid.New()
	repo := &mockHandlerTaskRepo{
		tasks: []task.Task{
			{ID: "task-1", SessionID: sessionID, Status: "pending", Phase: "research"},
			{ID: "task-2", SessionID: sessionID, Status: "completed", Phase: "implementation"},
		},
	}
	r, _ := setupChatWithTasks(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/tasks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[task.Task]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(2), page.TotalElements)
}

func TestHandler_GetTasks_InvalidSessionID_Returns400(t *testing.T) {
	repo := &mockHandlerTaskRepo{}
	r, _ := setupChatWithTasks(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/not-a-uuid/tasks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_GetTasks_NoRepo_Returns501(t *testing.T) {
	r, _ := setupChat() // handler without task repo

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+uuid.New().String()+"/tasks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestHandler_GetTaskNotifications_200(t *testing.T) {
	sessionID := uuid.New()
	repo := &mockHandlerTaskRepo{}
	r, _ := setupChatWithTasks(repo)

	url := "/api/chat/sessions/" + sessionID.String() + "/tasks/task-1/notifications"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- CopilotKit Phase 1: POST /client-state ----------------------------------

func TestHandler_ClientState_204_ForwardsFullPatch(t *testing.T) {
	r, svc := setupChat()

	sessionID := uuid.New()
	body := bytes.NewBufferString(`{
		"frontendActions":[{"name":"navigate_to","description":"d"}],
		"readables":[{"id":"route","description":"d","value":"/x"}],
		"actionResults":[{"id":"c-1","status":"ok","result":{"navigated":true}}]
	}`)

	req := httptest.NewRequest(http.MethodPost,
		"/api/chat/sessions/"+sessionID.String()+"/client-state", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, sessionID, svc.lastClientStateSession)
	require.Len(t, svc.lastClientState.FrontendActions, 1)
	assert.Equal(t, "navigate_to", svc.lastClientState.FrontendActions[0].Name)
	require.Len(t, svc.lastClientState.Readables, 1)
	assert.Equal(t, "route", svc.lastClientState.Readables[0].ID)
	require.Len(t, svc.lastClientState.ActionResults, 1)
	assert.Equal(t, "c-1", svc.lastClientState.ActionResults[0].ID)
	assert.Equal(t, "ok", svc.lastClientState.ActionResults[0].Status)
}

func TestHandler_ClientState_400_InvalidSessionID(t *testing.T) {
	r, _ := setupChat()

	req := httptest.NewRequest(http.MethodPost,
		"/api/chat/sessions/not-a-uuid/client-state",
		bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ClientState_400_InvalidJSON(t *testing.T) {
	r, _ := setupChat()

	req := httptest.NewRequest(http.MethodPost,
		"/api/chat/sessions/"+uuid.New().String()+"/client-state",
		bytes.NewBufferString(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ClientState_204_EmptyBodyIsOK(t *testing.T) {
	r, svc := setupChat()

	sessionID := uuid.New()
	req := httptest.NewRequest(http.MethodPost,
		"/api/chat/sessions/"+sessionID.String()+"/client-state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, sessionID, svc.lastClientStateSession)
}
