package skill_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockSkillSvc satisfies the private skillService interface in skill.Handler.
type mockSkillSvc struct {
	skills     map[uuid.UUID]skill.Response
	lastCreate *skill.CreateRequest
}

func newMockSkillSvc() *mockSkillSvc {
	return &mockSkillSvc{skills: make(map[uuid.UUID]skill.Response)}
}

func (m *mockSkillSvc) List(_ context.Context, _ *string, req pagination.PageRequest) (pagination.Page[skill.Response], error) {
	items := make([]skill.Response, 0, len(m.skills))
	for _, s := range m.skills {
		items = append(items, s)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockSkillSvc) Create(_ context.Context, req skill.CreateRequest) (skill.Response, error) {
	m.lastCreate = &req
	id := uuid.New()
	resp := skill.Response{ID: id, Name: req.Name, Slug: req.Slug, Category: req.Category}
	m.skills[id] = resp
	return resp, nil
}

type mockSkillExporter struct {
	skills map[uuid.UUID]skill.Skill
}

func (m *mockSkillExporter) GetByID(_ context.Context, id uuid.UUID) (skill.Skill, error) {
	sk, ok := m.skills[id]
	if !ok {
		return skill.Skill{}, skill.ErrNotFound
	}
	return sk, nil
}

func (m *mockSkillSvc) GetByID(_ context.Context, id uuid.UUID) (skill.Response, error) {
	s, ok := m.skills[id]
	if !ok {
		return skill.Response{}, skill.ErrNotFound
	}
	return s, nil
}

func (m *mockSkillSvc) Update(_ context.Context, id uuid.UUID, req skill.UpdateRequest) (skill.Response, error) {
	s, ok := m.skills[id]
	if !ok {
		return skill.Response{}, skill.ErrNotFound
	}
	s.Name = req.Name
	m.skills[id] = s
	return s, nil
}

func (m *mockSkillSvc) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.skills[id]; !ok {
		return skill.ErrNotFound
	}
	delete(m.skills, id)
	return nil
}

func setupSkill() (*chi.Mux, *mockSkillSvc) {
	return setupSkillWithRoles("admin")
}

func setupSkillWithRoles(roles ...string) (*chi.Mux, *mockSkillSvc) {
	svc := newMockSkillSvc()
	h := skill.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)
	return r, svc
}

func setupSkillWithExporter(exporter *mockSkillExporter) (*chi.Mux, *mockSkillSvc) {
	return setupSkillWithExporterAndRoles(exporter, "admin")
}

func setupSkillWithExporterAndRoles(exporter *mockSkillExporter, roles ...string) (*chi.Mux, *mockSkillSvc) {
	svc := newMockSkillSvc()
	h := skill.NewHandler(svc).WithRepository(exporter)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)
	return r, svc
}

