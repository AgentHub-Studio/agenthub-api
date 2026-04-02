package mcp_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockMCPSvc satisfies the private mcpService interface in mcp.Handler.
type mockMCPSvc struct {
	configs map[uuid.UUID]mcp.McpServerConfigResponse
}

func newMockMCPSvc() *mockMCPSvc {
	return &mockMCPSvc{configs: make(map[uuid.UUID]mcp.McpServerConfigResponse)}
}

func (m *mockMCPSvc) List(_ context.Context) ([]mcp.McpServerConfigResponse, error) {
	items := make([]mcp.McpServerConfigResponse, 0, len(m.configs))
	for _, c := range m.configs {
		items = append(items, c)
	}
	return items, nil
}

func (m *mockMCPSvc) Create(_ context.Context, req mcp.CreateRequest) (mcp.McpServerConfigResponse, error) {
	id := uuid.New()
	resp := mcp.McpServerConfigResponse{
		ID:            id,
		Name:          req.Name,
		TransportType: req.TransportType,
	}
	m.configs[id] = resp
	return resp, nil
}

func (m *mockMCPSvc) GetByID(_ context.Context, id uuid.UUID) (mcp.McpServerConfigResponse, error) {
	c, ok := m.configs[id]
	if !ok {
		return mcp.McpServerConfigResponse{}, mcp.ErrNotFound
	}
	return c, nil
}

func (m *mockMCPSvc) Update(_ context.Context, id uuid.UUID, req mcp.UpdateRequest) (mcp.McpServerConfigResponse, error) {
	c, ok := m.configs[id]
	if !ok {
		return mcp.McpServerConfigResponse{}, mcp.ErrNotFound
	}
	if req.Name != nil {
		c.Name = *req.Name
	}
	m.configs[id] = c
	return c, nil
}

func (m *mockMCPSvc) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.configs[id]; !ok {
		return mcp.ErrNotFound
	}
	delete(m.configs, id)
	return nil
}

func setupMCP() (*chi.Mux, *mockMCPSvc) {
	svc := newMockMCPSvc()
	h := mcp.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestMCPHandler_List_Success(t *testing.T) {
	r, svc := setupMCP()
	id := uuid.New()
	svc.configs[id] = mcp.McpServerConfigResponse{ID: id, Name: "filesystem", TransportType: "stdio"}

	req := httptest.NewRequest(http.MethodGet, "/api/mcp-server-configs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[mcp.McpServerConfigResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestMCPHandler_Create_Success(t *testing.T) {
	r, _ := setupMCP()
	body, _ := json.Marshal(mcp.CreateRequest{Name: "my-server", TransportType: "stdio"})
	req := httptest.NewRequest(http.MethodPost, "/api/mcp-server-configs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp mcp.McpServerConfigResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "my-server", resp.Name)
}

func TestMCPHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupMCP()
	req := httptest.NewRequest(http.MethodPost, "/api/mcp-server-configs", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMCPHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupMCP()
	req := httptest.NewRequest(http.MethodGet, "/api/mcp-server-configs/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMCPHandler_Delete_Success(t *testing.T) {
	r, svc := setupMCP()
	id := uuid.New()
	svc.configs[id] = mcp.McpServerConfigResponse{ID: id, Name: "to-delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/mcp-server-configs/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestMCPHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupMCP()
	req := httptest.NewRequest(http.MethodDelete, "/api/mcp-server-configs/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
