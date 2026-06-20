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
		ID:                    uuid.New(),
		AgentID:               agentID,
		Name:                  req.Name,
		CronExpression:        req.CronExpression,
		Enabled:               true,
		InputTemplate:         req.InputTemplate,
		NotificationWebhookID: req.NotificationWebhookID,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
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
	if req.NotificationWebhookID.Set {
		t.NotificationWebhookID = req.NotificationWebhookID.Value
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
	svc := newMockSvc()
	h := trigger.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

// --- handler tests ---

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

	body := []byte(`{"name":"updated"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/triggers/"+triggerID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result trigger.AgentTrigger
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "updated", result.Name)
}

func TestTriggerHandler_Update_ClearNotificationWebhook(t *testing.T) {
	r, svc := setupTrigger()
	agentID := uuid.New()
	triggerID := uuid.New()
	webhookID := uuid.New()
	svc.triggers[triggerID] = trigger.AgentTrigger{
		ID: triggerID, AgentID: agentID, Name: "original", NotificationWebhookID: &webhookID,
	}

	body := []byte(`{"notificationWebhookId":null}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/agents/"+agentID.String()+"/triggers/"+triggerID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result trigger.AgentTrigger
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Nil(t, result.NotificationWebhookID)
}

func TestTriggerHandler_Update_NotFound(t *testing.T) {
	r, _ := setupTrigger()
	agentID := uuid.New()
	body := []byte(`{}`)
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
