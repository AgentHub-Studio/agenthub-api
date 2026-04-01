package execution_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/execution"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockExecutionSvc satisfies the private executionService interface in execution.Handler.
type mockExecutionSvc struct {
	executions map[uuid.UUID]execution.AgentExecution
}

func newMockExecutionSvc() *mockExecutionSvc {
	return &mockExecutionSvc{executions: make(map[uuid.UUID]execution.AgentExecution)}
}

func (m *mockExecutionSvc) List(_ context.Context, _ *uuid.UUID, _ *string, req pagination.PageRequest) (pagination.Page[execution.AgentExecution], error) {
	items := make([]execution.AgentExecution, 0, len(m.executions))
	for _, e := range m.executions {
		items = append(items, e)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockExecutionSvc) Start(_ context.Context, req execution.StartExecutionRequest) (execution.AgentExecution, error) {
	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		return execution.AgentExecution{}, err
	}
	id := uuid.New()
	e := execution.AgentExecution{ID: id, AgentID: agentID, Status: "RUNNING"}
	m.executions[id] = e
	return e, nil
}

func (m *mockExecutionSvc) GetByID(_ context.Context, id uuid.UUID) (execution.AgentExecution, error) {
	e, ok := m.executions[id]
	if !ok {
		return execution.AgentExecution{}, execution.ErrNotFound
	}
	return e, nil
}

func (m *mockExecutionSvc) Cancel(_ context.Context, id uuid.UUID) error {
	if _, ok := m.executions[id]; !ok {
		return execution.ErrNotFound
	}
	delete(m.executions, id)
	return nil
}

func (m *mockExecutionSvc) GetDetails(_ context.Context, id uuid.UUID) (execution.ExecutionDetails, error) {
	e, ok := m.executions[id]
	if !ok {
		return execution.ExecutionDetails{}, execution.ErrNotFound
	}
	return execution.ExecutionDetails{AgentExecution: e, Nodes: []execution.NodeDetails{}}, nil
}

func (m *mockExecutionSvc) ListNodes(_ context.Context, _ uuid.UUID) ([]execution.AgentExecutionNode, error) {
	return []execution.AgentExecutionNode{}, nil
}

func (m *mockExecutionSvc) ListToolExecutions(_ context.Context, _ uuid.UUID) ([]execution.ToolExecution, error) {
	return []execution.ToolExecution{}, nil
}

func setupExecution() (*chi.Mux, *mockExecutionSvc) {
	svc := newMockExecutionSvc()
	h := execution.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestExecutionHandler_List_Success(t *testing.T) {
	r, svc := setupExecution()
	id := uuid.New()
	svc.executions[id] = execution.AgentExecution{ID: id, AgentID: uuid.New(), Status: "RUNNING"}

	req := httptest.NewRequest(http.MethodGet, "/api/executions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[execution.AgentExecution]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestExecutionHandler_Start_Success(t *testing.T) {
	r, _ := setupExecution()
	body, _ := json.Marshal(execution.StartExecutionRequest{
		AgentID: uuid.New().String(),
		Input:   json.RawMessage(`{"query":"hello"}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/executions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp execution.AgentExecution
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "RUNNING", resp.Status)
}

func TestExecutionHandler_Start_InvalidBody(t *testing.T) {
	r, _ := setupExecution()
	req := httptest.NewRequest(http.MethodPost, "/api/executions", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExecutionHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupExecution()
	req := httptest.NewRequest(http.MethodGet, "/api/executions/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExecutionHandler_Cancel_Success(t *testing.T) {
	r, svc := setupExecution()
	id := uuid.New()
	svc.executions[id] = execution.AgentExecution{ID: id, AgentID: uuid.New(), Status: "RUNNING"}

	req := httptest.NewRequest(http.MethodDelete, "/api/executions/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestExecutionHandler_Cancel_NotFound(t *testing.T) {
	r, _ := setupExecution()
	req := httptest.NewRequest(http.MethodDelete, "/api/executions/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExecutionHandler_ListNodes_Success(t *testing.T) {
	r, svc := setupExecution()
	id := uuid.New()
	svc.executions[id] = execution.AgentExecution{ID: id, AgentID: uuid.New(), Status: "SUCCESS"}

	req := httptest.NewRequest(http.MethodGet, "/api/executions/"+id.String()+"/nodes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
