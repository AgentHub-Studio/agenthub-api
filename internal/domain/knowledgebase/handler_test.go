package knowledgebase_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockKBSvc satisfies the private knowledgebaseService interface in knowledgebase.Handler.
type mockKBSvc struct {
	kbs map[uuid.UUID]knowledgebase.KnowledgeBaseResponse
}

func newMockKBSvc() *mockKBSvc {
	return &mockKBSvc{kbs: make(map[uuid.UUID]knowledgebase.KnowledgeBaseResponse)}
}

func (m *mockKBSvc) List(_ context.Context, req pagination.PageRequest) (pagination.Page[knowledgebase.KnowledgeBaseResponse], error) {
	items := make([]knowledgebase.KnowledgeBaseResponse, 0, len(m.kbs))
	for _, k := range m.kbs {
		items = append(items, k)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockKBSvc) Create(_ context.Context, req knowledgebase.CreateRequest) (knowledgebase.KnowledgeBaseResponse, error) {
	id := uuid.New()
	resp := knowledgebase.KnowledgeBaseResponse{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Status:      knowledgebase.StatusActive,
	}
	m.kbs[id] = resp
	return resp, nil
}

func (m *mockKBSvc) GetByID(_ context.Context, id uuid.UUID) (knowledgebase.KnowledgeBaseResponse, error) {
	k, ok := m.kbs[id]
	if !ok {
		return knowledgebase.KnowledgeBaseResponse{}, knowledgebase.ErrNotFound
	}
	return k, nil
}

func (m *mockKBSvc) Update(_ context.Context, id uuid.UUID, req knowledgebase.UpdateRequest) (knowledgebase.KnowledgeBaseResponse, error) {
	k, ok := m.kbs[id]
	if !ok {
		return knowledgebase.KnowledgeBaseResponse{}, knowledgebase.ErrNotFound
	}
	if req.Name != nil {
		k.Name = *req.Name
	}
	m.kbs[id] = k
	return k, nil
}

func (m *mockKBSvc) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.kbs[id]; !ok {
		return knowledgebase.ErrNotFound
	}
	delete(m.kbs, id)
	return nil
}

func (m *mockKBSvc) Activate(_ context.Context, id uuid.UUID) (knowledgebase.KnowledgeBaseResponse, error) {
	k, ok := m.kbs[id]
	if !ok {
		return knowledgebase.KnowledgeBaseResponse{}, knowledgebase.ErrNotFound
	}
	k.Status = knowledgebase.StatusActive
	m.kbs[id] = k
	return k, nil
}

func (m *mockKBSvc) Pause(_ context.Context, id uuid.UUID) (knowledgebase.KnowledgeBaseResponse, error) {
	k, ok := m.kbs[id]
	if !ok {
		return knowledgebase.KnowledgeBaseResponse{}, knowledgebase.ErrNotFound
	}
	k.Status = knowledgebase.StatusPaused
	m.kbs[id] = k
	return k, nil
}

func setupKnowledgeBase() (*chi.Mux, *mockKBSvc) {
	svc := newMockKBSvc()
	h := knowledgebase.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestKnowledgeBaseHandler_List_Success(t *testing.T) {
	r, svc := setupKnowledgeBase()
	id := uuid.New()
	svc.kbs[id] = knowledgebase.KnowledgeBaseResponse{ID: id, Name: "My KB", Status: knowledgebase.StatusActive}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-bases", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[knowledgebase.KnowledgeBaseResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestKnowledgeBaseHandler_Create_Success(t *testing.T) {
	r, _ := setupKnowledgeBase()
	body, _ := json.Marshal(knowledgebase.CreateRequest{Name: "Docs KB", Description: "Project docs"})
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp knowledgebase.KnowledgeBaseResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Docs KB", resp.Name)
}

func TestKnowledgeBaseHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupKnowledgeBase()
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestKnowledgeBaseHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupKnowledgeBase()
	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-bases/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestKnowledgeBaseHandler_Delete_Success(t *testing.T) {
	r, svc := setupKnowledgeBase()
	id := uuid.New()
	svc.kbs[id] = knowledgebase.KnowledgeBaseResponse{ID: id, Name: "To Delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/knowledge-bases/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestKnowledgeBaseHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupKnowledgeBase()
	req := httptest.NewRequest(http.MethodDelete, "/api/knowledge-bases/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestKnowledgeBaseHandler_Activate_Success(t *testing.T) {
	r, svc := setupKnowledgeBase()
	id := uuid.New()
	svc.kbs[id] = knowledgebase.KnowledgeBaseResponse{ID: id, Name: "KB", Status: knowledgebase.StatusPaused}

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+id.String()+"/activate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp knowledgebase.KnowledgeBaseResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, knowledgebase.StatusActive, resp.Status)
}

func TestKnowledgeBaseHandler_Pause_NotFound(t *testing.T) {
	r, _ := setupKnowledgeBase()
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+uuid.New().String()+"/pause", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
