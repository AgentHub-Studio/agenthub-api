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

func (m *mockMCPSvc) ListAutoStart(_ context.Context) ([]mcp.McpServerConfigResponse, error) {
	var items []mcp.McpServerConfigResponse
	for _, c := range m.configs {
		if c.AutoStart {
			items = append(items, c)
		}
	}
	return items, nil
}

func (m *mockMCPSvc) ListAllEnabled(_ context.Context) ([]mcp.McpServerConfigResponse, error) {
	var items []mcp.McpServerConfigResponse
	for _, c := range m.configs {
		if c.Enabled {
			items = append(items, c)
		}
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

func (m *mockMCPSvc) GetAuthStatus(_ context.Context, _ uuid.UUID) (mcp.AuthStatusResponse, error) {
	return mcp.AuthStatusResponse{}, nil
}

func (m *mockMCPSvc) GetConnectURL(_ context.Context, _ uuid.UUID, _ string) (mcp.ConnectURLResponse, error) {
	return mcp.ConnectURLResponse{}, nil
}

func (m *mockMCPSvc) HandleOAuthCallback(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockMCPSvc) ListTools(_ context.Context, _ uuid.UUID) ([]mcp.ToolResponse, error) {
	return nil, nil
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

// Bootstrap endpoint tests (ACT-F3-12 / P-C351-1).

// BUG-MCP-RUNTIME-STALE fix: bootstrap now returns all enabled configs (not just auto_start=true).
// Servers with auto_start=false but enabled=true are also registered so agents can reach them.

func TestMCPHandler_Bootstrap_ReturnsAllEnabledConfigs(t *testing.T) {
	r, svc := setupMCP()
	// enabled=true, auto_start=true — must appear.
	id1 := uuid.New()
	svc.configs[id1] = mcp.McpServerConfigResponse{ID: id1, Name: "auto-enabled", AutoStart: true, Enabled: true}
	// enabled=true, auto_start=false — must also appear (BUG-MCP-RUNTIME-STALE fix).
	id2 := uuid.New()
	svc.configs[id2] = mcp.McpServerConfigResponse{ID: id2, Name: "manual-enabled", AutoStart: false, Enabled: true}
	// enabled=false — must NOT appear.
	id3 := uuid.New()
	svc.configs[id3] = mcp.McpServerConfigResponse{ID: id3, Name: "disabled", AutoStart: true, Enabled: false}

	req := httptest.NewRequest(http.MethodGet, "/api/mcp-server-configs/bootstrap", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var items []mcp.McpServerConfigResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&items))
	require.Len(t, items, 2)
	names := make(map[string]bool)
	for _, it := range items {
		names[it.Name] = true
	}
	assert.True(t, names["auto-enabled"])
	assert.True(t, names["manual-enabled"])
	assert.False(t, names["disabled"])
}

func TestMCPHandler_Bootstrap_EmptyWhenAllDisabled(t *testing.T) {
	r, svc := setupMCP()
	id := uuid.New()
	svc.configs[id] = mcp.McpServerConfigResponse{ID: id, Name: "disabled", AutoStart: true, Enabled: false}

	req := httptest.NewRequest(http.MethodGet, "/api/mcp-server-configs/bootstrap", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var items []mcp.McpServerConfigResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&items))
	assert.Len(t, items, 0)
}
