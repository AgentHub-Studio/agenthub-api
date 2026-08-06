package trigger_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/trigger"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// --- mock service ---

type mockSvc struct {
	triggers map[uuid.UUID]trigger.AgentTrigger
	runs     map[uuid.UUID]trigger.AgentTriggerRun
}

func newMockSvc() *mockSvc {
	return &mockSvc{
		triggers: make(map[uuid.UUID]trigger.AgentTrigger),
		runs:     make(map[uuid.UUID]trigger.AgentTriggerRun),
	}
}

func (m *mockSvc) Create(_ context.Context, agentID uuid.UUID, req trigger.CreateTriggerRequest) (trigger.AgentTrigger, error) {
	if req.Name == "" {
		return trigger.AgentTrigger{}, trigger.ErrNotFound
	}
	t := trigger.AgentTrigger{
		ID:             uuid.New(),
		AgentID:        agentID,
		Name:           req.Name,
		CronExpression: req.CronExpression,
		Enabled:        true,
		InputTemplate:  req.InputTemplate,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	m.triggers[t.ID] = t
	return t, nil
}

func (m *mockSvc) GetByID(_ context.Context, id uuid.UUID) (trigger.AgentTrigger, error) {
	t, ok := m.triggers[id]
	if !ok {
		return trigger.AgentTrigger{}, trigger.ErrNotFound
	}
	return t, nil
}

func (m *mockSvc) List(_ context.Context, agentID uuid.UUID, page pagination.PageRequest) (pagination.Page[trigger.AgentTrigger], error) {
	var items []trigger.AgentTrigger
	for _, t := range m.triggers {
		if t.AgentID == agentID {
			items = append(items, t)
		}
	}
	if items == nil {
		items = []trigger.AgentTrigger{}
	}
	return pagination.NewPage(items, int64(len(items)), page), nil
}

func (m *mockSvc) Update(_ context.Context, id uuid.UUID, req trigger.UpdateTriggerRequest) (trigger.AgentTrigger, error) {
	t, ok := m.triggers[id]
	if !ok {
		return trigger.AgentTrigger{}, trigger.ErrNotFound
	}
	if req.Name != nil {
		t.Name = *req.Name
	}
	m.triggers[id] = t
	return t, nil
}

func (m *mockSvc) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.triggers[id]; !ok {
		return trigger.ErrNotFound
	}
	delete(m.triggers, id)
	return nil
}

func (m *mockSvc) ListRuns(_ context.Context, triggerID uuid.UUID, page pagination.PageRequest) (pagination.Page[trigger.AgentTriggerRun], error) {
	var items []trigger.AgentTriggerRun
	for _, r := range m.runs {
		if r.TriggerID == triggerID {
			items = append(items, r)
		}
	}
	if items == nil {
		items = []trigger.AgentTriggerRun{}
	}
	return pagination.NewPage(items, int64(len(items)), page), nil
}

func setupTrigger() (*chi.Mux, *mockSvc) {
	return setupTriggerWithRoles("admin")
}

