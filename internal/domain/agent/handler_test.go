package agent_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockAgentSvc implements agent.Service for handler tests.
type mockAgentSvc struct {
	agents     map[uuid.UUID]agent.AgentResponse
	updateErr  error // if set, Update returns this error
	publishErr error // if set, Publish returns this error
}

func newMockSvc() *mockAgentSvc {
	return &mockAgentSvc{agents: make(map[uuid.UUID]agent.AgentResponse)}
}

func (m *mockAgentSvc) List(_ context.Context, _ agent.AgentStatus, _ string, req pagination.PageRequest) (pagination.Page[agent.AgentResponse], error) {
	items := make([]agent.AgentResponse, 0, len(m.agents))
	for _, a := range m.agents {
		items = append(items, a)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockAgentSvc) Get(_ context.Context, id uuid.UUID) (agent.AgentResponse, error) {
	a, ok := m.agents[id]
	if !ok {
		return agent.AgentResponse{}, agent.ErrNotFound
	}
	return a, nil
}

func (m *mockAgentSvc) Create(_ context.Context, req agent.CreateAgentRequest) (agent.AgentResponse, error) {
	id := uuid.New()
	resp := agent.AgentResponse{
		ID:   id,
		Name: req.Name,
		Slug: req.Slug,
	}
	m.agents[id] = resp
	return resp, nil
}

func (m *mockAgentSvc) Update(_ context.Context, id uuid.UUID, req agent.UpdateAgentRequest) (agent.AgentResponse, error) {
	if m.updateErr != nil {
		return agent.AgentResponse{}, m.updateErr
	}
	a, ok := m.agents[id]
	if !ok {
		return agent.AgentResponse{}, agent.ErrNotFound
	}
	if req.Name != nil {
		a.Name = *req.Name
	}
	m.agents[id] = a
	return a, nil
}

func (m *mockAgentSvc) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.agents[id]; !ok {
		return agent.ErrNotFound
	}
	delete(m.agents, id)
	return nil
}

func (m *mockAgentSvc) BulkDelete(_ context.Context, ids []uuid.UUID) (int, error) {
	count := 0
	for _, id := range ids {
		if _, ok := m.agents[id]; ok {
			delete(m.agents, id)
			count++
		}
	}
	return count, nil
}

func (m *mockAgentSvc) Publish(_ context.Context, id uuid.UUID) (agent.AgentResponse, error) {
	if m.publishErr != nil {
		return agent.AgentResponse{}, m.publishErr
	}
	a, ok := m.agents[id]
	if !ok {
		return agent.AgentResponse{}, agent.ErrNotFound
	}
	a.Status = string(agent.StatusPublished)
	m.agents[id] = a
	return a, nil
}

func (m *mockAgentSvc) Archive(_ context.Context, id uuid.UUID) (agent.AgentResponse, error) {
	a, ok := m.agents[id]
	if !ok {
		return agent.AgentResponse{}, agent.ErrNotFound
	}
	a.Status = string(agent.StatusArchived)
	m.agents[id] = a
	return a, nil
}

func (m *mockAgentSvc) Clone(_ context.Context, id uuid.UUID, req agent.CloneAgentRequest) (agent.AgentResponse, error) {
	a, ok := m.agents[id]
	if !ok {
		return agent.AgentResponse{}, agent.ErrNotFound
	}
	newID := uuid.New()
	name := req.Name
	if name == "" {
		name = a.Name + " (copy)"
	}
	clone := agent.AgentResponse{ID: newID, Name: name}
	m.agents[newID] = clone
	return clone, nil
}

