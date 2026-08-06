package agent_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

func TestHookHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	h := agent.NewHookHandler(nil)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), "user")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)

	agentID := uuid.NewString()
	hookID := uuid.NewString()
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/agents/" + agentID + "/hooks"},
		{name: "create", method: http.MethodPost, path: "/api/agents/" + agentID + "/hooks", body: `{}`},
		{name: "get", method: http.MethodGet, path: "/api/agents/" + agentID + "/hooks/" + hookID},
		{name: "update", method: http.MethodPut, path: "/api/agents/" + agentID + "/hooks/" + hookID, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/agents/" + agentID + "/hooks/" + hookID},
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

func TestHookHandlerRejectsTrailingJSONBeforeDatabaseAcquire(t *testing.T) {
	h := agent.NewHookHandler(nil)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create", method: http.MethodPost, path: "/api/agents/" + uuid.NewString() + "/hooks", body: `{"event":"before_run","hookType":"HTTP"}{"event":"ignored"}`},
		{name: "update", method: http.MethodPut, path: "/api/agents/" + uuid.NewString() + "/hooks/" + uuid.NewString(), body: `{"matcher":"first"}{"matcher":"ignored"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		})
	}
}

func TestHookHandler_CreateRejectsConflictingPromptConfigAliasesBeforeDatabaseAcquire(t *testing.T) {
	h := agent.NewHookHandler(nil)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/agents/"+uuid.NewString()+"/hooks",
		bytes.NewBufferString(`{"event":"before_run","hookType":"prompt","config":{"template":"Prefer this","inject":"Use this instead"}}`),
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "template")
	assert.Contains(t, w.Body.String(), "inject")
}
