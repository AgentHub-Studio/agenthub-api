package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockIntegrationSvc struct {
	page     pagination.Page[integration.Response]
	filters  integration.ListFilters
	called   bool
	http     integration.HTTPResponse
	lastReq  integration.HTTPCreateRequest
	deleted  uuid.UUID
	database integration.DatabaseResponse
	dbReq    integration.DatabaseCreateRequest
	mcp      mcp.McpServerConfigResponse
	mcpReq   mcp.CreateRequest
	err      error
}

func (m *mockIntegrationSvc) List(_ context.Context, _ pagination.PageRequest, filters integration.ListFilters) (pagination.Page[integration.Response], error) {
	m.called = true
	m.filters = filters
	if m.err != nil {
		return pagination.Page[integration.Response]{}, m.err
	}
	return m.page, nil
}

func (m *mockIntegrationSvc) CreateHTTP(_ context.Context, req integration.HTTPCreateRequest) (integration.HTTPResponse, error) {
	m.lastReq = req
	if m.err != nil {
		return integration.HTTPResponse{}, m.err
	}
	if m.http.ID == uuid.Nil {
		m.http = integration.HTTPResponse{ID: uuid.New(), Name: req.Name, URL: req.URL, Method: req.Method}
	}
	return m.http, nil
}

func (m *mockIntegrationSvc) GetHTTP(_ context.Context, id uuid.UUID) (integration.HTTPResponse, error) {
	if m.err != nil {
		return integration.HTTPResponse{}, m.err
	}
	if m.http.ID == uuid.Nil || m.http.ID != id {
		return integration.HTTPResponse{}, tool.ErrNotFound
	}
	return m.http, nil
}

func (m *mockIntegrationSvc) UpdateHTTP(_ context.Context, id uuid.UUID, req integration.HTTPCreateRequest) (integration.HTTPResponse, error) {
	m.lastReq = req
	if m.err != nil {
		return integration.HTTPResponse{}, m.err
	}
	if m.http.ID == uuid.Nil || m.http.ID != id {
		return integration.HTTPResponse{}, tool.ErrNotFound
	}
	m.http.Name = req.Name
	m.http.URL = req.URL
	m.http.Method = req.Method
	return m.http, nil
}

func (m *mockIntegrationSvc) DeleteHTTP(_ context.Context, id uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	if m.http.ID == uuid.Nil || m.http.ID != id {
		return tool.ErrNotFound
	}
	m.deleted = id
	return nil
}

func (m *mockIntegrationSvc) CreateDatabase(_ context.Context, req integration.DatabaseCreateRequest) (integration.DatabaseResponse, error) {
	m.dbReq = req
	if m.err != nil {
		return integration.DatabaseResponse{}, m.err
	}
	if m.database.ID == uuid.Nil {
		m.database = integration.DatabaseResponse{ID: uuid.New(), Name: req.Name, Type: req.Type, Host: req.Host, Query: req.Query}
	}
	return m.database, nil
}

func (m *mockIntegrationSvc) GetDatabase(_ context.Context, id uuid.UUID) (integration.DatabaseResponse, error) {
	if m.err != nil {
		return integration.DatabaseResponse{}, m.err
	}
	if m.database.ID == uuid.Nil || m.database.ID != id {
		return integration.DatabaseResponse{}, datasource.ErrNotFound
	}
	return m.database, nil
}

func (m *mockIntegrationSvc) UpdateDatabase(_ context.Context, id uuid.UUID, req integration.DatabaseCreateRequest) (integration.DatabaseResponse, error) {
	if m.err != nil {
		return integration.DatabaseResponse{}, m.err
	}
	if m.database.ID == uuid.Nil || m.database.ID != id {
		return integration.DatabaseResponse{}, datasource.ErrNotFound
	}
	m.dbReq = req
	m.database.Name = req.Name
	m.database.Host = req.Host
	m.database.Query = req.Query
	m.database.AllowWrite = req.AllowWrite
	return m.database, nil
}

func (m *mockIntegrationSvc) DeleteDatabase(_ context.Context, id uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	if m.database.ID == uuid.Nil || m.database.ID != id {
		return datasource.ErrNotFound
	}
	m.deleted = id
	return nil
}

