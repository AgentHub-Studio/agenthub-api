package llmpreset_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/llmpreset"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockLLMPresetSvc implements llmpreset.Service for handler tests.
type mockLLMPresetSvc struct {
	presets map[uuid.UUID]llmpreset.LLMPresetResponse
}

func newMockLLMPresetSvc() *mockLLMPresetSvc {
	return &mockLLMPresetSvc{presets: make(map[uuid.UUID]llmpreset.LLMPresetResponse)}
}

func (m *mockLLMPresetSvc) List(_ context.Context, _ string, req pagination.PageRequest) (pagination.Page[llmpreset.LLMPresetResponse], error) {
	items := make([]llmpreset.LLMPresetResponse, 0, len(m.presets))
	for _, p := range m.presets {
		items = append(items, p)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockLLMPresetSvc) ListByProvider(_ context.Context, _ string, provider string, req pagination.PageRequest) (pagination.Page[llmpreset.LLMPresetResponse], error) {
	items := make([]llmpreset.LLMPresetResponse, 0)
	for _, p := range m.presets {
		if p.Provider == provider {
			items = append(items, p)
		}
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockLLMPresetSvc) Get(_ context.Context, _ string, id uuid.UUID) (llmpreset.LLMPresetResponse, error) {
	p, ok := m.presets[id]
	if !ok {
		return llmpreset.LLMPresetResponse{}, llmpreset.ErrNotFound
	}
	return p, nil
}

func (m *mockLLMPresetSvc) Create(_ context.Context, _ string, req llmpreset.CreateLLMPresetRequest) (llmpreset.LLMPresetResponse, error) {
	id := uuid.New()
	resp := llmpreset.LLMPresetResponse{
		ID:            id,
		Name:          req.Name,
		Provider:      req.Provider,
		Model:         req.Model,
		MaxTokens:     req.MaxTokens,
		ContextWindow: req.ContextWindow,
	}
	m.presets[id] = resp
	return resp, nil
}

func (m *mockLLMPresetSvc) Update(_ context.Context, _ string, id uuid.UUID, req llmpreset.UpdateLLMPresetRequest) (llmpreset.LLMPresetResponse, error) {
	p, ok := m.presets[id]
	if !ok {
		return llmpreset.LLMPresetResponse{}, llmpreset.ErrNotFound
	}
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.ContextWindow != nil {
		p.ContextWindow = *req.ContextWindow
	}
	m.presets[id] = p
	return p, nil
}

func (m *mockLLMPresetSvc) Delete(_ context.Context, _ string, id uuid.UUID) error {
	if _, ok := m.presets[id]; !ok {
		return llmpreset.ErrNotFound
	}
	delete(m.presets, id)
	return nil
}

func (m *mockLLMPresetSvc) SetDefault(_ context.Context, _ string, id uuid.UUID) error {
	if _, ok := m.presets[id]; !ok {
		return llmpreset.ErrNotFound
	}
	return nil
}

func setupLLMPreset() (*chi.Mux, *mockLLMPresetSvc) {
	return setupLLMPresetWithRoles("admin")
}

func setupLLMPresetWithRoles(roles ...string) (*chi.Mux, *mockLLMPresetSvc) {
	svc := newMockLLMPresetSvc()
	h := llmpreset.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := tenant.NewContext(r.Context(), "test-tenant")
			ctx = middleware.ContextWithRoles(ctx, roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterProtectedRoutes(r)
	return r, svc
}

func TestLLMPresetHandler_RequiresAdminRole(t *testing.T) {
	r, _ := setupLLMPresetWithRoles("user")
	id := uuid.NewString()
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/llm-config-presets"},
		{name: "create", method: http.MethodPost, path: "/api/llm-config-presets", body: `{}`},
		{name: "list by provider", method: http.MethodGet, path: "/api/llm-config-presets/by-provider/openai"},
		{name: "get", method: http.MethodGet, path: "/api/llm-config-presets/" + id},
		{name: "put", method: http.MethodPut, path: "/api/llm-config-presets/" + id, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/llm-config-presets/" + id, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/llm-config-presets/" + id},
		{name: "set default", method: http.MethodPut, path: "/api/llm-config-presets/" + id + "/default"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "missing required role")
		})
	}
}

func TestLLMPresetHandler_List_Success(t *testing.T) {
	r, svc := setupLLMPreset()
	id := uuid.New()
	svc.presets[id] = llmpreset.LLMPresetResponse{ID: id, Name: "GPT-4"}

	req := httptest.NewRequest(http.MethodGet, "/api/llm-config-presets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[llmpreset.LLMPresetResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestLLMPresetHandler_Create_Success(t *testing.T) {
	r, _ := setupLLMPreset()
	body, _ := json.Marshal(llmpreset.CreateLLMPresetRequest{
		Name:     "My Preset",
		Provider: "openai",
		Model:    "gpt-4",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/llm-config-presets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp llmpreset.LLMPresetResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "My Preset", resp.Name)
}

func TestLLMPresetHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupLLMPreset()
	req := httptest.NewRequest(http.MethodPost, "/api/llm-config-presets", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMPresetHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, svc := setupLLMPreset()
		req := httptest.NewRequest(http.MethodPost, "/api/llm-config-presets", bytes.NewBufferString(`{"name":"first","provider":"openai","model":"gpt-4"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.presets)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupLLMPreset()
		id := uuid.New()
		svc.presets[id] = llmpreset.LLMPresetResponse{ID: id, Name: "original"}
		req := httptest.NewRequest(http.MethodPatch, "/api/llm-config-presets/"+id.String(), bytes.NewBufferString(`{"name":"changed"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, "original", svc.presets[id].Name)
	})
}

func TestLLMPresetHandler_Get_NotFound(t *testing.T) {
	r, _ := setupLLMPreset()
	req := httptest.NewRequest(http.MethodGet, "/api/llm-config-presets/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestLLMPresetHandler_Get_InvalidID(t *testing.T) {
	r, _ := setupLLMPreset()
	req := httptest.NewRequest(http.MethodGet, "/api/llm-config-presets/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLLMPresetHandler_Delete_Success(t *testing.T) {
	r, svc := setupLLMPreset()
	id := uuid.New()
	svc.presets[id] = llmpreset.LLMPresetResponse{ID: id, Name: "To Delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/llm-config-presets/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestLLMPresetHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupLLMPreset()
	req := httptest.NewRequest(http.MethodDelete, "/api/llm-config-presets/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestLLMPresetHandler_SetDefault_Success(t *testing.T) {
	r, svc := setupLLMPreset()
	id := uuid.New()
	svc.presets[id] = llmpreset.LLMPresetResponse{ID: id, Name: "Default"}

	req := httptest.NewRequest(http.MethodPut, "/api/llm-config-presets/"+id.String()+"/default", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestLLMPresetHandler_SetDefault_NotFound(t *testing.T) {
	r, _ := setupLLMPreset()
	req := httptest.NewRequest(http.MethodPut, "/api/llm-config-presets/"+uuid.New().String()+"/default", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