func setupAgent() (*chi.Mux, *mockAgentSvc) {
	svc := newMockSvc()
	h := agent.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestAgentHandler_List_Success(t *testing.T) {
	r, svc := setupAgent()
	// seed one agent
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Agent A"}

	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[agent.AgentResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestAgentHandler_List_Empty(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentHandler_Create_Success(t *testing.T) {
	r, _ := setupAgent()
	body, _ := json.Marshal(agent.CreateAgentRequest{Name: "My Agent"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp agent.AgentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "My Agent", resp.Name)
}

func TestAgentHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentHandler_Get_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Test Agent"}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentHandler_Get_NotFound(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentHandler_Get_InvalidID(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentHandler_Delete_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "To Delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/agents/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAgentHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodDelete, "/api/agents/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentHandler_Publish_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Draft Agent", Status: string(agent.StatusDraft)}

	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+id.String()+"/publish", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp agent.AgentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, string(agent.StatusPublished), resp.Status)
}

func TestAgentHandler_Clone_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Original"}

	body, _ := json.Marshal(agent.CloneAgentRequest{Name: "Clone"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+id.String()+"/clone", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAgentHandler_Clone_NotFound(t *testing.T) {
	r, _ := setupAgent()
	body, _ := json.Marshal(agent.CloneAgentRequest{Name: "Clone"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+uuid.New().String()+"/clone", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentHandler_Patch_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Original", Status: string(agent.StatusDraft)}

	newName := "Patched"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &newName})
	req := httptest.NewRequest(http.MethodPatch, "/api/agents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp agent.AgentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Patched", resp.Name)
}

func TestAgentHandler_Patch_NotFound(t *testing.T) {
	r, _ := setupAgent()
	name := "x"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &name})
	req := httptest.NewRequest(http.MethodPatch, "/api/agents/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentHandler_Update_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Original", Status: string(agent.StatusDraft)}

	newName := "Updated"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &newName})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp agent.AgentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Updated", resp.Name)
}

func TestAgentHandler_Update_NotFound(t *testing.T) {
	r, _ := setupAgent()
	name := "x"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &name})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentHandler_Archive_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Published Agent", Status: string(agent.StatusPublished)}

	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+id.String()+"/archive", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp agent.AgentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, string(agent.StatusArchived), resp.Status)
}

func TestAgentHandler_Archive_NotFound(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+uuid.New().String()+"/archive", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- TR-01-TASK-32: mapeamento de erros de validação para 422 (P-C249-3) ---

func TestAgentHandler_Update_ValidationError_Returns422(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Agent"}
	svc.updateErr = fmt.Errorf("%w: invalid model config", agent.ErrInvalidModelConfig)

	name := "new name"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &name})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestAgentHandler_Update_SkillIDsError_Returns422(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Agent"}
	svc.updateErr = fmt.Errorf("%w: skill not found", agent.ErrInvalidSkillIDs)

	name := "new name"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &name})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestAgentHandler_Update_NestedModelConfig_Returns422(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Agent"}
	svc.updateErr = fmt.Errorf("%w: nested config", agent.ErrInvalidRequest)

	name := "new name"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &name})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// --- TR-01-TASK-37: publish validation (P-C278-1) ---

func TestAgentHandler_Publish_InvalidStatusTransition_Returns422(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Published Agent", Status: string(agent.StatusPublished)}
	svc.publishErr = fmt.Errorf("%w: agent is already published", agent.ErrInvalidStatusTransition)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+id.String()+"/publish", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestAgentHandler_Publish_InvalidID_Returns400(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodPost, "/api/agents/not-valid/publish", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Bulk delete tests (ACT-F3-19 / P-C341-1).

func TestAgentHandler_BulkDelete_Success(t *testing.T) {
	r, svc := setupAgent()
	id1 := uuid.New()
	id2 := uuid.New()
	svc.agents[id1] = agent.AgentResponse{ID: id1, Name: "a1"}
	svc.agents[id2] = agent.AgentResponse{ID: id2, Name: "a2"}

	body, _ := json.Marshal(map[string]interface{}{"ids": []uuid.UUID{id1, id2}})
	req := httptest.NewRequest(http.MethodDelete, "/api/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]int
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, 2, resp["deleted"])
	assert.NotContains(t, svc.agents, id1)
	assert.NotContains(t, svc.agents, id2)
}

func TestAgentHandler_BulkDelete_EmptyIDs_Returns400(t *testing.T) {
	r, _ := setupAgent()
	body, _ := json.Marshal(map[string]interface{}{"ids": []uuid.UUID{}})
	req := httptest.NewRequest(http.MethodDelete, "/api/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentHandler_BulkDelete_PartialSuccess_SkipsNotFound(t *testing.T) {
	r, svc := setupAgent()
	existingID := uuid.New()
	missingID := uuid.New()
	svc.agents[existingID] = agent.AgentResponse{ID: existingID, Name: "exists"}

	body, _ := json.Marshal(map[string]interface{}{"ids": []uuid.UUID{existingID, missingID}})
	req := httptest.NewRequest(http.MethodDelete, "/api/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]int
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, 1, resp["deleted"])
}
