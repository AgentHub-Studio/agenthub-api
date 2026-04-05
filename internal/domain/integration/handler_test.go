package integration_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockIntegrationSvc struct {
	page    pagination.Page[integration.Response]
	filters integration.ListFilters
	called  bool
}

func (m *mockIntegrationSvc) List(_ context.Context, _ pagination.PageRequest, filters integration.ListFilters) (pagination.Page[integration.Response], error) {
	m.called = true
	m.filters = filters
	return m.page, nil
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
