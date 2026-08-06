package chat_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	runEventCh             <-chan chat.RunEvent
	runSessionErr          error
	runCalls               int
	lastRunOptions         []chat.RunSessionOptions
	cloneCalls             int
	lastClientState        chat.ClientStatePatch
	lastClientStateSession uuid.UUID
	clientStateCalls       int
	elicitationCalls       int
	elicitationOK          bool
	lastElicitationResult  chat.ElicitationResult
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

func (m *mockChatSvc) CloneSession(_ context.Context, sessionID uuid.UUID, req chat.CloneSessionRequest) (chat.ChatSessionResponse, error) {
	m.cloneCalls++
	source, ok := m.sessions[sessionID]
	if !ok {
		return chat.ChatSessionResponse{}, chat.ErrNotFound
	}

	title := req.Title
	if title == "" {
		title = "Copy of " + source.Title
	}
	cloneID := uuid.New()
	sourceID := source.ID
	sourceTitle := source.Title
	clone := chat.ChatSession{
		ID:                     cloneID,
		AgentID:                source.AgentID,
		Title:                  title,
		Status:                 chat.StatusActive,
		ClonedFromSessionID:    &sourceID,
		ClonedFromSessionTitle: &sourceTitle,
	}
	m.sessions[cloneID] = clone

	for _, msg := range m.messages[sessionID] {
		copied := msg
		copied.ID = uuid.New()
		copied.SessionID = cloneID
		copied.RunID = nil
		m.messages[cloneID] = append(m.messages[cloneID], copied)
		if req.UntilMessageID != nil && msg.ID == *req.UntilMessageID {
			return chat.SessionResponseFrom(clone), nil
		}
	}
	if req.UntilMessageID != nil {
		return chat.ChatSessionResponse{}, chat.ErrMessageNotFound
	}

	return chat.SessionResponseFrom(clone), nil
}

