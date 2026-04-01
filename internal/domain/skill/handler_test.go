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
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockSkillSvc satisfies the private skillService interface in skill.Handler.
type mockSkillSvc struct {
	skills map[uuid.UUID]skill.Response
}

func newMockSkillSvc() *mockSkillSvc {
	return &mockSkillSvc{skills: make(map[uuid.UUID]skill.Response)}
}

func (m *mockSkillSvc) List(_ context.Context, req pagination.PageRequest) (pagination.Page[skill.Response], error) {
	items := make([]skill.Response, 0, len(m.skills))
	for _, s := range m.skills {
		items = append(items, s)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockSkillSvc) Create(_ context.Context, req skill.CreateRequest) (skill.Response, error) {
	id := uuid.New()
	resp := skill.Response{ID: id, Name: req.Name, Slug: req.Slug, Category: req.Category}
	m.skills[id] = resp
	return resp, nil
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
	svc := newMockSkillSvc()
	h := skill.NewHandler(svc)
	r := chi.NewRouter()
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
