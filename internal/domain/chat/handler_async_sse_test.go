package chat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerAsyncRunSessionStreamsSSE(t *testing.T) {
	agentID := uuid.New()
	sessionID := uuid.New()
	repo := &asyncExecutorRepoStub{session: ChatSession{
		ID:      sessionID,
		AgentID: &agentID,
		Status:  StatusActive,
	}}
	runner := &asyncExecutorRunnerStub{events: []RunEvent{
		{Type: "text_delta", Data: json.RawMessage(`{"content":"hello"}`)},
		{Type: "run_complete", Data: json.RawMessage(`{"totalTurns":1}`)},
	}}
	h := NewHandler(nil, NewAsyncExecutor(repo, runner, ""))
	router := chi.NewRouter()
	h.RegisterRoutes(router)

	body := bytes.NewBufferString(`{"message":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/chat/sessions/"+sessionID.String()+"/run", body)
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "text/event-stream", response.Header().Get("Content-Type"))
	require.NotEmpty(t, response.Header().Get("X-Run-ID"))
	assert.Contains(t, response.Body.String(), "event: text_delta\n")
	assert.Contains(t, response.Body.String(), "event: run_complete\n")
	assert.NotContains(t, response.Body.String(), `"status":"accepted"`)
}