func (m *mockChatSvc) RunSession(_ context.Context, sessionID uuid.UUID, userMessage, tenantID string, opts ...chat.RunSessionOptions) (<-chan chat.RunEvent, error) {
	m.runCalls++
	m.lastRunOptions = opts
	if _, ok := m.sessions[sessionID]; !ok {
		return nil, chat.ErrNotFound
	}
	if m.runSessionErr != nil {
		return nil, m.runSessionErr
	}
	if m.runEventCh != nil {
		return m.runEventCh, nil
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

func readSSEBlock(t *testing.T, r *bufio.Reader) string {
	t.Helper()

	var b strings.Builder
	for {
		line, err := r.ReadString('\n')
		require.NoError(t, err)
		if strings.TrimSpace(line) == "" {
			return b.String()
		}
		b.WriteString(line)
	}
}

func sseIDFromBlock(t *testing.T, block string) string {
	t.Helper()

	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(line, "id: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "id: "))
		}
	}
	t.Fatalf("SSE block has no id: %q", block)
	return ""
}

func (m *mockChatSvc) GetActiveRun(_ context.Context, _ uuid.UUID) (chat.ChatRunResponse, bool, error) {
	return chat.ChatRunResponse{}, false, nil
}

func (m *mockChatSvc) RespondElicitation(_ context.Context, sessionID, requestID string, result chat.ElicitationResult) bool {
	m.elicitationCalls++
	m.lastElicitationResult = result
	return m.elicitationOK
}

func (m *mockChatSvc) ApplyClientState(sessionID uuid.UUID, patch chat.ClientStatePatch) {
	m.clientStateCalls++
	m.lastClientState = patch
	m.lastClientStateSession = sessionID
}

type mockVoiceSvc struct {
	input         chat.VoiceTranscriptionInput
	synthesis     chat.VoiceSynthesisInput
	transcribeErr error
	synthesizeErr error
}

func (m *mockVoiceSvc) Transcribe(_ context.Context, in chat.VoiceTranscriptionInput) (chat.VoiceTranscription, error) {
	m.input = in
	if m.transcribeErr != nil {
		return chat.VoiceTranscription{}, m.transcribeErr
	}
	return chat.VoiceTranscription{Text: "abrir dashboard", Language: "pt-BR", Confidence: 0.95}, nil
}

func (m *mockVoiceSvc) Synthesize(_ context.Context, in chat.VoiceSynthesisInput) (chat.VoiceAudio, error) {
	m.synthesis = in
	if m.synthesizeErr != nil {
		return chat.VoiceAudio{}, m.synthesizeErr
	}
	return chat.VoiceAudio{Format: "mp3", Base64: "YXVkaW8="}, nil
}

type mockEffectivePromptInspector struct {
	identity chat.PromptIdentity
	resp     chat.EffectivePromptResponse
	err      error
}

func (m *mockEffectivePromptInspector) EffectivePrompt(_ context.Context, _ uuid.UUID, identity chat.PromptIdentity) (chat.EffectivePromptResponse, error) {
	m.identity = identity
	return m.resp, m.err
}

type mockPermissionAuditReader struct {
	called    bool
	sessionID uuid.UUID
	limit     int
	entries   []chat.PermissionAuditEntryResponse
	err       error
}

func (m *mockPermissionAuditReader) ListBySession(_ context.Context, sessionID uuid.UUID, limit int) ([]chat.PermissionAuditEntryResponse, error) {
	m.called = true
	m.sessionID = sessionID
	m.limit = limit
	return m.entries, m.err
}

func setupChat() (*chi.Mux, *mockChatSvc) {
	svc := newMockChatSvc()
	h := chat.NewHandler(svc, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func setupChatWithVoice() (*chi.Mux, *mockChatSvc, *mockVoiceSvc) {
	svc := newMockChatSvc()
	voice := &mockVoiceSvc{}
	h := chat.NewHandler(svc, nil).WithVoiceService(voice)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc, voice
}

func setupChatWithEffectivePrompt(inspector *mockEffectivePromptInspector) (*chi.Mux, *mockChatSvc) {
	svc := newMockChatSvc()
	h := chat.NewHandler(svc, nil).WithEffectivePromptInspector(inspector, func(*http.Request) chat.PromptIdentity {
		return chat.PromptIdentity{
			UserEmail:  "ana@example.test",
			Roles:      []string{"admin"},
			TenantID:   "test",
			TenantName: "Test Tenant",
		}
	})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func setupChatWithPermissionAudit(reader chat.PermissionAuditReader) (*chi.Mux, *mockChatSvc) {
	svc := newMockChatSvc()
	h := chat.NewHandler(svc, nil).WithPermissionAuditReader(reader)
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

func TestChatHandler_VoiceInputProviderNotConfigured(t *testing.T) {
	r, svc := setupChat()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Voice", Status: chat.StatusActive}

	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/voice/input", bytes.NewReader([]byte("audio")))
	req.Header.Set("Content-Type", "audio/wav")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestChatHandler_VoiceInputTranscribesMultipartAudio(t *testing.T) {
	r, svc, voice := setupChatWithVoice()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Voice", Status: chat.StatusActive}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("audio", "hello.wav")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake wav"))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("language", "pt-BR"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/voice/input?run=false", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello.wav", voice.input.Filename)
	assert.Equal(t, "pt-BR", voice.input.Language)
	assert.Equal(t, []byte("fake wav"), voice.input.Audio)
	var resp chat.VoiceInputResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "transcribed", resp.Status)
	assert.Equal(t, "abrir dashboard", resp.Transcription.Text)
	assert.Nil(t, resp.RunID)
}

func TestChatHandler_VoiceInputTranscriptionFailureDoesNotLeakInfrastructureDetail(t *testing.T) {
	r, svc, voice := setupChatWithVoice()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Voice", Status: chat.StatusActive}
	voice.transcribeErr = errors.New("post https://voice.internal.example/v1/audio: dial tcp 10.42.0.19:443: connection refused")

	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/voice/input?run=false", bytes.NewReader([]byte("fake wav")))
	req.Header.Set("Content-Type", "audio/wav")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "voice transcription is temporarily unavailable")
	assert.NotContains(t, w.Body.String(), "voice.internal.example")
	assert.NotContains(t, w.Body.String(), "10.42.0.19")
	assert.NotContains(t, w.Body.String(), "connection refused")
}

func TestChatHandler_VoiceInputRunsAndSynthesizesWithoutAsyncExecutor(t *testing.T) {
	r, svc, voice := setupChatWithVoice()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Voice", Status: chat.StatusActive}

	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/voice/input", bytes.NewReader([]byte("fake wav")))
	req.Header.Set("Content-Type", "audio/wav")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp chat.VoiceInputResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.RunID)
	require.NotNil(t, resp.Audio)
	assert.Equal(t, "completed", resp.Status)
	assert.Equal(t, "mp3", resp.Audio.Format)
	assert.Equal(t, "YXVkaW8=", resp.Audio.Base64)
	assert.Equal(t, []byte("fake wav"), voice.input.Audio)
	assert.Equal(t, "Hello from agent", voice.synthesis.Text)
}

func TestChatHandler_VoiceInputSynchronousRunFailureDoesNotLeakInfrastructureDetail(t *testing.T) {
	r, svc, _ := setupChatWithVoice()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Voice", Status: chat.StatusActive}
	svc.runSessionErr = errors.New("provider https://llm.internal.example: dial tcp 10.42.0.20:443: connection refused")

	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/voice/input", bytes.NewReader([]byte("fake wav")))
	req.Header.Set("Content-Type", "audio/wav")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "failed to process voice run")
	assert.NotContains(t, w.Body.String(), "llm.internal.example")
	assert.NotContains(t, w.Body.String(), "10.42.0.20")
	assert.NotContains(t, w.Body.String(), "connection refused")
}

