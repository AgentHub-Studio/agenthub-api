package agent_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockBindingRepo implements agent.BindingRepository for handler tests.
type mockBindingRepo struct {
	skills map[uuid.UUID][]uuid.UUID
	kbs    map[uuid.UUID][]uuid.UUID
}

func newMockBindingRepo() *mockBindingRepo {
	return &mockBindingRepo{
		skills: make(map[uuid.UUID][]uuid.UUID),
		kbs:    make(map[uuid.UUID][]uuid.UUID),
	}
}

func (m *mockBindingRepo) ListSkillIDs(_ context.Context, agentID uuid.UUID) ([]uuid.UUID, error) {
	ids := m.skills[agentID]
	if ids == nil {
		return []uuid.UUID{}, nil
	}
	return ids, nil
}

func (m *mockBindingRepo) SyncSkills(_ context.Context, agentID uuid.UUID, skillIDs []uuid.UUID) error {
	m.skills[agentID] = skillIDs
	return nil
}

func (m *mockBindingRepo) ListKnowledgeBaseIDs(_ context.Context, agentID uuid.UUID) ([]uuid.UUID, error) {
	ids := m.kbs[agentID]
	if ids == nil {
		return []uuid.UUID{}, nil
	}
	return ids, nil
}

func (m *mockBindingRepo) SyncKnowledgeBases(_ context.Context, agentID uuid.UUID, kbIDs []uuid.UUID) error {
	m.kbs[agentID] = kbIDs
	return nil
}

// mockBindingAgentRepo implements agent.Repository for binding handler tests (only FindByID needed).
type mockBindingAgentRepo struct {
	agents map[uuid.UUID]agent.Agent
}

func newMockAgentRepo() *mockBindingAgentRepo {
	return &mockBindingAgentRepo{agents: make(map[uuid.UUID]agent.Agent)}
}

func (m *mockBindingAgentRepo) FindAll(_ context.Context, _ agent.AgentStatus, _ string, _ pagination.PageRequest) ([]agent.Agent, int64, error) {
	return nil, 0, nil
}

func (m *mockBindingAgentRepo) FindByID(_ context.Context, id uuid.UUID) (agent.Agent, error) {
	a, ok := m.agents[id]
	if !ok {
		return agent.Agent{}, agent.ErrNotFound
	}
	return a, nil
}

func (m *mockBindingAgentRepo) Create(_ context.Context, a agent.Agent) (agent.Agent, error) {
	m.agents[a.ID] = a
	return a, nil
}

func (m *mockBindingAgentRepo) Update(_ context.Context, a agent.Agent) (agent.Agent, error) {
	m.agents[a.ID] = a
	return a, nil
}

func (m *mockBindingAgentRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.agents, id)
	return nil
}

func (m *mockBindingAgentRepo) UpdateStatus(_ context.Context, id uuid.UUID, status agent.AgentStatus) (agent.Agent, error) {
	a := m.agents[id]
	a.Status = status
	m.agents[id] = a
	return a, nil
}

func setupBindingHandler() (*chi.Mux, *mockBindingAgentRepo, *mockBindingRepo) {
	agentRepo := newMockAgentRepo()
	bindingRepo := newMockBindingRepo()
	h := agent.NewBindingHandler(agentRepo, bindingRepo)
	r := chi.NewRouter()
	h.RegisterBindingRoutes(r)
	return r, agentRepo, bindingRepo
}

func seedBindingAgent(repo *mockBindingAgentRepo) uuid.UUID {
	id := uuid.New()
	repo.agents[id] = agent.Agent{ID: id, Name: "Test Agent"}
	return id
}

// --- Skill binding tests ---

func TestBindingHandler_ListSkills_Empty(t *testing.T) {
	r, repo, _ := setupBindingHandler()
	agentID := seedBindingAgent(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/skills", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var ids []uuid.UUID
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ids))
	assert.Empty(t, ids)
}

func TestBindingHandler_SyncSkills_Success(t *testing.T) {
	r, repo, _ := setupBindingHandler()
	agentID := seedBindingAgent(repo)

	skillID1 := uuid.New()
	skillID2 := uuid.New()
	body, _ := json.Marshal(map[string]any{"ids": []string{skillID1.String(), skillID2.String()}})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/skills", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var ids []uuid.UUID
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ids))
	assert.Len(t, ids, 2)
}

func TestBindingHandler_SyncSkills_AgentNotFound(t *testing.T) {
	r, _, _ := setupBindingHandler()

	body, _ := json.Marshal(map[string]any{"ids": []string{}})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+uuid.New().String()+"/skills", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestBindingHandler_SyncSkills_InvalidBody(t *testing.T) {
	r, repo, _ := setupBindingHandler()
	agentID := seedBindingAgent(repo)

	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/skills", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBindingHandler_SyncSkills_ClearAll(t *testing.T) {
	r, repo, bindingRepo := setupBindingHandler()
	agentID := seedBindingAgent(repo)
	bindingRepo.skills[agentID] = []uuid.UUID{uuid.New(), uuid.New()}

	// Sync with empty list clears all bindings.
	body, _ := json.Marshal(map[string]any{"ids": []string{}})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/skills", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var ids []uuid.UUID
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ids))
	assert.Empty(t, ids)
}

// --- Knowledge Base binding tests ---

func TestBindingHandler_ListKBs_Empty(t *testing.T) {
	r, repo, _ := setupBindingHandler()
	agentID := seedBindingAgent(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/knowledge-bases", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var ids []uuid.UUID
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ids))
	assert.Empty(t, ids)
}

func TestBindingHandler_SyncKBs_Success(t *testing.T) {
	r, repo, _ := setupBindingHandler()
	agentID := seedBindingAgent(repo)

	kbID := uuid.New()
	body, _ := json.Marshal(map[string]any{"ids": []string{kbID.String()}})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/knowledge-bases", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var ids []uuid.UUID
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ids))
	assert.Len(t, ids, 1)
	assert.Equal(t, kbID, ids[0])
}

func TestBindingHandler_SyncKBs_AgentNotFound(t *testing.T) {
	r, _, _ := setupBindingHandler()

	body, _ := json.Marshal(map[string]any{"ids": []string{}})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+uuid.New().String()+"/knowledge-bases", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestBindingHandler_InvalidAgentID(t *testing.T) {
	r, _, _ := setupBindingHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/agents/not-a-uuid/skills", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
