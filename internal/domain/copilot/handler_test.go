package copilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/copilot"
	commonsai "github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// --- shared fakes ------------------------------------------------------------

type fakeChatModel struct {
	response string
	err      error
	calls    int
}

func (m *fakeChatModel) Chat(_ context.Context, _ []commonsai.Message, _ commonsai.ChatOptions) (*commonsai.ChatResponse, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &commonsai.ChatResponse{Content: m.response}, nil
}

func (m *fakeChatModel) ChatStream(_ context.Context, _ []commonsai.Message, _ commonsai.ChatOptions) (<-chan commonsai.StreamChunk, error) {
	return nil, errors.New("not supported")
}

func (m *fakeChatModel) GetProviderName() string { return "mock" }

type fakeFactory struct {
	model    commonsai.ChatModel
	buildErr error
}

func (f *fakeFactory) Build(_ context.Context, _, _ string) (commonsai.ChatModel, error) {
	return f.model, f.buildErr
}

func (f *fakeFactory) ResolveModel(_ context.Context, _ string) string { return "" }

func (f *fakeFactory) ResolveDefaultProvider(_ context.Context) string { return "mock" }

// --- handler tests -----------------------------------------------------------

func setup(svc *copilot.Service) *chi.Mux {
	r := chi.NewRouter()
	copilot.NewHandler(svc).RegisterRoutes(r)
	return r
}

func TestHandler_Completions_200(t *testing.T) {
	svc := copilot.NewService(&fakeFactory{model: &fakeChatModel{response: "world"}})
	r := setup(svc)

	body := bytes.NewBufferString(`{"text":"hello "}`)
	req := httptest.NewRequest(http.MethodPost, "/api/copilot/completions", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp copilot.CompletionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "world", resp.Suggestion)
}

func TestHandler_Completions_400_InvalidJSON(t *testing.T) {
	svc := copilot.NewService(&fakeFactory{model: &fakeChatModel{}})
	r := setup(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/copilot/completions",
		bytes.NewBufferString(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_CompletionsRejectsTrailingJSONWithoutCallingModel(t *testing.T) {
	model := &fakeChatModel{response: "world"}
	r := setup(copilot.NewService(&fakeFactory{model: model}))
	req := httptest.NewRequest(http.MethodPost, "/api/copilot/completions", bytes.NewBufferString(`{"text":"hello"} {"text":"ignored"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Zero(t, model.calls)
}

func TestHandler_Completions_422_EmptyText(t *testing.T) {
	svc := copilot.NewService(&fakeFactory{model: &fakeChatModel{}})
	r := setup(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/copilot/completions",
		bytes.NewBufferString(`{"text":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestHandler_Completions_503_NoProvider(t *testing.T) {
	svc := copilot.NewService(&fakeFactory{model: nil})
	r := setup(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/copilot/completions",
		bytes.NewBufferString(`{"text":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandler_Completions_500_LLMError(t *testing.T) {
	svc := copilot.NewService(&fakeFactory{
		model: &fakeChatModel{err: errors.New("api down")},
	})
	r := setup(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/copilot/completions",
		bytes.NewBufferString(`{"text":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
