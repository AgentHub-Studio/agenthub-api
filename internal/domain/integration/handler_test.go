package integration_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockIntegrationSvc struct {
	page    pagination.Page[integration.Response]
	filters integration.ListFilters
	called  bool
	http    integration.HTTPResponse
	lastReq integration.HTTPCreateRequest
	deleted uuid.UUID
}

func (m *mockIntegrationSvc) List(_ context.Context, _ pagination.PageRequest, filters integration.ListFilters) (pagination.Page[integration.Response], error) {
	m.called = true
	m.filters = filters
	return m.page, nil
}

func (m *mockIntegrationSvc) CreateHTTP(_ context.Context, req integration.HTTPCreateRequest) (integration.HTTPResponse, error) {
	m.lastReq = req
	if m.http.ID == uuid.Nil {
		m.http = integration.HTTPResponse{ID: uuid.New(), Name: req.Name, URL: req.URL, Method: req.Method}
	}
	return m.http, nil
}

func (m *mockIntegrationSvc) GetHTTP(_ context.Context, id uuid.UUID) (integration.HTTPResponse, error) {
	if m.http.ID == uuid.Nil || m.http.ID != id {
		return integration.HTTPResponse{}, tool.ErrNotFound
	}
	return m.http, nil
}

func (m *mockIntegrationSvc) UpdateHTTP(_ context.Context, id uuid.UUID, req integration.HTTPCreateRequest) (integration.HTTPResponse, error) {
	m.lastReq = req
	if m.http.ID == uuid.Nil || m.http.ID != id {
		return integration.HTTPResponse{}, tool.ErrNotFound
	}
	m.http.Name = req.Name
	m.http.URL = req.URL
	m.http.Method = req.Method
	return m.http, nil
}

func (m *mockIntegrationSvc) DeleteHTTP(_ context.Context, id uuid.UUID) error {
	if m.http.ID == uuid.Nil || m.http.ID != id {
		return tool.ErrNotFound
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
