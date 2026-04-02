package pipeline_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/pipeline"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockPipelineSvc satisfies the private pipelineService interface in pipeline.Handler.
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

func (m *mockPipelineSvc) Create(_ context.Context, req pipeline.CreateRequest) (pipeline.Response, error) {
	id := uuid.New()
	resp := pipeline.Response{ID: id, Name: req.Name, AgentID: req.AgentID}
	m.pipelines[id] = resp
	return resp, nil
}

func (m *mockPipelineSvc) GetByID(_ context.Context, id uuid.UUID) (pipeline.Response, error) {
	p, ok := m.pipelines[id]
	if !ok {
		return pipeline.Response{}, pipeline.ErrNotFound
	}
	return p, nil
}

func (m *mockPipelineSvc) Update(_ context.Context, id uuid.UUID, req pipeline.UpdateRequest) (pipeline.Response, error) {
	p, ok := m.pipelines[id]
	if !ok {
		return pipeline.Response{}, pipeline.ErrNotFound
	}
	p.Name = req.Name
	m.pipelines[id] = p
	return p, nil
}

func (m *mockPipelineSvc) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.pipelines[id]; !ok {
		return pipeline.ErrNotFound
	}
	delete(m.pipelines, id)
	return nil
}

func (m *mockPipelineSvc) ReplaceNodes(_ context.Context, pipelineID uuid.UUID, nodes []pipeline.NodeRequest) ([]pipeline.NodeResponse, error) {
	if _, ok := m.pipelines[pipelineID]; !ok {
		return nil, pipeline.ErrNotFound
	}
	resp := make([]pipeline.NodeResponse, len(nodes))
	for i, n := range nodes {
		resp[i] = pipeline.NodeResponse{ID: uuid.New(), PipelineID: pipelineID, NodeType: n.NodeType, Name: n.Name}
	}
	return resp, nil
}

func (m *mockPipelineSvc) ReplaceEdges(_ context.Context, pipelineID uuid.UUID, edges []pipeline.EdgeRequest) ([]pipeline.EdgeResponse, error) {
	if _, ok := m.pipelines[pipelineID]; !ok {
		return nil, pipeline.ErrNotFound
	}
	resp := make([]pipeline.EdgeResponse, len(edges))
	for i, e := range edges {
		resp[i] = pipeline.EdgeResponse{ID: uuid.New(), PipelineID: pipelineID, SourceNodeID: e.SourceNodeID, TargetNodeID: e.TargetNodeID}
	}
	return resp, nil
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

func (m *mockPipelineSvc) UpdateGraph(_ context.Context, id uuid.UUID, req pipeline.GraphRequest) (pipeline.GraphResponse, error) {
	if _, ok := m.pipelines[id]; !ok {
		return pipeline.GraphResponse{}, pipeline.ErrNotFound
	}
	return pipeline.GraphResponse{
		Nodes:        []pipeline.GraphNodeResponse{},
		Edges:        []pipeline.GraphEdgeResponse{},
		BlocklyState: req.BlocklyState,
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
	var page pagination.Page[pipeline.Response]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestPipelineHandler_Create_Success(t *testing.T) {
	r, _ := setupPipeline()
	body, _ := json.Marshal(pipeline.CreateRequest{Name: "New Pipeline", AgentID: uuid.New()})
	req := httptest.NewRequest(http.MethodPost, "/api/pipelines", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp pipeline.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "New Pipeline", resp.Name)
}

func TestPipelineHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupPipeline()
	req := httptest.NewRequest(http.MethodPost, "/api/pipelines", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPipelineHandler_Create_MissingName(t *testing.T) {
	r, _ := setupPipeline()
	body, _ := json.Marshal(pipeline.CreateRequest{Name: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/pipelines", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestPipelineHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupPipeline()
	req := httptest.NewRequest(http.MethodGet, "/api/pipelines/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPipelineHandler_Delete_Success(t *testing.T) {
	r, svc := setupPipeline()
	id := uuid.New()
	svc.pipelines[id] = pipeline.Response{ID: id, Name: "To Delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/pipelines/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestPipelineHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupPipeline()
	req := httptest.NewRequest(http.MethodDelete, "/api/pipelines/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPipelineHandler_ReplaceNodes_Success(t *testing.T) {
	r, svc := setupPipeline()
	id := uuid.New()
	svc.pipelines[id] = pipeline.Response{ID: id, Name: "Pipeline"}

	nodes := []pipeline.NodeRequest{{NodeType: "INPUT", Name: "Start"}}
	body, _ := json.Marshal(nodes)
	req := httptest.NewRequest(http.MethodPut, "/api/pipelines/"+id.String()+"/nodes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPipelineHandler_GetGraph_Success(t *testing.T) {
	r, svc := setupPipeline()
	id := uuid.New()
	svc.pipelines[id] = pipeline.Response{ID: id, Name: "Pipeline"}

	req := httptest.NewRequest(http.MethodGet, "/api/pipelines/"+id.String()+"/graph", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
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

func TestPipelineHandler_UpdateGraph_Success(t *testing.T) {
	r, svc := setupPipeline()
	id := uuid.New()
	svc.pipelines[id] = pipeline.Response{ID: id, Name: "Pipeline"}

	graphReq := pipeline.GraphRequest{
		Nodes: []pipeline.GraphNodeRequest{
			{ID: "n1", Type: "INPUT", Label: "Start"},
		},
		Edges: []pipeline.GraphEdgeRequest{},
	}
	body, _ := json.Marshal(graphReq)
	req := httptest.NewRequest(http.MethodPut, "/api/pipelines/"+id.String()+"/graph", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp pipeline.GraphResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Nodes)
}

func TestPipelineHandler_UpdateGraph_NotFound(t *testing.T) {
	r, _ := setupPipeline()
	body, _ := json.Marshal(pipeline.GraphRequest{})
	req := httptest.NewRequest(http.MethodPut, "/api/pipelines/"+uuid.New().String()+"/graph", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPipelineHandler_UpdateGraph_InvalidBody(t *testing.T) {
	r, _ := setupPipeline()
	req := httptest.NewRequest(http.MethodPut, "/api/pipelines/"+uuid.New().String()+"/graph", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