func TestChatHandler_RunSessionSynchronousFailureDoesNotLeakInfrastructureDetail(t *testing.T) {
	r, svc := setupChat()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Chat", Status: chat.StatusActive}
	svc.runSessionErr = errors.New("provider https://llm.internal.example: dial tcp 10.42.0.21:443: connection refused")

	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/run", strings.NewReader(`{"message":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "failed to start chat run")
	assert.NotContains(t, w.Body.String(), "llm.internal.example")
	assert.NotContains(t, w.Body.String(), "10.42.0.21")
	assert.NotContains(t, w.Body.String(), "connection refused")
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

func TestChatHandler_GetSession_IncludesLegacyBackgroundActiveRun(t *testing.T) {
	r, svc := setupChat()
	server := httptest.NewServer(r)
	defer server.Close()

	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Reconnect", Status: chat.StatusActive}
	runEvents := make(chan chat.RunEvent)
	svc.runEventCh = runEvents

	runReq, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/api/chat/sessions/"+sessionID.String()+"/run",
		strings.NewReader(`{"message":"start a slow run"}`),
	)
	require.NoError(t, err)
	runReq.Header.Set("Accept", "text/event-stream")
	runReq.Header.Set("Content-Type", "application/json")

	runRespCh := make(chan *http.Response, 1)
	runErrCh := make(chan error, 1)
	go func() {
		resp, err := server.Client().Do(runReq)
		if err != nil {
			runErrCh <- err
			return
		}
		runRespCh <- resp
	}()

	runEvents <- chat.RunEvent{Type: "text_delta", Data: json.RawMessage(`{"content":"partial"}`)}

	var runResp *http.Response
	select {
	case err := <-runErrCh:
		require.NoError(t, err)
	case runResp = <-runRespCh:
	case <-time.After(3 * time.Second):
		t.Fatal("POST /run did not start streaming")
	}
	defer func() { _ = runResp.Body.Close() }()
	defer close(runEvents)
	require.Equal(t, http.StatusOK, runResp.StatusCode)
	runID := runResp.Header.Get("X-Run-ID")
	require.NotEmpty(t, runID)

	getResp, err := server.Client().Get(server.URL + "/api/chat/sessions/" + sessionID.String())
	require.NoError(t, err)
	defer func() { _ = getResp.Body.Close() }()
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&body))
	activeRun, ok := body["activeRun"].(map[string]any)
	require.True(t, ok, "GET session must expose the legacy in-memory active run")
	assert.Equal(t, runID, activeRun["id"])
	assert.Equal(t, sessionID.String(), activeRun["sessionId"])
	assert.Equal(t, "active", activeRun["status"])
}

func TestChatHandler_EffectivePrompt_Success(t *testing.T) {
	sessionID := uuid.New()
	agentID := uuid.New()
	inspector := &mockEffectivePromptInspector{resp: chat.EffectivePromptResponse{
		SessionID:    sessionID,
		AgentID:      &agentID,
		SystemPrompt: "Olá ana@example.test",
		Warnings:     []string{},
	}}
	r, _ := setupChatWithEffectivePrompt(inspector)
	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/effective-prompt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ana@example.test", inspector.identity.UserEmail)
	assert.Equal(t, []string{"admin"}, inspector.identity.Roles)
	var resp chat.EffectivePromptResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, sessionID, resp.SessionID)
	assert.Equal(t, "Olá ana@example.test", resp.SystemPrompt)
	assert.Empty(t, resp.Warnings)
}

