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

// mockVersionSvc implements agent.VersionService for handler tests.
type mockVersionSvc struct {
	versions map[uuid.UUID]agent.AgentVersionResponse
	drafts   map[uuid.UUID]agent.AgentVersionResponse // keyed by agentID
}

func newMockVersionSvc() *mockVersionSvc {
	return &mockVersionSvc{
		versions: make(map[uuid.UUID]agent.AgentVersionResponse),
		drafts:   make(map[uuid.UUID]agent.AgentVersionResponse),
	}
}

func (m *mockVersionSvc) ListVersions(_ context.Context, agentID uuid.UUID, req pagination.PageRequest) (pagination.Page[agent.AgentVersionResponse], error) {
	var items []agent.AgentVersionResponse
	for _, v := range m.versions {
		if v.AgentID == agentID {
			items = append(items, v)
		}
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockVersionSvc) CreateDraft(_ context.Context, agentID uuid.UUID, req agent.CreateAgentVersionRequest) (agent.AgentVersionResponse, error) {
	if _, exists := m.drafts[agentID]; exists {
		return agent.AgentVersionResponse{}, agent.ErrDraftAlreadyExists
	}
	id := uuid.New()
	resp := agent.AgentVersionResponse{
		ID:            id,
		AgentID:       agentID,
		VersionNumber: 1,
		Status:        "DRAFT",
		Description:   req.Description,
	}
	m.versions[id] = resp
	m.drafts[agentID] = resp
	return resp, nil
}

func (m *mockVersionSvc) GetDraft(_ context.Context, agentID uuid.UUID) (agent.AgentVersionResponse, error) {
	v, ok := m.drafts[agentID]
	if !ok {
		return agent.AgentVersionResponse{}, agent.ErrVersionNotFound
	}
	return v, nil
}

func (m *mockVersionSvc) GetLatestPublished(_ context.Context, agentID uuid.UUID) (agent.AgentVersionResponse, error) {
	for _, v := range m.versions {
		if v.AgentID == agentID && v.Status == "PUBLISHED" {
			return v, nil
		}
	}
	return agent.AgentVersionResponse{}, agent.ErrVersionNotFound
}

func (m *mockVersionSvc) UpdateDraft(_ context.Context, versionID uuid.UUID, req agent.UpdateAgentVersionRequest) (agent.AgentVersionResponse, error) {
	v, ok := m.versions[versionID]
	if !ok {
		return agent.AgentVersionResponse{}, agent.ErrVersionNotFound
	}
	if v.Status != "DRAFT" {
		return agent.AgentVersionResponse{}, agent.ErrVersionImmutable
	}
	if req.Description != nil {
		v.Description = *req.Description
	}
	m.versions[versionID] = v
	m.drafts[v.AgentID] = v
	return v, nil
}

func (m *mockVersionSvc) Publish(_ context.Context, versionID uuid.UUID) (agent.AgentVersionResponse, error) {
	v, ok := m.versions[versionID]
	if !ok {
		return agent.AgentVersionResponse{}, agent.ErrVersionNotFound
	}
	if v.Status != "DRAFT" {
		return agent.AgentVersionResponse{}, agent.ErrVersionImmutable
	}
	v.Status = "PUBLISHED"
	m.versions[versionID] = v
	delete(m.drafts, v.AgentID)
	return v, nil
}

func (m *mockVersionSvc) Rollback(_ context.Context, agentID, versionID uuid.UUID) (agent.AgentVersionResponse, error) {
	v, ok := m.versions[versionID]
	if !ok {
		return agent.AgentVersionResponse{}, agent.ErrVersionNotFound
	}
	if v.AgentID != agentID {
		return agent.AgentVersionResponse{}, agent.ErrVersionNotFound
	}
	return v, nil
}

func (m *mockVersionSvc) GetVersionByID(_ context.Context, versionID uuid.UUID) (agent.AgentVersionResponse, error) {
	v, ok := m.versions[versionID]
	if !ok {
		return agent.AgentVersionResponse{}, agent.ErrVersionNotFound
	}
	return v, nil
}

func setupVersionRouter() (*chi.Mux, *mockVersionSvc) {
	svc := newMockVersionSvc()
	h := agent.NewVersionHandler(svc)
	r := chi.NewRouter()
	h.RegisterVersionRoutes(r)
	return r, svc
}

func TestVersionHandler_List_Empty(t *testing.T) {
	r, _ := setupVersionRouter()
	agentID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/versions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[agent.AgentVersionResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(0), page.TotalElements)
}

func TestVersionHandler_List_WithVersions(t *testing.T) {
	r, svc := setupVersionRouter()
	agentID := uuid.New()
	vID := uuid.New()
	svc.versions[vID] = agent.AgentVersionResponse{ID: vID, AgentID: agentID, Status: "DRAFT", VersionNumber: 1}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/versions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[agent.AgentVersionResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestVersionHandler_List_InvalidAgentID(t *testing.T) {
	r, _ := setupVersionRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/agents/not-a-uuid/versions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestVersionHandler_CreateDraft_Success(t *testing.T) {
	r, _ := setupVersionRouter()
	agentID := uuid.New()

	body, _ := json.Marshal(map[string]any{"description": "v1 draft"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/versions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp agent.AgentVersionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, agentID, resp.AgentID)
	assert.Equal(t, "DRAFT", resp.Status)
	assert.Equal(t, "v1 draft", resp.Description)
}

func TestVersionHandler_CreateDraft_Conflict(t *testing.T) {
	r, _ := setupVersionRouter()
	agentID := uuid.New()

	// Create first draft
	body, _ := json.Marshal(map[string]any{"description": "first"})
	req1 := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/versions", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusCreated, w1.Code)

	// Second draft must conflict
	body2, _ := json.Marshal(map[string]any{"description": "second"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/versions", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusConflict, w2.Code)
}

func TestVersionHandler_GetDraft_Success(t *testing.T) {
	r, svc := setupVersionRouter()
	agentID := uuid.New()
	vID := uuid.New()
	svc.versions[vID] = agent.AgentVersionResponse{ID: vID, AgentID: agentID, Status: "DRAFT"}
	svc.drafts[agentID] = svc.versions[vID]

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/versions/draft", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp agent.AgentVersionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "DRAFT", resp.Status)
}

func TestVersionHandler_GetDraft_NotFound(t *testing.T) {
	r, _ := setupVersionRouter()
	agentID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/versions/draft", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestVersionHandler_GetLatestPublished_Success(t *testing.T) {
	r, svc := setupVersionRouter()
	agentID := uuid.New()
	vID := uuid.New()
	svc.versions[vID] = agent.AgentVersionResponse{ID: vID, AgentID: agentID, Status: "PUBLISHED", VersionNumber: 1}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/versions/latest-published", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp agent.AgentVersionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "PUBLISHED", resp.Status)
}

func TestVersionHandler_GetLatestPublished_NotFound(t *testing.T) {
	r, _ := setupVersionRouter()
	agentID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/versions/latest-published", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestVersionHandler_UpdateDraft_Success(t *testing.T) {
	r, svc := setupVersionRouter()
	agentID := uuid.New()
	vID := uuid.New()
	svc.versions[vID] = agent.AgentVersionResponse{ID: vID, AgentID: agentID, Status: "DRAFT"}
	svc.drafts[agentID] = svc.versions[vID]

	newDesc := "updated description"
	body, _ := json.Marshal(map[string]any{"description": newDesc})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/versions/by-id/"+vID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp agent.AgentVersionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, newDesc, resp.Description)
}

func TestVersionHandler_UpdateDraft_NotFound(t *testing.T) {
	r, _ := setupVersionRouter()
	agentID := uuid.New()
	vID := uuid.New()

	body, _ := json.Marshal(map[string]any{"description": "x"})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/versions/by-id/"+vID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestVersionHandler_UpdateDraft_ImmutablePublished(t *testing.T) {
	r, svc := setupVersionRouter()
	agentID := uuid.New()
	vID := uuid.New()
	svc.versions[vID] = agent.AgentVersionResponse{ID: vID, AgentID: agentID, Status: "PUBLISHED"}

	body, _ := json.Marshal(map[string]any{"description": "attempt"})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+agentID.String()+"/versions/by-id/"+vID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestVersionHandler_Publish_Success(t *testing.T) {
	r, svc := setupVersionRouter()
	agentID := uuid.New()
	vID := uuid.New()
	svc.versions[vID] = agent.AgentVersionResponse{ID: vID, AgentID: agentID, Status: "DRAFT"}
	svc.drafts[agentID] = svc.versions[vID]

	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/versions/"+vID.String()+"/publish", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp agent.AgentVersionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "PUBLISHED", resp.Status)
}

func TestVersionHandler_Publish_AlreadyPublished(t *testing.T) {
	r, svc := setupVersionRouter()
	agentID := uuid.New()
	vID := uuid.New()
	svc.versions[vID] = agent.AgentVersionResponse{ID: vID, AgentID: agentID, Status: "PUBLISHED"}

	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/versions/"+vID.String()+"/publish", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestVersionHandler_Publish_NotFound(t *testing.T) {
	r, _ := setupVersionRouter()
	agentID := uuid.New()
	vID := uuid.New()

	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/versions/"+vID.String()+"/publish", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
