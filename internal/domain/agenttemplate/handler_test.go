package agenttemplate_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agenttemplate"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

// buildRouter creates a test router with agent template routes mounted.
func buildRouter() (*chi.Mux, *memRepo, *stubAgentCreator) {
	return buildRouterWithRoles("admin")
}

func buildRouterWithRoles(roles ...string) (*chi.Mux, *memRepo, *stubAgentCreator) {
	repo := newMemRepo()
	creator := &stubAgentCreator{}
	svc := agenttemplate.NewService(repo).WithAgentCreator(creator)
	h := agenttemplate.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)
	return r, repo, creator
}

func TestAgentTemplateHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _, _ := buildRouterWithRoles("user")
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/agent-templates"},
		{name: "create", method: http.MethodPost, path: "/api/agent-templates", body: `{}`},
		{name: "get", method: http.MethodGet, path: "/api/agent-templates/template"},
		{name: "instantiate", method: http.MethodPost, path: "/api/agent-templates/template/instantiate", body: `{}`},
	} {
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

func TestHandler_List_ReturnsEmptyArray(t *testing.T) {
	r, _, _ := buildRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/agent-templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []agenttemplate.TemplateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Empty(t, got)
}

func TestHandler_List_ReturnsSeededTemplates(t *testing.T) {
	r, repo, _ := buildRouter()
	seedTemplate(repo, "rag-assistant", "rag")
	seedTemplate(repo, "sql-analyst", "data")

	req := httptest.NewRequest(http.MethodGet, "/api/agent-templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []agenttemplate.TemplateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 2)
}

func TestHandler_List_FiltersByCategory(t *testing.T) {
	r, repo, _ := buildRouter()
	seedTemplate(repo, "rag-assistant", "rag")
	seedTemplate(repo, "sql-analyst", "data")

	req := httptest.NewRequest(http.MethodGet, "/api/agent-templates?category=rag", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []agenttemplate.TemplateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 1)
	assert.Equal(t, "rag-assistant", got[0].Slug)
}

func TestHandler_GetBySlug_Found(t *testing.T) {
	r, repo, _ := buildRouter()
	seedTemplate(repo, "rag-assistant", "rag")

	req := httptest.NewRequest(http.MethodGet, "/api/agent-templates/rag-assistant", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got agenttemplate.TemplateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "rag-assistant", got.Slug)
}

func TestHandler_GetBySlug_NotFound(t *testing.T) {
	r, _, _ := buildRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/agent-templates/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_Create_Success(t *testing.T) {
	r, _, _ := buildRouter()
	body, _ := json.Marshal(agenttemplate.CreateTemplateRequest{
		Name:       "My Template",
		Category:   "custom",
		Definition: json.RawMessage(`{"systemPrompt":"hello","skills":[]}`),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/agent-templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got agenttemplate.TemplateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "My Template", got.Name)
	assert.Equal(t, "my-template", got.Slug)
}

func TestHandler_Create_MissingName(t *testing.T) {
	r, _, _ := buildRouter()
	body, _ := json.Marshal(agenttemplate.CreateTemplateRequest{})

	req := httptest.NewRequest(http.MethodPost, "/api/agent-templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestHandler_CreateAndInstantiateRejectTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, repo, _ := buildRouter()
		req := httptest.NewRequest(http.MethodPost, "/api/agent-templates", bytes.NewBufferString(`{"name":"Template","category":"custom"} {"name":"ignored"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, repo.data)
	})

	t.Run("instantiate", func(t *testing.T) {
		r, repo, creator := buildRouter()
		seedTemplate(repo, "rag-assistant", "rag")
		req := httptest.NewRequest(http.MethodPost, "/api/agent-templates/rag-assistant/instantiate", bytes.NewBufferString(`{"name":"Agent"} {"name":"ignored"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, creator.created)
	})
}

func TestHandler_Instantiate_Success(t *testing.T) {
	r, repo, creator := buildRouter()
	seedTemplate(repo, "rag-assistant", "rag")

	req := httptest.NewRequest(http.MethodPost, "/api/agent-templates/rag-assistant/instantiate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got agenttemplate.InstantiateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.NotEmpty(t, got.AgentID)
	require.Len(t, creator.created, 1)
	assert.Equal(t, "Test Template", creator.created[0].Name)
}

func TestHandler_Instantiate_TemplateNotFound(t *testing.T) {
	r, _, _ := buildRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/agent-templates/missing/instantiate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_Instantiate_WithNameOverride(t *testing.T) {
	r, repo, creator := buildRouter()
	seedTemplate(repo, "rag-assistant", "rag")

	body, _ := json.Marshal(agenttemplate.InstantiateRequest{Name: "Custom Agent Name"})
	req := httptest.NewRequest(http.MethodPost, "/api/agent-templates/rag-assistant/instantiate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	require.Len(t, creator.created, 1)
	assert.Equal(t, "Custom Agent Name", creator.created[0].Name)
}