func TestSkillHandler_List_Success(t *testing.T) {
	r, svc := setupSkill()
	id := uuid.New()
	svc.skills[id] = skill.Response{ID: id, Name: "Document Search"}

	req := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[skill.Response]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestSkillHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupSkillWithRoles("user")
	id := uuid.NewString()
	skillMD := "---\nname: Imported\n---\n\nInstructions"
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/skills"},
		{name: "create", method: http.MethodPost, path: "/api/skills", body: `{"name":"skill"}`},
		{name: "import", method: http.MethodPost, path: "/api/skills/import", body: skillMD},
		{name: "import skillmd alias", method: http.MethodPost, path: "/api/skills/import-skillmd", body: skillMD},
		{name: "export", method: http.MethodGet, path: "/api/skills/" + id + "/export"},
		{name: "get", method: http.MethodGet, path: "/api/skills/" + id},
		{name: "put", method: http.MethodPut, path: "/api/skills/" + id, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/skills/" + id, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/skills/" + id},
		{name: "skillmd export alias", method: http.MethodGet, path: "/api/skills/" + id + "/skillmd"},
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

func TestSkillHandler_Create_Success(t *testing.T) {
	r, _ := setupSkill()
	body, _ := json.Marshal(skill.CreateRequest{Name: "My Skill", Category: "search"})
	req := httptest.NewRequest(http.MethodPost, "/api/skills", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp skill.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "My Skill", resp.Name)
}

func TestSkillHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupSkill()
	req := httptest.NewRequest(http.MethodPost, "/api/skills", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSkillHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, svc := setupSkill()
		req := httptest.NewRequest(http.MethodPost, "/api/skills", bytes.NewBufferString(`{"name":"first","category":"search"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.skills)
		assert.Nil(t, svc.lastCreate)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupSkill()
		id := uuid.New()
		svc.skills[id] = skill.Response{ID: id, Name: "original"}
		req := httptest.NewRequest(http.MethodPut, "/api/skills/"+id.String(), bytes.NewBufferString(`{"name":"changed"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, "original", svc.skills[id].Name)
	})
}

func TestSkillHandler_Create_MissingName(t *testing.T) {
	r, _ := setupSkill()
	body, _ := json.Marshal(skill.CreateRequest{Name: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/skills", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestSkillHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupSkill()
	req := httptest.NewRequest(http.MethodGet, "/api/skills/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSkillHandler_Delete_Success(t *testing.T) {
	r, svc := setupSkill()
	id := uuid.New()
	svc.skills[id] = skill.Response{ID: id, Name: "To Delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/skills/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestSkillHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupSkill()
	req := httptest.NewRequest(http.MethodDelete, "/api/skills/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSkillHandler_Update_Success(t *testing.T) {
	r, svc := setupSkill()
	id := uuid.New()
	svc.skills[id] = skill.Response{ID: id, Name: "Original"}

	body, _ := json.Marshal(skill.UpdateRequest{Name: "Updated"})
	req := httptest.NewRequest(http.MethodPut, "/api/skills/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp skill.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Updated", resp.Name)
}

func TestSkillHandler_Update_NotFound(t *testing.T) {
	r, _ := setupSkill()
	body, _ := json.Marshal(skill.UpdateRequest{Name: "x"})
	req := httptest.NewRequest(http.MethodPut, "/api/skills/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSkillHandler_Patch_Success(t *testing.T) {
	r, svc := setupSkill()
	id := uuid.New()
	svc.skills[id] = skill.Response{ID: id, Name: "Original"}

	body, _ := json.Marshal(skill.UpdateRequest{Name: "Patched"})
	req := httptest.NewRequest(http.MethodPatch, "/api/skills/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp skill.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Patched", resp.Name)
}

func TestSkillHandler_Patch_NotFound(t *testing.T) {
	r, _ := setupSkill()
	body, _ := json.Marshal(skill.UpdateRequest{Name: "x"})
	req := httptest.NewRequest(http.MethodPatch, "/api/skills/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSkillHandler_ImportSkillMD_UsesPortableContractRoute(t *testing.T) {
	r, svc := setupSkill()
	body := `---
name: Portable Search
slug: portable-search
description: Search external documentation
category: research
when_to_use: When a user needs external sources
context_mode: fork
should_defer: true
---
Search the provided documentation and cite the relevant passages.`

	req := httptest.NewRequest(http.MethodPost, "/api/skills/import", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "text/markdown")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, svc.lastCreate)
	assert.Equal(t, "Portable Search", svc.lastCreate.Name)
	assert.Equal(t, "portable-search", svc.lastCreate.Slug)
	assert.Equal(t, "Search the provided documentation and cite the relevant passages.", svc.lastCreate.Instructions)
	assert.True(t, svc.lastCreate.ShouldDefer)
}

func TestSkillHandler_ExportSkillMD_UsesPortableContractRoute(t *testing.T) {
	id := uuid.New()
	r, _ := setupSkillWithExporter(&mockSkillExporter{skills: map[uuid.UUID]skill.Skill{
		id: {
			ID:           id,
			Name:         "Portable Search",
			Slug:         "portable-search",
			Description:  "Search external documentation",
			Instructions: "Search the provided documentation and cite the relevant passages.",
			Category:     "research",
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/api/skills/"+id.String()+"/export", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/markdown; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "portable-search.skill.md")
	assert.Contains(t, w.Body.String(), "name: Portable Search")
	assert.Contains(t, w.Body.String(), "Search the provided documentation")
}
