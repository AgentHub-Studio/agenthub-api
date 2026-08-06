package knowledgebase_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledge"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase/graph"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockKBSvc satisfies the private knowledgebaseService interface in knowledgebase.Handler.
type mockKBSvc struct {
	kbs         map[uuid.UUID]knowledgebase.KnowledgeBaseResponse
	createCalls int
	updateCalls int
}

func newMockKBSvc() *mockKBSvc {
	return &mockKBSvc{kbs: make(map[uuid.UUID]knowledgebase.KnowledgeBaseResponse)}
}

func (m *mockKBSvc) List(_ context.Context, req pagination.PageRequest, filters knowledgebase.ListFilters) (pagination.Page[knowledgebase.KnowledgeBaseResponse], error) {
	items := make([]knowledgebase.KnowledgeBaseResponse, 0, len(m.kbs))
	for _, k := range m.kbs {
		if filters.Status != nil && k.Status != *filters.Status {
			continue
		}
		items = append(items, k)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockKBSvc) Create(_ context.Context, req knowledgebase.CreateRequest) (knowledgebase.KnowledgeBaseResponse, error) {
	m.createCalls++
	id := uuid.New()
	resp := knowledgebase.KnowledgeBaseResponse{
		ID:             id,
		Name:           req.Name,
		Description:    req.Description,
		Status:         knowledgebase.StatusActive,
		RerankStrategy: knowledgebase.RerankStrategyNone,
		GraphEnabled:   req.GraphEnabled || req.GraphEnabledSnake,
	}
	if req.RerankStrategy != "" {
		resp.RerankStrategy = knowledgebase.RerankStrategy(req.RerankStrategy)
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
	m.updateCalls++
	k, ok := m.kbs[id]
	if !ok {
		return knowledgebase.KnowledgeBaseResponse{}, knowledgebase.ErrNotFound
	}
	if req.Name != nil {
		k.Name = *req.Name
	}
	if req.RerankStrategy != nil {
		k.RerankStrategy = knowledgebase.RerankStrategy(*req.RerankStrategy)
	}
	if req.GraphEnabled != nil {
		k.GraphEnabled = *req.GraphEnabled
	}
	m.kbs[id] = k
	return k, nil
}

type mockDocumentSearchClient struct {
	results []knowledge.SearchResult
	err     error
	opts    knowledge.SearchOptions
	calls   int
}

func (m *mockDocumentSearchClient) Search(_ context.Context, _ string, opts knowledge.SearchOptions) ([]knowledge.SearchResult, error) {
	m.calls++
	m.opts = opts
	return m.results, m.err
}

type mockGraphRepo struct {
	snapshot graph.PersistedGraph
	result   graph.SearchResult
	found    bool
	err      error
}

func (m mockGraphRepo) List(_ context.Context, _ uuid.UUID) (graph.PersistedGraph, error) {
	return m.snapshot, m.err
}

func (m mockGraphRepo) SearchReportsTo(_ context.Context, _ uuid.UUID, _ string, _ int) (graph.SearchResult, bool, error) {
	return m.result, m.found, m.err
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
	return setupKnowledgeBaseWithRoles("admin")
}

func setupKnowledgeBaseWithRoles(roles ...string) (*chi.Mux, *mockKBSvc) {
	svc := newMockKBSvc()
	h := knowledgebase.NewHandler(svc)
	return setupKnowledgeBaseRouter(h, roles...), svc
}

func setupKnowledgeBaseRouter(h *knowledgebase.Handler, roles ...string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)
	return r
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

func TestKnowledgeBaseHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupKnowledgeBaseWithRoles("user")
	id := uuid.NewString()
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/knowledge-bases"},
		{name: "create", method: http.MethodPost, path: "/api/knowledge-bases", body: `{"name":"kb"}`},
		{name: "get", method: http.MethodGet, path: "/api/knowledge-bases/" + id},
		{name: "put", method: http.MethodPut, path: "/api/knowledge-bases/" + id, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/knowledge-bases/" + id, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/knowledge-bases/" + id},
		{name: "activate", method: http.MethodPost, path: "/api/knowledge-bases/" + id + "/activate"},
		{name: "pause", method: http.MethodPost, path: "/api/knowledge-bases/" + id + "/pause"},
		{name: "entities", method: http.MethodGet, path: "/api/knowledge-bases/" + id + "/entities"},
		{name: "search", method: http.MethodPost, path: "/api/knowledge-bases/" + id + "/search", body: `{"query":"query"}`},
	}

	for _, tc := range cases {
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

func TestKnowledgeBaseHandler_List_FiltersByStatus(t *testing.T) {
	r, svc := setupKnowledgeBase()
	activeID := uuid.New()
	pausedID := uuid.New()
	svc.kbs[activeID] = knowledgebase.KnowledgeBaseResponse{ID: activeID, Name: "Active KB", Status: knowledgebase.StatusActive}
	svc.kbs[pausedID] = knowledgebase.KnowledgeBaseResponse{ID: pausedID, Name: "Paused KB", Status: knowledgebase.StatusPaused}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-bases?status=PAUSED", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[knowledgebase.KnowledgeBaseResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Content, 1)
	assert.Equal(t, pausedID, page.Content[0].ID)
	assert.Equal(t, knowledgebase.StatusPaused, page.Content[0].Status)
}

func TestKnowledgeBaseHandler_List_InvalidStatusFilter(t *testing.T) {
	r, _ := setupKnowledgeBase()
	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-bases?status=INVALID", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
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

func TestKnowledgeBaseHandler_Create_RejectsTrailingJSONWithoutCreating(t *testing.T) {
	r, svc := setupKnowledgeBase()
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases", bytes.NewBufferString(`{"name":"first"}{"name":"ignored"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, svc.createCalls)
}

func TestKnowledgeBaseHandler_Update_RejectsTrailingJSONWithoutUpdating(t *testing.T) {
	r, svc := setupKnowledgeBase()
	id := uuid.New()
	svc.kbs[id] = knowledgebase.KnowledgeBaseResponse{ID: id, Name: "Original", Status: knowledgebase.StatusActive}
	req := httptest.NewRequest(http.MethodPatch, "/api/knowledge-bases/"+id.String(), bytes.NewBufferString(`{"description":"first"}{"name":"ignored"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, svc.updateCalls)
}

func TestKnowledgeBaseHandler_RejectsConflictingAliasesWithoutCallingService(t *testing.T) {
	conflicting := `{"rerankStrategy":"llm","rerank_strategy":"rrf","graphEnabled":false,"graph_enabled":true}`

	t.Run("create", func(t *testing.T) {
		r, svc := setupKnowledgeBase()
		req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases", bytes.NewBufferString(`{"name":"conflicting",`+conflicting[1:]))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Zero(t, svc.createCalls)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupKnowledgeBase()
		id := uuid.New()
		svc.kbs[id] = knowledgebase.KnowledgeBaseResponse{ID: id, Name: "Original", Status: knowledgebase.StatusActive}
		req := httptest.NewRequest(http.MethodPatch, "/api/knowledge-bases/"+id.String(), bytes.NewBufferString(conflicting))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Zero(t, svc.updateCalls)
	})
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

func TestKnowledgeBaseHandler_Search_ReranksLLM(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	firstChunkID := uuid.New()
	bestChunkID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{
		ID:             kbID,
		Name:           "Rerank KB",
		Status:         knowledgebase.StatusActive,
		RerankStrategy: knowledgebase.RerankStrategyLLM,
	}
	h := knowledgebase.NewHandler(svc).WithSearchClient(&mockDocumentSearchClient{results: []knowledge.SearchResult{
		{ChunkID: firstChunkID, DocumentID: uuid.New(), KnowledgeBaseID: kbID, Content: "baseline chunk", Score: 1},
		{ChunkID: bestChunkID, DocumentID: uuid.New(), KnowledgeBaseID: kbID, Content: "final answer yellow square", Score: 0.5},
	}})
	r := setupKnowledgeBaseRouter(h, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewReader([]byte(`{"query":"final answer yellow square","topK":5}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Results []struct {
			ID      uuid.UUID `json:"id"`
			Content string    `json:"content"`
		} `json:"results"`
		RerankStrategy string `json:"rerankStrategy"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 2)
	assert.Equal(t, bestChunkID, resp.Results[0].ID)
	assert.Equal(t, "llm", resp.RerankStrategy)
}

func TestKnowledgeBaseHandler_Search_RerankerFailureFallsBack(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	firstChunkID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{
		ID:             kbID,
		Name:           "Fallback KB",
		Status:         knowledgebase.StatusActive,
		RerankStrategy: knowledgebase.RerankStrategyBrokenReranker,
	}
	h := knowledgebase.NewHandler(svc).WithSearchClient(&mockDocumentSearchClient{results: []knowledge.SearchResult{
		{ChunkID: firstChunkID, DocumentID: uuid.New(), KnowledgeBaseID: kbID, Content: "baseline chunk", Score: 1},
		{ChunkID: uuid.New(), DocumentID: uuid.New(), KnowledgeBaseID: kbID, Content: "better lexical match", Score: 0.5},
	}})
	r := setupKnowledgeBaseRouter(h, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewReader([]byte(`{"query":"better lexical match","topK":5}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Results []struct {
			ID uuid.UUID `json:"id"`
		} `json:"results"`
		RerankerError string `json:"reranker_error"`
		Fallback      string `json:"fallback"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 2)
	assert.Equal(t, firstChunkID, resp.Results[0].ID)
	assert.NotEmpty(t, resp.RerankerError)
	assert.Equal(t, "vector", resp.Fallback)
}

func TestKnowledgeBaseHandler_Entities_ReturnsGraph(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{
		ID:           kbID,
		Name:         "Graph KB",
		Status:       knowledgebase.StatusActive,
		GraphEnabled: true,
	}
	h := knowledgebase.NewHandler(svc).WithGraphRepository(mockGraphRepo{snapshot: graph.PersistedGraph{
		Entities: []graph.EntityRecord{{ID: uuid.New(), KnowledgeBaseID: kbID, Name: "Alice", Type: "person"}},
		Edges:    []graph.EdgeRecord{{ID: uuid.New(), KnowledgeBaseID: kbID, Source: "Alice", Target: "Bob", Relation: "reports_to"}},
	}})
	r := setupKnowledgeBaseRouter(h, "admin")

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-bases/"+kbID.String()+"/entities", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp graph.PersistedGraph
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Entities, 1)
	require.Len(t, resp.Edges, 1)
	assert.Equal(t, "Alice", resp.Entities[0].Name)
	assert.Equal(t, "Bob", resp.Edges[0].Target)
}

func TestKnowledgeBaseHandler_Search_GraphModeUsesGraph(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	edgeID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{
		ID:           kbID,
		Name:         "Graph KB",
		Status:       knowledgebase.StatusActive,
		GraphEnabled: true,
	}
	h := knowledgebase.NewHandler(svc).
		WithSearchClient(&mockDocumentSearchClient{results: []knowledge.SearchResult{}}).
		WithGraphRepository(mockGraphRepo{
			found: true,
			result: graph.SearchResult{
				ID:              edgeID,
				DocumentID:      uuid.New(),
				KnowledgeBaseID: kbID,
				Content:         "Alice reports_to Bob. Chain: Alice -> Bob",
				Score:           1,
				DocumentName:    "knowledge graph",
				Metadata: graph.SearchMetadata{
					Chain: []string{"Alice", "Bob"},
				},
			},
		})
	r := setupKnowledgeBaseRouter(h, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewReader([]byte(`{"query":"Quem Alice reporta?","mode":"graph","topK":5}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Results []struct {
			ID      uuid.UUID `json:"id"`
			Content string    `json:"content"`
		} `json:"results"`
		Graph *graph.SearchMetadata `json:"graph"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 1)
	assert.Equal(t, edgeID, resp.Results[0].ID)
	assert.Contains(t, resp.Results[0].Content, "Bob")
	require.NotNil(t, resp.Graph)
	assert.Equal(t, []string{"Alice", "Bob"}, resp.Graph.Chain)
}

func TestKnowledgeBaseHandler_Search_ForwardsMetadataFilter(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	client := &mockDocumentSearchClient{}
	h := knowledgebase.NewHandler(svc).WithSearchClient(client)
	r := setupKnowledgeBaseRouter(h, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(`{"query":"release notes","metadataFilter":{"field":"tags","op":"containsAny","value":["release"]}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, client.opts.MetadataFilter)
}

func TestKnowledgeBaseHandler_Search_RejectsConflictingLimitAliasesBeforeSearch(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	client := &mockDocumentSearchClient{}
	h := knowledgebase.NewHandler(svc).WithSearchClient(client)
	r := setupKnowledgeBaseRouter(h, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(`{"query":"release notes","limit":2,"topK":3}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, client.calls)
}

func TestKnowledgeBaseHandler_Search_AllowsEquivalentLimitAliases(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	client := &mockDocumentSearchClient{}
	h := knowledgebase.NewHandler(svc).WithSearchClient(client)
	r := setupKnowledgeBaseRouter(h, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(`{"query":"release notes","limit":2,"topK":2}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, client.calls)
	assert.Equal(t, 2, client.opts.TopK)
}

func TestKnowledgeBaseHandler_Search_RejectsTrailingJSONWithoutSearching(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	client := &mockDocumentSearchClient{}
	h := knowledgebase.NewHandler(svc).WithSearchClient(client)
	r := setupKnowledgeBaseRouter(h, "admin")

	body := `{"query":"release notes","metadataFilter":{"field":"tags","op":"containsAny","value":["release"]}}{"query":"ignored"}`
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, client.calls)
}

func TestKnowledgeBaseHandler_Search_RejectsDuplicateMetadataFilterKeyWithoutSearching(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	client := &mockDocumentSearchClient{}
	h := knowledgebase.NewHandler(svc).WithSearchClient(client)
	r := setupKnowledgeBaseRouter(h, "admin")

	body := `{"query":"release notes","metadataFilter":{"field":"source","field":"owner","op":"exists"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, client.calls)
}

func TestKnowledgeBaseHandler_Search_RejectsInvalidMetadataFilter(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	h := knowledgebase.NewHandler(svc).WithSearchClient(&mockDocumentSearchClient{})
	r := setupKnowledgeBaseRouter(h, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(`{"query":"release notes","metadataFilter":{"field":"tags","op":"containsAny","value":[1]}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestKnowledgeBaseHandler_Search_RejectsMetadataOutsidePostgresJSONBNumericRange(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	client := &mockDocumentSearchClient{}
	h := knowledgebase.NewHandler(svc).WithSearchClient(client)
	r := setupKnowledgeBaseRouter(h, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(`{"query":"release notes","metadataFilter":{"field":"year","op":"eq","value":1e131072}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Zero(t, client.calls)
}

func TestKnowledgeBaseHandler_Search_RejectsMetadataWithPostgresJSONBNullCharacter(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	client := &mockDocumentSearchClient{}
	h := knowledgebase.NewHandler(svc).WithSearchClient(client)
	r := setupKnowledgeBaseRouter(h, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(`{"query":"release notes","metadataFilter":{"field":"source","op":"eq","value":"manual\u0000draft"}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Zero(t, client.calls)
}

func TestKnowledgeBaseHandler_Search_RejectsOversizedMetadataFilterBeforeSearch(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	client := &mockDocumentSearchClient{}
	h := knowledgebase.NewHandler(svc).WithSearchClient(client)
	r := setupKnowledgeBaseRouter(h, "admin")

	filter := `{"field":"source","op":"eq","value":"` + strings.Repeat("x", knowledge.MaxMetadataFilterBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(`{"query":"release notes","metadataFilter":`+filter+`}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Zero(t, client.calls)
}

func TestKnowledgeBaseHandler_Search_AcceptsMetadataAtPostgresJSONBNumericBoundary(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	client := &mockDocumentSearchClient{}
	h := knowledgebase.NewHandler(svc).WithSearchClient(client)
	r := setupKnowledgeBaseRouter(h, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(`{"query":"release notes","metadataFilter":{"field":"year","op":"eq","value":1e131071}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 1, client.calls)
}

func TestKnowledgeBaseHandler_Search_RejectsExcessiveMetadataFilterComplexity(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	h := knowledgebase.NewHandler(svc).WithSearchClient(&mockDocumentSearchClient{})
	r := setupKnowledgeBaseRouter(h, "admin")

	values := make([]string, 65)
	for i := range values {
		values[i] = `"release"`
	}
	body := `{"query":"release notes","metadataFilter":{"field":"tags","op":"containsAny","value":[` + strings.Join(values, ",") + `]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestKnowledgeBaseHandler_Search_RejectsMetadataFilterInGraphMode(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Graph KB", Status: knowledgebase.StatusActive}
	h := knowledgebase.NewHandler(svc).
		WithSearchClient(&mockDocumentSearchClient{}).
		WithGraphRepository(mockGraphRepo{found: true})
	r := setupKnowledgeBaseRouter(h, "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(`{"query":"release notes","mode":"graph","metadataFilter":{"field":"source","op":"eq","value":"manual"}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
