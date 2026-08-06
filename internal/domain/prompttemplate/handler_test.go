package prompttemplate_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/prompttemplate"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

// stubService wraps a real service backed by a stub repo for unit tests.
// For handler tests we need a lightweight in-memory approach.
// We test the handler layer by verifying HTTP status codes and response format.

func setupRouter(handler *prompttemplate.Handler) *chi.Mux {
	return setupRouterWithRoles(handler, "admin")
}

func setupRouterWithRoles(handler *prompttemplate.Handler, roles ...string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	handler.RegisterRoutes(r)
	return r
}

// Note: list/CRUD tests that need a real service are skipped when no DB is available.
// We only test input validation and routing in handler_test.go.

func TestHandler_Get_InvalidID(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/prompt-templates/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouterWithRoles(handler, "user")
	id := "00000000-0000-0000-0000-000000000001"
	agentID := "00000000-0000-0000-0000-000000000002"
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/prompt-templates"},
		{name: "create", method: http.MethodPost, path: "/api/prompt-templates", body: `{"name":"template"}`},
		{name: "get", method: http.MethodGet, path: "/api/prompt-templates/" + id},
		{name: "put", method: http.MethodPut, path: "/api/prompt-templates/" + id, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/prompt-templates/" + id, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/prompt-templates/" + id},
		{name: "list by agent", method: http.MethodGet, path: "/api/agents/" + agentID + "/prompt-templates"},
		{name: "create for agent", method: http.MethodPost, path: "/api/agents/" + agentID + "/prompt-templates", body: `{"name":"template"}`},
	}

	for _, tc := range cases {
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

func TestHandler_Create_InvalidBody(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/prompt-templates", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerRejectsTrailingJSONBeforeRepository(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create global", method: http.MethodPost, path: "/api/prompt-templates", body: `{"name":"first","slug":"first","content":"content"}{"name":"ignored"}`},
		{name: "create for agent", method: http.MethodPost, path: "/api/agents/" + uuid.NewString() + "/prompt-templates", body: `{"name":"first","slug":"first","content":"content"}{"name":"ignored"}`},
		{name: "update", method: http.MethodPatch, path: "/api/prompt-templates/" + uuid.NewString(), body: `{"name":"changed"}{"name":"ignored"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

func TestHandler_Create_MissingRequiredFields(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	body := prompttemplate.CreateRequest{
		Name: "", // empty
		Slug: "test",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/prompt-templates", bytes.NewReader(b))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestHandler_Update_InvalidID(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPut, "/api/prompt-templates/bad-id", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Delete_InvalidID(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/prompt-templates/bad-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_ListByAgent_InvalidAgentID(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/not-uuid/prompt-templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_CreateForAgent_InvalidBody(t *testing.T) {
	handler := prompttemplate.NewHandler(prompttemplate.NewService(nil))
	r := setupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/00000000-0000-0000-0000-000000000001/prompt-templates", bytes.NewReader([]byte("bad")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResponseFrom(t *testing.T) {
	tmpl := prompttemplate.PromptTemplate{
		Name:     "Test",
		Slug:     "test",
		Category: prompttemplate.CategoryRAG,
		Content:  "Hello",
	}
	resp := prompttemplate.ResponseFrom(tmpl)
	require.Equal(t, "Test", resp.Name)
	require.Equal(t, "rag", resp.Category)
	require.Equal(t, "Hello", resp.Content)
}