func TestChatHandler_PermissionAudit_Success(t *testing.T) {
	sessionID := uuid.New()
	runID := uuid.New()
	runIDString := runID.String()
	createdAt := time.Date(2026, 6, 22, 16, 30, 0, 0, time.UTC).Format(time.RFC3339)
	reader := &mockPermissionAuditReader{entries: []chat.PermissionAuditEntryResponse{
		{
			SessionID:    sessionID.String(),
			RunID:        &runIDString,
			ToolName:     "execute-sql",
			Decision:     "confirm_denied",
			MatchedRule:  "execute-*",
			InputSnippet: `{"query":"DROP TABLE users"}`,
			CreatedAt:    createdAt,
		},
		{
			SessionID: sessionID.String(),
			ToolName:  "execute-sql",
			Decision:  "confirm_approved",
			CreatedAt: createdAt,
		},
	}}
	r, svc := setupChatWithPermissionAudit(reader)
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Audit", Status: chat.StatusActive}

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/permission-audit", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, reader.called)
	assert.Equal(t, sessionID, reader.sessionID)
	assert.Equal(t, 0, reader.limit)

	var resp []chat.PermissionAuditEntryResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp, 2)
	assert.Equal(t, "execute-sql", resp[0].ToolName)
	assert.Equal(t, "confirm_denied", resp[0].Decision)
	require.NotNil(t, resp[0].RunID)
	assert.Equal(t, runID.String(), *resp[0].RunID)
	assert.Equal(t, "execute-*", resp[0].MatchedRule)
	assert.Contains(t, resp[0].InputSnippet, "DROP TABLE users")
	assert.Equal(t, "confirm_approved", resp[1].Decision)
}

func TestChatHandler_PermissionAudit_NotConfigured(t *testing.T) {
	r, svc := setupChat()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Audit", Status: chat.StatusActive}

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/permission-audit", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestChatHandler_PermissionAudit_SessionNotFoundDoesNotReadAudit(t *testing.T) {
	reader := &mockPermissionAuditReader{}
	r, _ := setupChatWithPermissionAudit(reader)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+uuid.New().String()+"/permission-audit", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, reader.called, "audit reader must not be queried for a missing session")
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