func (m *mockIntegrationSvc) CreateMCP(_ context.Context, req mcp.CreateRequest) (mcp.McpServerConfigResponse, error) {
	m.mcpReq = req
	if m.err != nil {
		return mcp.McpServerConfigResponse{}, m.err
	}
	if m.mcp.ID == uuid.Nil {
		m.mcp = mcp.McpServerConfigResponse{ID: uuid.New(), Name: req.Name, TransportType: req.TransportType}
	}
	return m.mcp, nil
}

func (m *mockIntegrationSvc) GetMCP(_ context.Context, id uuid.UUID) (mcp.McpServerConfigResponse, error) {
	if m.err != nil {
		return mcp.McpServerConfigResponse{}, m.err
	}
	if m.mcp.ID == uuid.Nil || m.mcp.ID != id {
		return mcp.McpServerConfigResponse{}, mcp.ErrNotFound
	}
	return m.mcp, nil
}

func (m *mockIntegrationSvc) UpdateMCP(_ context.Context, id uuid.UUID, req mcp.UpdateRequest) (mcp.McpServerConfigResponse, error) {
	if m.err != nil {
		return mcp.McpServerConfigResponse{}, m.err
	}
	if m.mcp.ID == uuid.Nil || m.mcp.ID != id {
		return mcp.McpServerConfigResponse{}, mcp.ErrNotFound
	}
	if req.Name != nil {
		m.mcp.Name = *req.Name
	}
	return m.mcp, nil
}

func (m *mockIntegrationSvc) DeleteMCP(_ context.Context, id uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	if m.mcp.ID == uuid.Nil || m.mcp.ID != id {
		return mcp.ErrNotFound
	}
	m.deleted = id
	return nil
}

