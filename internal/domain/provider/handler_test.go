package provider_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/provider"
)

func buildRouter() (*chi.Mux, *memRepo) {
	repo := newMemRepo()
	svc := provider.NewService(repo)
	h := provider.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, repo
}

func TestHandler_List_Empty(t *testing.T) {
	r, _ := buildRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []provider.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Empty(t, got)
}

func TestHandler_List_Seeded(t *testing.T) {
	r, repo := buildRouter()
	seed(repo, "github-mcp", provider.KindMCP)
	seed(repo, "postgresql", provider.KindDatabase)

	req := httptest.NewRequest(http.MethodGet, "/api/providers", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []provider.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 2)
}

func TestHandler_List_FilterByKind(t *testing.T) {
	r, repo := buildRouter()
	seed(repo, "github-mcp", provider.KindMCP)
	seed(repo, "postgresql", provider.KindDatabase)

	req := httptest.NewRequest(http.MethodGet, "/api/providers?kind=database", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []provider.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 1)
	assert.Equal(t, "postgresql", got[0].Slug)
}

func TestHandler_GetBySlug_Found(t *testing.T) {
	r, repo := buildRouter()
	seed(repo, "slack-api", provider.KindHTTP)

	req := httptest.NewRequest(http.MethodGet, "/api/providers/slack-api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got provider.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "slack-api", got.Slug)
	assert.Equal(t, provider.KindHTTP, got.Kind)
}

func TestHandler_GetBySlug_NotFound(t *testing.T) {
	r, _ := buildRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/providers/nope", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
