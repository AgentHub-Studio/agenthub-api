package execution_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/execution"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
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
		return execution.AgentExecution{}, fmt.Errorf("%w: invalid agentId", execution.ErrInvalidInput)
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

func (m *mockExecutionSvc) ListToolExecutions(_ context.Context, executionID uuid.UUID, _ uuid.UUID) ([]execution.ToolExecution, error) {
	if _, ok := m.executions[executionID]; !ok {
		return nil, execution.ErrNotFound
	}
	return []execution.ToolExecution{}, nil
}

func setupExecution() (*chi.Mux, *mockExecutionSvc) {
	return setupExecutionWithRoles("admin")
}

func setupExecutionWithRoles(roles ...string) (*chi.Mux, *mockExecutionSvc) {
	svc := newMockExecutionSvc()
	h := execution.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)
	return r, svc
}

func TestExecutionHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupExecutionWithRoles("user")
	executionID := uuid.NewString()
	nodeID := uuid.NewString()
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/executions"},
		{name: "start", method: http.MethodPost, path: "/api/executions", body: `{}`},
		{name: "get", method: http.MethodGet, path: "/api/executions/" + executionID},
		{name: "cancel", method: http.MethodDelete, path: "/api/executions/" + executionID},
		{name: "list nodes", method: http.MethodGet, path: "/api/executions/" + executionID + "/nodes"},
		{name: "list tools", method: http.MethodGet, path: "/api/executions/" + executionID + "/nodes/" + nodeID + "/tools"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "missing required role")
		})
	}
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

func TestExecutionHandler_List_RedactsSensitiveErrorMessage(t *testing.T) {
	const authorizationSecret = "execution-authorization-secret"
	const passwordSecret = "execution-password-secret"
	r, svc := setupExecution()
	id := uuid.New()
	errorMessage := "Authorization: Bearer " + authorizationSecret + "\npassword=" + passwordSecret
	svc.executions[id] = execution.AgentExecution{
		ID: id, AgentID: uuid.New(), Status: execution.StatusFailed, ErrorMessage: &errorMessage,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/executions", nil)
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

func TestPublicExecutionDetailsFrom_RedactsNestedErrorMessages(t *testing.T) {
	const authorizationSecret = "execution-details-authorization-secret"
	const passwordSecret = "execution-details-password-secret"
	errorMessage := "Authorization: Bearer " + authorizationSecret + "\npassword=" + passwordSecret

	public := execution.PublicExecutionDetailsFrom(execution.ExecutionDetails{
		AgentExecution: execution.AgentExecution{ErrorMessage: &errorMessage},
		Nodes: []execution.NodeDetails{{
			AgentExecutionNode: execution.AgentExecutionNode{ErrorMessage: &errorMessage},
			Tools:              []execution.ToolExecution{{ErrorMessage: &errorMessage}},
		}},
	})

	require.NotNil(t, public.ErrorMessage)
	require.NotNil(t, public.Nodes[0].ErrorMessage)
	require.NotNil(t, public.Nodes[0].Tools[0].ErrorMessage)
	for _, value := range []string{*public.ErrorMessage, *public.Nodes[0].ErrorMessage, *public.Nodes[0].Tools[0].ErrorMessage} {
		assert.NotContains(t, value, authorizationSecret)
		assert.NotContains(t, value, passwordSecret)
		assert.NotContains(t, value, "Authorization:")
		assert.NotContains(t, value, "password")
		assert.Contains(t, value, "[REDACTED]")
	}
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

func TestExecutionHandlerStartRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	r, svc := setupExecution()
	req := httptest.NewRequest(http.MethodPost, "/api/executions", bytes.NewBufferString(`{"agentId":"`+uuid.NewString()+`","input":{"query":"first"}}{"agentId":"`+uuid.NewString()+`"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Empty(t, svc.executions)
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

func TestExecutionHandler_ListTools_InvalidExecutionID_Returns400(t *testing.T) {
	r, _ := setupExecution()
	req := httptest.NewRequest(http.MethodGet, "/api/executions/not-a-uuid/nodes/"+uuid.New().String()+"/tools", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExecutionHandler_ListTools_WrongExecutionID_Returns404(t *testing.T) {
	r, svc := setupExecution()
	id := uuid.New()
	svc.executions[id] = execution.AgentExecution{ID: id, AgentID: uuid.New(), Status: "RUNNING"}

	req := httptest.NewRequest(http.MethodGet, "/api/executions/"+uuid.New().String()+"/nodes/"+uuid.New().String()+"/tools", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExecutionHandler_Start_InvalidAgentID_Returns422(t *testing.T) {
	r, _ := setupExecution()
	body, _ := json.Marshal(execution.StartExecutionRequest{
		AgentID: "not-a-uuid",
		Input:   json.RawMessage(`{}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/executions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