func setupIntegrationHandler() (*chi.Mux, *mockIntegrationSvc) {
	svc := &mockIntegrationSvc{
		page: pagination.NewPage([]integration.Response{{
			ID:         uuid.New(),
			Name:       "ERP API",
			Slug:       "erp-api",
			Type:       integration.IntegrationTypeHTTPAPI,
			SourceKind: integration.SourceKindTool,
			Enabled:    true,
		}}, 1, pagination.PageRequest{Page: 0, Size: 20}),
		http: integration.HTTPResponse{
			ID:     uuid.New(),
			Name:   "ERP API",
			Method: "POST",
			URL:    "https://api.example.com/customers",
		},
		database: integration.DatabaseResponse{
			ID:        uuid.New(),
			Name:      "Orders DB",
			Type:      datasource.DataSourceTypePostgreSQL,
			Host:      "pg.internal",
			Query:     "SELECT * FROM orders",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		mcp: mcp.McpServerConfigResponse{
			ID:            uuid.New(),
			Name:          "filesystem",
			TransportType: "stdio",
		},
	}
	h := integration.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestIntegrationHandler_List_Success(t *testing.T) {
	r, _ := setupIntegrationHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/integrations?page=0&size=20", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[integration.Response]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Content, 1)
	assert.Equal(t, integration.IntegrationTypeHTTPAPI, page.Content[0].Type)
}

func TestIntegrationHandler_List_ForwardsFilters(t *testing.T) {
	r, svc := setupIntegrationHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/integrations?type=MCP&enabled=false&origin=legacy", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.True(t, svc.called)
	require.NotNil(t, svc.filters.Type)
	require.NotNil(t, svc.filters.Enabled)
	require.NotNil(t, svc.filters.Origin)
	assert.Equal(t, integration.IntegrationTypeMCP, *svc.filters.Type)
	assert.False(t, *svc.filters.Enabled)
	assert.Equal(t, integration.IntegrationOriginLegacy, *svc.filters.Origin)
}

func TestIntegrationHandler_List_InvalidFilter(t *testing.T) {
	r, _ := setupIntegrationHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/integrations?type=UNKNOWN", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid type filter")
}

func TestIntegrationHandler_CreateHTTP_Success(t *testing.T) {
	r, svc := setupIntegrationHandler()
	body, _ := json.Marshal(integration.HTTPCreateRequest{Name: "ERP API", Method: "POST", URL: "https://api.example.com/customers"})
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/http", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "ERP API", svc.lastReq.Name)
}

func TestIntegrationHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		r, svc := setupIntegrationHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/integrations/http", bytes.NewBufferString(`{"name":"first","method":"GET","url":"https://api.example.com"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.lastReq.Name)
	})

	t.Run("database", func(t *testing.T) {
		r, svc := setupIntegrationHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/integrations/database", bytes.NewBufferString(`{"name":"first","type":"POSTGRESQL","host":"db.example.com"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.dbReq.Name)
	})

	t.Run("mcp", func(t *testing.T) {
		r, svc := setupIntegrationHandler()
		req := httptest.NewRequest(http.MethodPost, "/api/integrations/mcp", bytes.NewBufferString(`{"name":"first","transportType":"stdio"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.mcpReq.Name)
	})
}

func TestIntegrationHandler_GetHTTP_Success(t *testing.T) {
	r, svc := setupIntegrationHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/integrations/http/"+svc.http.ID.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ERP API")
}

func TestIntegrationHandler_UpdateHTTP_Success(t *testing.T) {
	r, svc := setupIntegrationHandler()
	body, _ := json.Marshal(integration.HTTPCreateRequest{Name: "ERP API Updated", Method: "PATCH", URL: "https://api.example.com/customers"})
	req := httptest.NewRequest(http.MethodPut, "/api/integrations/http/"+svc.http.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ERP API Updated", svc.http.Name)
}

func TestIntegrationHandler_DeleteHTTP_Success(t *testing.T) {
	r, svc := setupIntegrationHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/integrations/http/"+svc.http.ID.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, svc.http.ID, svc.deleted)
}

func TestIntegrationHandler_CreateDatabaseSuccess(t *testing.T) {
	r, svc := setupIntegrationHandler()
	body, _ := json.Marshal(integration.DatabaseCreateRequest{Name: "Orders DB", Type: datasource.DataSourceTypePostgreSQL, Host: "pg.internal", Query: "SELECT 1"})
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/database", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "Orders DB", svc.dbReq.Name)
}

func TestIntegrationHandler_GetDatabaseSuccess(t *testing.T) {
	r, svc := setupIntegrationHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/integrations/database/"+svc.database.ID.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Orders DB")
}

func TestIntegrationHandler_CreateMCPSuccess(t *testing.T) {
	r, svc := setupIntegrationHandler()
	body, _ := json.Marshal(mcp.CreateRequest{Name: "filesystem", TransportType: "stdio"})
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "filesystem", svc.mcpReq.Name)
}

func TestIntegrationHandler_GetMCPSuccess(t *testing.T) {
	r, svc := setupIntegrationHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/integrations/mcp/"+svc.mcp.ID.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "filesystem")
}

func TestIntegrationHandler_UpdateDatabaseSuccess(t *testing.T) {
	r, svc := setupIntegrationHandler()
	body, _ := json.Marshal(integration.DatabaseCreateRequest{
		Name: "Orders DB v2", Type: datasource.DataSourceTypePostgreSQL,
		Host: "pg.new", Port: 5432, Database: "orders", DBUser: "u", DBPassword: "p",
		Query: "SELECT 2", AllowWrite: true,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/integrations/database/"+svc.database.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Orders DB v2", svc.dbReq.Name)
	assert.True(t, svc.dbReq.AllowWrite)
}

func TestIntegrationHandler_UpdateDatabaseRejectsDuplicateJSONKeyBeforeService(t *testing.T) {
	r, svc := setupIntegrationHandler()
	body := []byte(`{"name":"first","name":"second","type":"POSTGRESQL","host":"pg.internal","query":"SELECT 1"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/integrations/database/"+svc.database.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "Orders DB", svc.database.Name)
}

func TestIntegrationHandler_PatchDatabase_MergesOmittedFields(t *testing.T) {
	r, svc := setupIntegrationHandler()
	svc.database.Description = "existing description"
	svc.database.Type = datasource.DataSourceTypePostgreSQL
	svc.database.Host = "pg.internal"
	svc.database.Port = 5432
	svc.database.Database = "orders"
	svc.database.DBUser = "orders_user"
	svc.database.Query = "SELECT 1"
	svc.database.AllowWrite = true

	body := []byte(`{"query":"SELECT 2"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/integrations/database/"+svc.database.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Orders DB", svc.dbReq.Name)
	assert.Equal(t, "existing description", svc.dbReq.Description)
	assert.Equal(t, datasource.DataSourceTypePostgreSQL, svc.dbReq.Type)
	assert.Equal(t, "pg.internal", svc.dbReq.Host)
	assert.Equal(t, 5432, svc.dbReq.Port)
	assert.Equal(t, "orders", svc.dbReq.Database)
	assert.Equal(t, "orders_user", svc.dbReq.DBUser)
	assert.Equal(t, "SELECT 2", svc.dbReq.Query)
	assert.True(t, svc.dbReq.AllowWrite)
}

func TestIntegrationHandler_UpdateDatabase_NotFound(t *testing.T) {
	r, _ := setupIntegrationHandler()
	body, _ := json.Marshal(integration.DatabaseCreateRequest{Name: "x", Type: datasource.DataSourceTypePostgreSQL})
	req := httptest.NewRequest(http.MethodPut, "/api/integrations/database/"+uuid.NewString(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIntegrationHandler_DeleteDatabaseSuccess(t *testing.T) {
	r, svc := setupIntegrationHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/integrations/database/"+svc.database.ID.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, svc.database.ID, svc.deleted)
}

func TestIntegrationHandler_DeleteDatabase_NotFound(t *testing.T) {
	r, _ := setupIntegrationHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/integrations/database/"+uuid.NewString(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIntegrationHandler_UpdateMCPSuccess(t *testing.T) {
	r, svc := setupIntegrationHandler()
	newName := "filesystem-renamed"
	body, _ := json.Marshal(mcp.UpdateRequest{Name: &newName})
	req := httptest.NewRequest(http.MethodPut, "/api/integrations/mcp/"+svc.mcp.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "filesystem-renamed", svc.mcp.Name)
}

func TestIntegrationHandler_UpdateMCP_NotFound(t *testing.T) {
	r, _ := setupIntegrationHandler()
	body, _ := json.Marshal(mcp.UpdateRequest{})
	req := httptest.NewRequest(http.MethodPut, "/api/integrations/mcp/"+uuid.NewString(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIntegrationHandler_DeleteMCPSuccess(t *testing.T) {
	r, svc := setupIntegrationHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/integrations/mcp/"+svc.mcp.ID.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, svc.mcp.ID, svc.deleted)
}

func TestIntegrationHandler_DeleteMCP_NotFound(t *testing.T) {
	r, _ := setupIntegrationHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/integrations/mcp/"+uuid.NewString(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIntegrationHandler_List_RejectsInvalidFilters(t *testing.T) {
	tests := []struct {
		name  string
		query string
		field string
	}{
		{name: "invalid enabled", query: "enabled=maybe", field: "enabled"},
		{name: "invalid origin", query: "origin=unknown", field: "origin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := setupIntegrationHandler()
			req := httptest.NewRequest(http.MethodGet, "/api/integrations?"+tt.query, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "invalid "+tt.field+" filter")
		})
	}
}

func TestIntegrationHandler_List_ServiceFailure(t *testing.T) {
	r, svc := setupIntegrationHandler()
	svc.err = errors.New("service unavailable")
	req := httptest.NewRequest(http.MethodGet, "/api/integrations", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal error")
}

func TestIntegrationHandler_HTTPRejectsInvalidRequests(t *testing.T) {
	validID := uuid.NewString()
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "get invalid id", method: http.MethodGet, path: "/api/integrations/http/invalid"},
		{name: "update invalid id", method: http.MethodPut, path: "/api/integrations/http/invalid", body: `{}`},
		{name: "delete invalid id", method: http.MethodDelete, path: "/api/integrations/http/invalid"},
		{name: "create invalid body", method: http.MethodPost, path: "/api/integrations/http", body: `{`},
		{name: "update invalid body", method: http.MethodPut, path: "/api/integrations/http/" + validID, body: `{`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := setupIntegrationHandler()
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestIntegrationHandler_HTTPNotFound(t *testing.T) {
	tests := []struct {
		name   string
		method string
		body   string
	}{
		{name: "get", method: http.MethodGet},
		{name: "update", method: http.MethodPatch, body: `{"name":"ERP","method":"GET","url":"https://example.com"}`},
		{name: "delete", method: http.MethodDelete},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := setupIntegrationHandler()
			req := httptest.NewRequest(tt.method, "/api/integrations/http/"+uuid.NewString(), bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestIntegrationHandler_HTTPServiceFailures(t *testing.T) {
	validID := uuid.NewString()
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		err    error
		status int
	}{
		{name: "create duplicate", method: http.MethodPost, path: "/api/integrations/http", body: `{}`, err: tool.ErrDuplicateName, status: http.StatusConflict},
		{name: "create generic", method: http.MethodPost, path: "/api/integrations/http", body: `{}`, err: errors.New("create failed"), status: http.StatusUnprocessableEntity},
		{name: "get generic", method: http.MethodGet, path: "/api/integrations/http/" + validID, err: errors.New("get failed"), status: http.StatusUnprocessableEntity},
		{name: "update generic", method: http.MethodPut, path: "/api/integrations/http/" + validID, body: `{}`, err: errors.New("update failed"), status: http.StatusUnprocessableEntity},
		{name: "delete generic", method: http.MethodDelete, path: "/api/integrations/http/" + validID, err: errors.New("delete failed"), status: http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, svc := setupIntegrationHandler()
			svc.err = tt.err
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.status, w.Code)
		})
	}
}

func TestIntegrationHandler_DatabaseRejectsInvalidRequests(t *testing.T) {
	validID := uuid.NewString()
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create invalid body", method: http.MethodPost, path: "/api/integrations/database", body: `{`},
		{name: "get invalid id", method: http.MethodGet, path: "/api/integrations/database/invalid"},
		{name: "update invalid id", method: http.MethodPut, path: "/api/integrations/database/invalid", body: `{}`},
		{name: "patch invalid id", method: http.MethodPatch, path: "/api/integrations/database/invalid", body: `{}`},
		{name: "delete invalid id", method: http.MethodDelete, path: "/api/integrations/database/invalid"},
		{name: "update invalid body", method: http.MethodPut, path: "/api/integrations/database/" + validID, body: `{`},
		{name: "patch invalid body", method: http.MethodPatch, path: "/api/integrations/database/" + validID, body: `{`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := setupIntegrationHandler()
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestIntegrationHandler_DatabaseNotFound(t *testing.T) {
	tests := []struct {
		name   string
		method string
		body   string
	}{
		{name: "get", method: http.MethodGet},
		{name: "patch", method: http.MethodPatch, body: `{"query":"SELECT 1"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := setupIntegrationHandler()
			req := httptest.NewRequest(tt.method, "/api/integrations/database/"+uuid.NewString(), bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestIntegrationHandler_DatabaseServiceFailures(t *testing.T) {
	validID := uuid.NewString()
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create", method: http.MethodPost, path: "/api/integrations/database", body: `{}`},
		{name: "get", method: http.MethodGet, path: "/api/integrations/database/" + validID},
		{name: "update", method: http.MethodPut, path: "/api/integrations/database/" + validID, body: `{}`},
		{name: "patch read current", method: http.MethodPatch, path: "/api/integrations/database/" + validID, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/integrations/database/" + validID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, svc := setupIntegrationHandler()
			svc.err = errors.New("database failed")
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		})
	}
}

func TestIntegrationHandler_MCPRejectsInvalidRequests(t *testing.T) {
	validID := uuid.NewString()
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create invalid body", method: http.MethodPost, path: "/api/integrations/mcp", body: `{`},
		{name: "get invalid id", method: http.MethodGet, path: "/api/integrations/mcp/invalid"},
		{name: "update invalid id", method: http.MethodPut, path: "/api/integrations/mcp/invalid", body: `{}`},
		{name: "delete invalid id", method: http.MethodDelete, path: "/api/integrations/mcp/invalid"},
		{name: "update invalid body", method: http.MethodPut, path: "/api/integrations/mcp/" + validID, body: `{`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := setupIntegrationHandler()
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestIntegrationHandler_MCPNotFound(t *testing.T) {
	r, _ := setupIntegrationHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/integrations/mcp/"+uuid.NewString(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIntegrationHandler_MCPServiceFailures(t *testing.T) {
	validID := uuid.NewString()
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create", method: http.MethodPost, path: "/api/integrations/mcp", body: `{}`},
		{name: "get", method: http.MethodGet, path: "/api/integrations/mcp/" + validID},
		{name: "update", method: http.MethodPatch, path: "/api/integrations/mcp/" + validID, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/integrations/mcp/" + validID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, svc := setupIntegrationHandler()
			svc.err = errors.New("mcp failed")
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		})
	}
}
