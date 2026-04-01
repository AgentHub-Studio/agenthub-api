package search_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/search"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockSearchSvc satisfies the private searchService interface in search.Handler.
type mockSearchSvc struct{}

func (m *mockSearchSvc) Search(_ context.Context, _ string, query string, _ int) (search.GlobalSearchResponse, error) {
	return search.GlobalSearchResponse{
		Query:          query,
		Agents:         []search.SearchResult{{ID: "a1", Name: "Agent One", Type: "agent"}},
		Skills:         []search.SearchResult{},
		Tools:          []search.SearchResult{},
		KnowledgeBases: []search.SearchResult{},
	}, nil
}

func setupSearch() *chi.Mux {
	svc := &mockSearchSvc{}
	h := search.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.NewContext(r.Context(), "test-tenant")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Mount("/api/search", h.Routes())
	return r
}

func TestSearchHandler_Search_Success(t *testing.T) {
	r := setupSearch()
	req := httptest.NewRequest(http.MethodGet, "/api/search/?q=agent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp search.GlobalSearchResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "agent", resp.Query)
	assert.Len(t, resp.Agents, 1)
}

func TestSearchHandler_Search_MissingQuery(t *testing.T) {
	r := setupSearch()
	req := httptest.NewRequest(http.MethodGet, "/api/search/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearchHandler_Search_WithLimit(t *testing.T) {
	r := setupSearch()
	req := httptest.NewRequest(http.MethodGet, "/api/search/?q=skill&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