func setupTriggerWithRoles(roles ...string) (*chi.Mux, *mockSvc) {
	svc := newMockSvc()
	h := trigger.NewHandler(svc)
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

// --- handler tests ---

func TestTriggerHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupTriggerWithRoles("user")
	agentID := uuid.NewString()
	triggerID := uuid.NewString()
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create", method: http.MethodPost, path: "/api/agents/" + agentID + "/triggers", body: `{}`},
		{name: "list", method: http.MethodGet, path: "/api/agents/" + agentID + "/triggers"},
		{name: "get", method: http.MethodGet, path: "/api/agents/" + agentID + "/triggers/" + triggerID},
		{name: "put", method: http.MethodPut, path: "/api/agents/" + agentID + "/triggers/" + triggerID, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/agents/" + agentID + "/triggers/" + triggerID, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/agents/" + agentID + "/triggers/" + triggerID},
		{name: "list runs", method: http.MethodGet, path: "/api/agents/" + agentID + "/triggers/" + triggerID + "/runs"},
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

func TestTriggerHandler_Create_Success(t *testing.T) {
	r, _ := setupTrigger()
	agentID := uuid.New()
	body, _ := json.Marshal(trigger.CreateTriggerRequest{
		Name:           "Daily check",
		CronExpression: "0 9 * * *",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/triggers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var result trigger.AgentTrigger
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "Daily check", result.Name)
}

func TestTriggerHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupTrigger()
	agentID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/triggers", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTriggerHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, svc := setupTrigger()
		agentID := uuid.New()
		req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/triggers", bytes.NewBufferString(`{"name":"first","cronExpression":"0 9 * * *"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.triggers)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupTrigger()
		agentID := uuid.New()
		triggerID := uuid.New()
		svc.triggers[triggerID] = trigger.AgentTrigger{ID: triggerID, AgentID: agentID, Name: "original"}
		req := httptest.NewRequest(http.MethodPatch, "/api/agents/"+agentID.String()+"/triggers/"+triggerID.String(), bytes.NewBufferString(`{"name":"changed"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, "original", svc.triggers[triggerID].Name)
	})
}

func TestTriggerHandler_Create_InvalidAgentID(t *testing.T) {
	r, _ := setupTrigger()
	body, _ := json.Marshal(trigger.CreateTriggerRequest{Name: "t", CronExpression: "0 * * * *"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/invalid/triggers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTriggerHandler_List_Success(t *testing.T) {
	r, svc := setupTrigger()
	agentID := uuid.New()
	svc.triggers[uuid.New()] = trigger.AgentTrigger{
		ID: uuid.New(), AgentID: agentID, Name: "t1", CronExpression: "0 * * * *",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/triggers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTriggerHandler_GetByID_Success(t *testing.T) {
	r, svc := setupTrigger()
	agentID := uuid.New()
	triggerID := uuid.New()
	svc.triggers[triggerID] = trigger.AgentTrigger{
		ID: triggerID, AgentID: agentID, Name: "t1",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/triggers/"+triggerID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTriggerHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupTrigger()
	agentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/triggers/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTriggerHandler_Update_Success(t *testing.T) {
	r, svc := setupTrigger()
	agentID := uuid.New()
	triggerID := uuid.New()
	svc.triggers[triggerID] = trigger.AgentTrigger{
		ID: triggerID, AgentID: agentID, Name: "original",
	}

	newName := "updated"
	body, _ := json.Marshal(trigger.UpdateTriggerRequest{Name: &newName})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/triggers/"+triggerID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result trigger.AgentTrigger
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "updated", result.Name)
}

func TestTriggerHandler_Update_NotFound(t *testing.T) {
	r, _ := setupTrigger()
	agentID := uuid.New()
	body, _ := json.Marshal(trigger.UpdateTriggerRequest{})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/triggers/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTriggerHandler_Delete_Success(t *testing.T) {
	r, svc := setupTrigger()
	agentID := uuid.New()
	triggerID := uuid.New()
	svc.triggers[triggerID] = trigger.AgentTrigger{
		ID: triggerID, AgentID: agentID, Name: "t1",
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/agents/"+agentID.String()+"/triggers/"+triggerID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestTriggerHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupTrigger()
	agentID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/agents/"+agentID.String()+"/triggers/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTriggerHandler_ListRuns_Success(t *testing.T) {
	r, svc := setupTrigger()
	agentID := uuid.New()
	triggerID := uuid.New()
	svc.triggers[triggerID] = trigger.AgentTrigger{ID: triggerID, AgentID: agentID}
	svc.runs[uuid.New()] = trigger.AgentTriggerRun{
		ID: uuid.New(), TriggerID: triggerID, SessionID: uuid.New(), Status: trigger.RunStatusCompleted,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/triggers/"+triggerID.String()+"/runs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTriggerHandler_ListRuns_RedactsSensitiveError(t *testing.T) {
	const authorizationSecret = "trigger-run-authorization-secret"
	const passwordSecret = "trigger-run-password-secret"
	r, svc := setupTrigger()
	agentID := uuid.New()
	triggerID := uuid.New()
	errorMessage := "Authorization: Bearer " + authorizationSecret + "\npassword=" + passwordSecret
	svc.triggers[triggerID] = trigger.AgentTrigger{ID: triggerID, AgentID: agentID}
	svc.runs[uuid.New()] = trigger.AgentTriggerRun{
		ID: uuid.New(), TriggerID: triggerID, SessionID: uuid.New(), Status: trigger.RunStatusFailed, Error: &errorMessage,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/triggers/"+triggerID.String()+"/runs", nil)
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

func TestTriggerHandler_NestedRoutesRejectTriggerOutsideRouteAgent(t *testing.T) {
	r, svc := setupTrigger()
	ownerID := uuid.New()
	otherAgentID := uuid.New()
	triggerID := uuid.New()

	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{
			name:   "get",
			method: http.MethodGet,
			path:   "/api/agents/" + otherAgentID.String() + "/triggers/" + triggerID.String(),
		},
		{
			name:   "update",
			method: http.MethodPatch,
			path:   "/api/agents/" + otherAgentID.String() + "/triggers/" + triggerID.String(),
			body:   []byte(`{"name":"must-not-update"}`),
		},
		{
			name:   "delete",
			method: http.MethodDelete,
			path:   "/api/agents/" + otherAgentID.String() + "/triggers/" + triggerID.String(),
		},
		{
			name:   "list runs",
			method: http.MethodGet,
			path:   "/api/agents/" + otherAgentID.String() + "/triggers/" + triggerID.String() + "/runs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc.triggers[triggerID] = trigger.AgentTrigger{ID: triggerID, AgentID: ownerID, Name: "owner trigger"}
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(tt.body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}
