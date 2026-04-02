package pipeline_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/pipeline"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockPipelineSvc satisfies the read-only pipelineService interface in pipeline.Handler.
type mockPipelineSvc struct {
	pipelines map[uuid.UUID]pipeline.Response
}

func newMockPipelineSvc() *mockPipelineSvc {
	return &mockPipelineSvc{pipelines: make(map[uuid.UUID]pipeline.Response)}
}

func (m *mockPipelineSvc) List(_ context.Context, req pagination.PageRequest) (pagination.Page[pipeline.Response], error) {
	items := make([]pipeline.Response, 0, len(m.pipelines))
	for _, p := range m.pipelines {
		items = append(items, p)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockPipelineSvc) GetByID(_ context.Context, id uuid.UUID) (pipeline.Response, error) {
	p, ok := m.pipelines[id]
	if !ok {
		return pipeline.Response{}, pipeline.ErrNotFound
	}
	return p, nil
}

func (m *mockPipelineSvc) GetGraph(_ context.Context, id uuid.UUID) (pipeline.GraphResponse, error) {
	if _, ok := m.pipelines[id]; !ok {
		return pipeline.GraphResponse{}, pipeline.ErrNotFound
	}
	return pipeline.GraphResponse{
		Nodes: []pipeline.GraphNodeResponse{},
		Edges: []pipeline.GraphEdgeResponse{},
	}, nil
}

func setupPipeline() (*chi.Mux, *mockPipelineSvc) {
	svc := newMockPipelineSvc()
	h := pipeline.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestPipelineHandler_List_Success(t *testing.T) {
	r, svc := setupPipeline()
	id := uuid.New()
	svc.pipelines[id] = pipeline.Response{ID: id, Name: "My Pipeline"}

	req := httptest.NewRequest(http.MethodGet, "/api/pipelines", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "true", w.Header().Get("Deprecated"))
	assert.Equal(t, "2026-07-01", w.Header().Get("Sunset"))
	var page pagination.Page[pipeline.Response]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestPipelineHandler_GetByID_Success(t *testing.T) {
	r, svc := setupPipeline()
	id := uuid.New()
	svc.pipelines[id] = pipeline.Response{ID: id, Name: "My Pipeline"}

	req := httptest.NewRequest(http.MethodGet, "/api/pipelines/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "true", w.Header().Get("Deprecated"))
	var resp pipeline.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "My Pipeline", resp.Name)
}

func TestPipelineHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupPipeline()
	req := httptest.NewRequest(http.MethodGet, "/api/pipelines/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPipelineHandler_GetByID_InvalidID(t *testing.T) {
	r, _ := setupPipeline()
	req := httptest.NewRequest(http.MethodGet, "/api/pipelines/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineHandler_GetGraph_Success(t *testing.T) {
	r, svc := setupPipeline()
	id := uuid.New()
	svc.pipelines[id] = pipeline.Response{ID: id, Name: "Pipeline"}

	req := httptest.NewRequest(http.MethodGet, "/api/pipelines/"+id.String()+"/graph", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "true", w.Header().Get("Deprecated"))
	var resp pipeline.GraphResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Nodes)
	assert.NotNil(t, resp.Edges)
}

func TestPipelineHandler_GetGraph_NotFound(t *testing.T) {
	r, _ := setupPipeline()
	req := httptest.NewRequest(http.MethodGet, "/api/pipelines/"+uuid.New().String()+"/graph", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Verify write endpoints are no longer accessible (removed routes).

func TestPipelineHandler_WriteEndpoints_Removed(t *testing.T) {
	r, _ := setupPipeline()

	tests := []struct {
		method string
		path   string
		expect int
	}{
		// Routes that have GET equivalents → chi returns 405 Method Not Allowed.
		{http.MethodPost, "/api/pipelines", http.StatusMethodNotAllowed},
		{http.MethodPut, "/api/pipelines/" + uuid.New().String(), http.StatusMethodNotAllowed},
		{http.MethodPatch, "/api/pipelines/" + uuid.New().String(), http.StatusMethodNotAllowed},
		{http.MethodDelete, "/api/pipelines/" + uuid.New().String(), http.StatusMethodNotAllowed},
		{http.MethodPut, "/api/pipelines/" + uuid.New().String() + "/graph", http.StatusMethodNotAllowed},
		// Routes with no GET equivalent → chi returns 404 Not Found.
		{http.MethodPut, "/api/pipelines/" + uuid.New().String() + "/nodes", http.StatusNotFound},
		{http.MethodPut, "/api/pipelines/" + uuid.New().String() + "/edges", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expect, w.Code)
		})
	}
}