func TestChatHandler_CloneSession_Success(t *testing.T) {
	r, svc := setupChat()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Chat", Status: chat.StatusActive}
	firstID := uuid.New()
	svc.messages[sessionID] = []chat.ChatMessage{
		{ID: firstID, SessionID: sessionID, Role: "user", Content: "primeira", MessageType: chat.MessageTypeText},
		{ID: uuid.New(), SessionID: sessionID, Role: "user", Content: "segunda", MessageType: chat.MessageTypeText},
	}

	body, _ := json.Marshal(chat.CloneSessionRequest{UntilMessageID: &firstID, Title: "Clone A"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/clone", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp chat.ChatSessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEqual(t, sessionID, resp.ID)
	assert.Equal(t, "Clone A", resp.Title)
	require.NotNil(t, resp.ClonedFromSessionID)
	assert.Equal(t, sessionID, *resp.ClonedFromSessionID)
	require.NotNil(t, resp.ClonedFromSessionTitle)
	assert.Equal(t, "Chat", *resp.ClonedFromSessionTitle)

	require.Len(t, svc.messages[resp.ID], 1)
	assert.Equal(t, "primeira", svc.messages[resp.ID][0].Content)
	assert.NotEqual(t, firstID, svc.messages[resp.ID][0].ID)
	assert.Nil(t, svc.messages[resp.ID][0].RunID)
}

func TestChatHandler_CloneSession_400_ConcatenatedJSONDoesNotCallService(t *testing.T) {
	r, svc := setupChat()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Chat", Status: chat.StatusActive}

	req := httptest.NewRequest(http.MethodPost,
		"/api/chat/sessions/"+sessionID.String()+"/clone",
		bytes.NewBufferString(`{"title":"Clone A"}{"title":"Clone B"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Zero(t, svc.cloneCalls)
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

func TestChatHandler_ListMessages_RedactsSensitiveToolCallArguments(t *testing.T) {
	r, svc := setupChat()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Title: "Chat", Status: chat.StatusActive}

	const bearerSecret = "Bearer history-tool-call-bearer-sentinel"
	const apiKeySecret = "history-tool-call-api-key-sentinel"
	const passwordSecret = "history-tool-call-password-sentinel"
	toolCalls, err := json.Marshal([]map[string]any{{
		"id":   "tool-call-history-1",
		"type": "function",
		"function": map[string]any{
			"name":      "http_request",
			"arguments": `{"safe":"keep-this","Authorization":"` + bearerSecret + `","password":"` + passwordSecret + `","nested":{"api_key":"` + apiKeySecret + `"}}`,
		},
	}})
	require.NoError(t, err)
	svc.messages[sessionID] = []chat.ChatMessage{{
		ID:          uuid.New(),
		SessionID:   sessionID,
		Role:        "assistant",
		MessageType: chat.MessageTypeToolUse,
		ToolCalls:   toolCalls,
	}}

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/messages", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, bearerSecret)
	assert.NotContains(t, body, apiKeySecret)
	assert.NotContains(t, body, passwordSecret)
	assert.NotContains(t, body, "Authorization")
	assert.NotContains(t, body, "api_key")
	assert.NotContains(t, body, "password")
	assert.Contains(t, body, "keep-this")
	assert.Equal(t, json.RawMessage(toolCalls), svc.messages[sessionID][0].ToolCalls, "the public projection must not mutate persisted tool calls")
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

func TestChatHandler_RunSession_ForwardsSystemPromptOverride(t *testing.T) {
	r, svc := setupChat()

	agentID := uuid.New()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, AgentID: &agentID, Title: "studio", Status: chat.StatusActive}

	body, _ := json.Marshal(map[string]any{
		"message": "Hello",
		"overrides": map[string]any{
			"systemPrompt": "Preview prompt from Studio.",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, svc.lastRunOptions, 1)
	require.NotNil(t, svc.lastRunOptions[0].SystemPromptOverride)
	assert.Equal(t, "Preview prompt from Studio.", *svc.lastRunOptions[0].SystemPromptOverride)
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

func TestChatHandler_RunSession_RejectsTrailingJSONWithoutStartingRun(t *testing.T) {
	r, svc := setupChat()
	agentID := uuid.New()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, AgentID: &agentID, Title: "test", Status: chat.StatusActive}

	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/run", bytes.NewBufferString(`{"message":"first"}{"message":"ignored"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, svc.runCalls)
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

func TestChatHandler_ResumeSession_DoesNotAllowSSEFrameInjection(t *testing.T) {
	r, svc := setupChat()

	agentID := uuid.New()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, AgentID: &agentID, Title: "test", Status: chat.StatusActive}
	svc.runEvents = []chat.RunEvent{
		{
			Type: "text_delta\nevent: injected",
			Data: json.RawMessage("{\"content\":\"safe\"}\nevent: injected\ndata: bad"),
		},
		{
			Type: "run_complete",
			Data: json.RawMessage(`{"totalTurns":1,"totalTokens":50}`),
		},
	}

	body, _ := json.Marshal(map[string]string{"message": "Hello"})
	runReq := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/run", bytes.NewReader(body))
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()
	r.ServeHTTP(runW, runReq)

	require.Equal(t, http.StatusOK, runW.Code)
	runID := runW.Header().Get("X-Run-ID")
	require.NotEmpty(t, runID)

	resumeReq := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/run/"+runID+"/resume", nil)
	resumeW := httptest.NewRecorder()
	r.ServeHTTP(resumeW, resumeReq)

	require.Equal(t, http.StatusOK, resumeW.Code)
	resumeBody := resumeW.Body.String()
	assert.NotContains(t, resumeBody, "event: injected\n")
	assert.NotContains(t, resumeBody, "\ndata: bad\n")
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

func TestChatHandler_ResumeSession_WrongSessionDoesNotReplayLegacyBuffer(t *testing.T) {
	r, svc := setupChat()

	agentID := uuid.New()
	sessionID := uuid.New()
	wrongSessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, AgentID: &agentID, Title: "test", Status: chat.StatusActive}
	svc.sessions[wrongSessionID] = chat.ChatSession{ID: wrongSessionID, AgentID: &agentID, Title: "wrong", Status: chat.StatusActive}

	body, _ := json.Marshal(map[string]string{"message": "Hello"})
	runReq := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/run", bytes.NewReader(body))
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()
	r.ServeHTTP(runW, runReq)

	require.Equal(t, http.StatusOK, runW.Code)
	runID := runW.Header().Get("X-Run-ID")
	require.NotEmpty(t, runID)

	resumeReq := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+wrongSessionID.String()+"/run/"+runID+"/resume", nil)
	resumeReq.Header.Set("Last-Event-ID", runID+":1")
	resumeW := httptest.NewRecorder()
	r.ServeHTTP(resumeW, resumeReq)

	assert.Equal(t, http.StatusNotFound, resumeW.Code)
	assert.NotContains(t, resumeW.Body.String(), "event: run_complete\n")
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

func TestChatHandler_ResumeSession_HeaderTakesPrecedenceOverQueryParamLastEventID(t *testing.T) {
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
	require.NotEmpty(t, runID)

	resumeReq := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/run/"+runID+"/resume?lastEventId="+runID+":0", nil)
	resumeReq.Header.Set("Last-Event-ID", runID+":1")
	resumeW := httptest.NewRecorder()
	r.ServeHTTP(resumeW, resumeReq)

	require.Equal(t, http.StatusOK, resumeW.Code)
	resumeBody := resumeW.Body.String()
	assert.NotContains(t, resumeBody, "id: "+runID+":1\n")
	assert.Contains(t, resumeBody, "id: "+runID+":2\n")
}

func TestChatHandler_ResumeSession_RejectsLastEventIDForDifferentRun(t *testing.T) {
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
	require.NotEmpty(t, runID)

	resumeReq := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/run/"+runID+"/resume", nil)
	resumeReq.Header.Set("Last-Event-ID", uuid.New().String()+":1")
	resumeW := httptest.NewRecorder()
	r.ServeHTTP(resumeW, resumeReq)

	assert.Equal(t, http.StatusBadRequest, resumeW.Code)
	assert.NotContains(t, resumeW.Body.String(), "event: run_complete\n")
}

func TestChatHandler_ResumeSession_RejectsMalformedLastEventID(t *testing.T) {
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
	require.NotEmpty(t, runID)

	resumeReq := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/run/"+runID+"/resume", nil)
	resumeReq.Header.Set("Last-Event-ID", "not-an-sse-id")
	resumeW := httptest.NewRecorder()
	r.ServeHTTP(resumeW, resumeReq)

	assert.Equal(t, http.StatusBadRequest, resumeW.Code)
	assert.NotContains(t, resumeW.Body.String(), "event: run_complete\n")
}

func TestChatHandler_ResumeSession_RejectsNonCanonicalLastEventID(t *testing.T) {
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
	require.NotEmpty(t, runID)

	tests := []struct {
		name        string
		lastEventID string
		useQuery    bool
	}{
		{name: "leading zero header", lastEventID: runID + ":01"},
		{name: "leading plus header", lastEventID: runID + ":+1"},
		{name: "leading zero query", lastEventID: runID + ":01", useQuery: true},
		{name: "leading plus query", lastEventID: runID + ":+1", useQuery: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resumeURL := "/api/chat/sessions/" + sessionID.String() + "/run/" + runID + "/resume"
			if tt.useQuery {
				resumeURL += "?lastEventId=" + url.QueryEscape(tt.lastEventID)
			}
			resumeReq := httptest.NewRequest(http.MethodGet, resumeURL, nil)
			if !tt.useQuery {
				resumeReq.Header.Set("Last-Event-ID", tt.lastEventID)
			}
			resumeW := httptest.NewRecorder()
			r.ServeHTTP(resumeW, resumeReq)

			assert.Equal(t, http.StatusBadRequest, resumeW.Code)
			assert.NotEqual(t, "text/event-stream", resumeW.Header().Get("Content-Type"))
			assert.NotContains(t, resumeW.Body.String(), "event: run_complete\n")
		})
	}
}

func TestChatHandler_ResumeSession_ReconnectOverflow(t *testing.T) {
	r, svc := setupChat()

	agentID := uuid.New()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, AgentID: &agentID, Title: "test", Status: chat.StatusActive}

	svc.runEvents = make([]chat.RunEvent, 0, chat.DefaultEventBufferSize+2)
	for i := 0; i < chat.DefaultEventBufferSize+1; i++ {
		svc.runEvents = append(svc.runEvents, chat.RunEvent{
			Type: "text_delta",
			Data: json.RawMessage(`{"content":"chunk"}`),
		})
	}
	svc.runEvents = append(svc.runEvents, chat.RunEvent{
		Type: "run_complete",
		Data: json.RawMessage(`{"totalTurns":1,"totalTokens":50}`),
	})

	body, _ := json.Marshal(map[string]string{"message": "Hello"})
	runReq := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/run", bytes.NewReader(body))
	runReq.Header.Set("Content-Type", "application/json")
	runW := httptest.NewRecorder()
	r.ServeHTTP(runW, runReq)

	require.Equal(t, http.StatusOK, runW.Code)
	runID := runW.Header().Get("X-Run-ID")
	require.NotEmpty(t, runID)

	resumeReq := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/run/"+runID+"/resume", nil)
	resumeReq.Header.Set("Last-Event-ID", runID+":1")
	resumeW := httptest.NewRecorder()
	r.ServeHTTP(resumeW, resumeReq)

	require.Equal(t, http.StatusOK, resumeW.Code)
	resumeBody := resumeW.Body.String()
	assert.Contains(t, resumeBody, "event: reconnect_overflow\n")
	assert.Contains(t, resumeBody, `"lastEventId":"`+runID+`:1"`)
	assert.NotContains(t, resumeBody, "id: "+runID+":1\n")
	assert.NotContains(t, resumeBody, "event: run_complete\n")
}

func TestChatHandler_ResumeSession_ContinuesAfterClientDisconnect(t *testing.T) {
	r, svc := setupChat()

	agentID := uuid.New()
	sessionID := uuid.New()
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, AgentID: &agentID, Title: "test", Status: chat.StatusActive}

	eventCh := make(chan chat.RunEvent, 4)
	svc.runEventCh = eventCh

	server := httptest.NewServer(r)
	defer server.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	body, _ := json.Marshal(map[string]string{"message": "Hello"})
	runReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/chat/sessions/"+sessionID.String()+"/run", bytes.NewReader(body))
	require.NoError(t, err)
	runReq.Header.Set("Content-Type", "application/json")
	runReq.Header.Set("Accept", "text/event-stream")

	eventCh <- chat.RunEvent{Type: "text_delta", Data: json.RawMessage(`{"content":"first"}`)}
	runResp, err := client.Do(runReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, runResp.StatusCode)
	runID := runResp.Header.Get("X-Run-ID")
	require.NotEmpty(t, runID)

	initialReader := bufio.NewReader(runResp.Body)
	firstBlock := readSSEBlock(t, initialReader)
	require.Contains(t, firstBlock, "event: text_delta\n")
	lastEventID := sseIDFromBlock(t, firstBlock)
	require.NoError(t, runResp.Body.Close())

	eventCh <- chat.RunEvent{Type: "text_delta", Data: json.RawMessage(`{"content":"second"}`)}
	eventCh <- chat.RunEvent{Type: "run_complete", Data: json.RawMessage(`{"totalTurns":1,"totalTokens":50}`)}
	close(eventCh)

	resumeReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/chat/sessions/"+sessionID.String()+"/run/"+runID+"/resume", nil)
	require.NoError(t, err)
	resumeReq.Header.Set("Accept", "text/event-stream")
	resumeReq.Header.Set("Last-Event-ID", lastEventID)
	resumeResp, err := client.Do(resumeReq)
	require.NoError(t, err)
	defer func() { _ = resumeResp.Body.Close() }()

	require.Equal(t, http.StatusOK, resumeResp.StatusCode)
	resumeBody, err := io.ReadAll(resumeResp.Body)
	require.NoError(t, err)
	resumeText := string(resumeBody)
	assert.NotContains(t, resumeText, "id: "+lastEventID+"\n")
	assert.Contains(t, resumeText, "id: "+runID+":2\n")
	assert.Contains(t, resumeText, `{"content":"second"}`)
	assert.Contains(t, resumeText, "id: "+runID+":3\n")
	assert.Contains(t, resumeText, "event: run_complete\n")
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

func TestGetRun_RedactsSensitiveFailureReason(t *testing.T) {
	const authorizationSecret = "run-response-authorization-secret"
	const passwordSecret = "run-response-password-secret"
	runID := uuid.New()
	sessionID := uuid.New()
	failureReason := "Authorization: Bearer " + authorizationSecret + "\npassword=" + passwordSecret

	rl := &mockRunLookup{runs: map[uuid.UUID]chat.ChatRun{
		runID: {
			ID:            runID,
			SessionID:     sessionID,
			Status:        chat.ChatRunStatusFailed,
			FailureReason: &failureReason,
		},
	}}
	r, _ := setupChatWithRuns(rl)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/runs/"+runID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, authorizationSecret)
	assert.NotContains(t, body, passwordSecret)
	assert.NotContains(t, body, "Authorization:")
	assert.NotContains(t, body, "password")
	assert.Contains(t, body, "[REDACTED]")
}

func TestGetRun_RedactsSensitiveMetadataErrorMessage(t *testing.T) {
	const authorizationSecret = "run-metadata-authorization-secret"
	const passwordSecret = "run-metadata-password-secret"
	runID := uuid.New()
	sessionID := uuid.New()
	metadata := json.RawMessage(`{"finishReason":"error","errorMessage":"Authorization: Bearer ` + authorizationSecret + `\npassword=` + passwordSecret + `","nested":{"api_key":"nested-metadata-api-key"}}`)

	rl := &mockRunLookup{runs: map[uuid.UUID]chat.ChatRun{
		runID: {
			ID:        runID,
			SessionID: sessionID,
			Status:    chat.ChatRunStatusFailed,
			Metadata:  metadata,
		},
	}}
	r, _ := setupChatWithRuns(rl)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/runs/"+runID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, authorizationSecret)
	assert.NotContains(t, body, passwordSecret)
	assert.NotContains(t, body, "nested-metadata-api-key")
	assert.NotContains(t, body, "api_key")
	assert.NotContains(t, body, "Authorization:")
	assert.NotContains(t, body, "password")
	assert.Contains(t, body, "[REDACTED]")
	assert.JSONEq(t, string(metadata), string(rl.runs[runID].Metadata), "the persisted run must remain unchanged")
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
	tasks         []task.Task
	notifications []task.Notification
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
	return m.notifications, nil
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
	r, svc := setupChatWithTasks(repo)
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Status: chat.StatusActive}

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
	r, svc := setupChatWithTasks(repo)
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Status: chat.StatusActive}

	url := "/api/chat/sessions/" + sessionID.String() + "/tasks/task-1/notifications"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_GetTaskNotifications_RedactsSensitiveDiagnostics(t *testing.T) {
	sessionID := uuid.New()
	errorMessage := "Authorization: Bearer task-notification-authorization-secret\npassword=task-notification-password-secret"
	repo := &mockHandlerTaskRepo{
		notifications: []task.Notification{{
			ID:       "notification-1",
			TaskID:   "task-1",
			Status:   "failed",
			Findings: json.RawMessage(`{"provider":{"api_key":"task-notification-findings-secret"},"message":"Authorization: Bearer task-notification-findings-bearer"}`),
			Error:    &errorMessage,
		}},
	}
	r, svc := setupChatWithTasks(repo)
	svc.sessions[sessionID] = chat.ChatSession{ID: sessionID, Status: chat.StatusActive}

	req := httptest.NewRequest(http.MethodGet, "/api/chat/sessions/"+sessionID.String()+"/tasks/task-1/notifications", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "task-notification-authorization-secret")
	assert.NotContains(t, w.Body.String(), "task-notification-password-secret")
	assert.NotContains(t, w.Body.String(), "task-notification-findings-secret")
	assert.NotContains(t, w.Body.String(), "task-notification-findings-bearer")
	assert.Contains(t, w.Body.String(), "[REDACTED]")
	assert.Contains(t, string(repo.notifications[0].Findings), "task-notification-findings-secret")
	assert.Equal(t, errorMessage, *repo.notifications[0].Error)
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

func TestHandler_ClientState_400_ConcatenatedJSONDoesNotCallService(t *testing.T) {
	r, svc := setupChat()
	sessionID := uuid.New()
	req := httptest.NewRequest(http.MethodPost,
		"/api/chat/sessions/"+sessionID.String()+"/client-state",
		bytes.NewBufferString(`{"readables":[]}{"readables":[]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Zero(t, svc.clientStateCalls)
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
