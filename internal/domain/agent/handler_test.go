package agent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockAgentSvc implements agent.Service for handler tests.
type mockAgentSvc struct {
	agents      map[uuid.UUID]agent.AgentResponse
	createCalls int
	updateCalls int
	updateErr   error // if set, Update returns this error
	publishErr  error // if set, Publish returns this error
}

func newMockSvc() *mockAgentSvc {
	return &mockAgentSvc{agents: make(map[uuid.UUID]agent.AgentResponse)}
}

func (m *mockAgentSvc) List(_ context.Context, _ agent.AgentStatus, _ string, req pagination.PageRequest) (pagination.Page[agent.AgentResponse], error) {
	items := make([]agent.AgentResponse, 0, len(m.agents))
	for _, a := range m.agents {
		items = append(items, a)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockAgentSvc) Get(_ context.Context, id uuid.UUID) (agent.AgentResponse, error) {
	a, ok := m.agents[id]
	if !ok {
		return agent.AgentResponse{}, agent.ErrNotFound
	}
	return a, nil
}

func (m *mockAgentSvc) Create(_ context.Context, req agent.CreateAgentRequest) (agent.AgentResponse, error) {
	m.createCalls++
	id := uuid.New()
	resp := agent.AgentResponse{
		ID:   id,
		Name: req.Name,
		Slug: req.Slug,
	}
	m.agents[id] = resp
	return resp, nil
}

func TestHandlerCreateRejectsConflictingEvalSampleRateAliasesWithoutCallingService(t *testing.T) {
	r, svc := setupAgent()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewBufferString(`{
"name":"conflicting eval sample rate",
"slug":"conflicting-eval-sample-rate",
"eval_config":{"scorers":["exact_match"],"sampleRate":0,"sample_rate":1}
}`))
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Zero(t, svc.createCalls, "agent service must not receive an ambiguous eval configuration")
}

func (m *mockAgentSvc) Update(_ context.Context, id uuid.UUID, req agent.UpdateAgentRequest) (agent.AgentResponse, error) {
	m.updateCalls++
	if m.updateErr != nil {
		return agent.AgentResponse{}, m.updateErr
	}
	a, ok := m.agents[id]
	if !ok {
		return agent.AgentResponse{}, agent.ErrNotFound
	}
	if req.Name != nil {
		a.Name = *req.Name
	}
	m.agents[id] = a
	return a, nil
}

func (m *mockAgentSvc) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.agents[id]; !ok {
		return agent.ErrNotFound
	}
	delete(m.agents, id)
	return nil
}

func (m *mockAgentSvc) BulkDelete(_ context.Context, ids []uuid.UUID) (int, error) {
	count := 0
	for _, id := range ids {
		if _, ok := m.agents[id]; ok {
			delete(m.agents, id)
			count++
		}
	}
	return count, nil
}

func (m *mockAgentSvc) Publish(_ context.Context, id uuid.UUID) (agent.AgentResponse, error) {
	if m.publishErr != nil {
		return agent.AgentResponse{}, m.publishErr
	}
	a, ok := m.agents[id]
	if !ok {
		return agent.AgentResponse{}, agent.ErrNotFound
	}
	a.Status = string(agent.StatusPublished)
	m.agents[id] = a
	return a, nil
}

func (m *mockAgentSvc) Archive(_ context.Context, id uuid.UUID) (agent.AgentResponse, error) {
	a, ok := m.agents[id]
	if !ok {
		return agent.AgentResponse{}, agent.ErrNotFound
	}
	a.Status = string(agent.StatusArchived)
	m.agents[id] = a
	return a, nil
}

func (m *mockAgentSvc) Restore(_ context.Context, id uuid.UUID) (agent.AgentResponse, error) {
	a, ok := m.agents[id]
	if !ok {
		return agent.AgentResponse{}, agent.ErrNotFound
	}
	a.Status = string(agent.StatusDraft)
	m.agents[id] = a
	return a, nil
}

func (m *mockAgentSvc) Clone(_ context.Context, id uuid.UUID, req agent.CloneAgentRequest) (agent.AgentResponse, error) {
	a, ok := m.agents[id]
	if !ok {
		return agent.AgentResponse{}, agent.ErrNotFound
	}
	newID := uuid.New()
	name := req.Name
	if name == "" {
		name = a.Name + " (copy)"
	}
	clone := agent.AgentResponse{ID: newID, Name: name}
	m.agents[newID] = clone
	return clone, nil
}

func (m *mockAgentSvc) GetWithReadiness(_ context.Context, id uuid.UUID) (agent.AgentResponse, error) {
	a, ok := m.agents[id]
	if !ok {
		return agent.AgentResponse{}, agent.ErrNotFound
	}
	return a, nil
}

func setupAgent() (*chi.Mux, *mockAgentSvc) {
	return setupAgentWithRoles("admin")
}

func setupAgentWithRoles(roles ...string) (*chi.Mux, *mockAgentSvc) {
	svc := newMockSvc()
	h := agent.NewHandler(svc)
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

func setupAgentWithService(svc agent.Service) *chi.Mux {
	h := agent.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)
	return r
}

func TestAgentHandler_CreateRejectsConflictingModelFallbackAliases(t *testing.T) {
	repo := newMockRepo()
	svc := agent.NewService(repo, &mockNoopBindingRepo{}, &mockNoopSkillRepo{})
	r := setupAgentWithService(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewBufferString(`{
		"name":"conflicting fallback aliases",
		"modelConfig":{
			"fallback_chain":[{"provider":"openrouter","model":"snake-primary"}],
			"fallbackChain":[{"provider":"ollama","model":"camel-primary"}]
		}
	}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "fallback_chain")
	assert.Empty(t, repo.data, "the HTTP boundary must not persist an ambiguous modelConfig")
}

type mockTemplateGetter struct {
	calls int
}

func (m *mockTemplateGetter) Get(_ context.Context, _ uuid.UUID) (agent.TemplateContent, error) {
	m.calls++
	return agent.TemplateContent{Content: "template content"}, nil
}

func setupAgentWithTemplate(templates agent.TemplateGetter) (*chi.Mux, *mockAgentSvc) {
	svc := newMockSvc()
	h := agent.NewHandler(svc).WithTemplateGetter(templates)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)
	return r, svc
}

type fakeReadAccess struct {
	allowed      bool
	calls        int
	subjectID    string
	resourceType string
	resourceID   string
	action       string
}

func (f *fakeReadAccess) CanAccess(_ context.Context, subjectID, resourceType, resourceID, action string) (bool, error) {
	f.calls++
	f.subjectID = subjectID
	f.resourceType = resourceType
	f.resourceID = resourceID
	f.action = action
	return f.allowed, nil
}

func setupAgentWithReadAccess(identity agent.RequestIdentity, checker *fakeReadAccess) (*chi.Mux, *mockAgentSvc) {
	svc := newMockSvc()
	h := agent.NewHandler(svc).WithReadAccess(checker, func(*http.Request) agent.RequestIdentity {
		return identity
	})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestAgentHandler_List_Success(t *testing.T) {
	r, svc := setupAgent()
	// seed one agent
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Agent A"}

	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[agent.AgentResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestAgentHandler_List_Empty(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupAgentWithRoles("user")
	id := uuid.NewString()
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create", method: http.MethodPost, path: "/api/agents", body: `{"name":"Agent","slug":"agent"}`},
		{name: "bulk delete", method: http.MethodDelete, path: "/api/agents", body: `{"ids":["` + id + `"]}`},
		{name: "put", method: http.MethodPut, path: "/api/agents/" + id, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/agents/" + id, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/agents/" + id},
		{name: "publish", method: http.MethodPost, path: "/api/agents/" + id + "/publish"},
		{name: "archive", method: http.MethodPost, path: "/api/agents/" + id + "/archive"},
		{name: "restore", method: http.MethodPost, path: "/api/agents/" + id + "/restore"},
		{name: "clone", method: http.MethodPost, path: "/api/agents/" + id + "/clone", body: `{}`},
		{name: "apply template", method: http.MethodPost, path: "/api/agents/" + id + "/apply-template", body: `{}`},
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

func TestAgentHandler_Create_Success(t *testing.T) {
	r, _ := setupAgent()
	body, _ := json.Marshal(agent.CreateAgentRequest{Name: "My Agent"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp agent.AgentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "My Agent", resp.Name)
}

func TestAgentHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, svc := setupAgent()
		req := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewBufferString(`{"name":"first"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.agents)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupAgent()
		id := uuid.New()
		svc.agents[id] = agent.AgentResponse{ID: id, Name: "original"}
		req := httptest.NewRequest(http.MethodPatch, "/api/agents/"+id.String(), bytes.NewBufferString(`{"name":"changed"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, "original", svc.agents[id].Name)
		assert.Zero(t, svc.updateCalls)
	})

	t.Run("bulk delete", func(t *testing.T) {
		r, svc := setupAgent()
		id := uuid.New()
		svc.agents[id] = agent.AgentResponse{ID: id, Name: "original"}
		req := httptest.NewRequest(http.MethodDelete, "/api/agents", bytes.NewBufferString(`{"ids":["`+id.String()+`"]}{"ids":[]}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Contains(t, svc.agents, id)
	})

	t.Run("clone", func(t *testing.T) {
		r, svc := setupAgent()
		id := uuid.New()
		svc.agents[id] = agent.AgentResponse{ID: id, Name: "original"}
		req := httptest.NewRequest(http.MethodPost, "/api/agents/"+id.String()+"/clone", bytes.NewBufferString(`{"name":"copy"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Len(t, svc.agents, 1)
	})

	t.Run("apply template", func(t *testing.T) {
		templates := &mockTemplateGetter{}
		r, svc := setupAgentWithTemplate(templates)
		id := uuid.New()
		svc.agents[id] = agent.AgentResponse{ID: id, Name: "original"}
		req := httptest.NewRequest(http.MethodPost, "/api/agents/"+id.String()+"/apply-template", bytes.NewBufferString(`{"template_id":"`+uuid.NewString()+`"}{"merge":true}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Zero(t, templates.calls)
		assert.Zero(t, svc.updateCalls)
	})
}

func TestAgentHandler_Get_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Test Agent"}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentHandler_Get_NotFound(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentHandler_Get_InvalidID(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentHandler_Get_ForbiddenWithoutReadGrant(t *testing.T) {
	checker := &fakeReadAccess{allowed: false}
	r, svc := setupAgentWithReadAccess(agent.RequestIdentity{SubjectID: "viewer@test.local", Roles: []string{"user"}}, checker)
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Private Agent"}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, 1, checker.calls)
	assert.Equal(t, "viewer@test.local", checker.subjectID)
	assert.Equal(t, "agents", checker.resourceType)
	assert.Equal(t, id.String(), checker.resourceID)
	assert.Equal(t, "read", checker.action)
}

func TestAgentHandler_Get_AllowedWithReadGrant(t *testing.T) {
	checker := &fakeReadAccess{allowed: true}
	r, svc := setupAgentWithReadAccess(agent.RequestIdentity{SubjectID: "viewer@test.local", Roles: []string{"user"}}, checker)
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Shared Agent"}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, checker.calls)
}

func TestAgentHandler_Get_AdminBypassesReadGrant(t *testing.T) {
	checker := &fakeReadAccess{allowed: false}
	r, svc := setupAgentWithReadAccess(agent.RequestIdentity{SubjectID: "admin@test.local", Roles: []string{"admin"}}, checker)
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Admin Agent"}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, checker.calls)
}

func TestAgentHandler_Delete_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "To Delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/agents/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAgentHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodDelete, "/api/agents/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentHandler_Publish_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Draft Agent", Status: string(agent.StatusDraft)}

	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+id.String()+"/publish", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp agent.AgentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, string(agent.StatusPublished), resp.Status)
}

func TestAgentHandler_Clone_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Original"}

	body, _ := json.Marshal(agent.CloneAgentRequest{Name: "Clone"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+id.String()+"/clone", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAgentHandler_Clone_NotFound(t *testing.T) {
	r, _ := setupAgent()
	body, _ := json.Marshal(agent.CloneAgentRequest{Name: "Clone"})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+uuid.New().String()+"/clone", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentHandler_Patch_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Original", Status: string(agent.StatusDraft)}

	newName := "Patched"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &newName})
	req := httptest.NewRequest(http.MethodPatch, "/api/agents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp agent.AgentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Patched", resp.Name)
}

func TestAgentHandler_Patch_NotFound(t *testing.T) {
	r, _ := setupAgent()
	name := "x"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &name})
	req := httptest.NewRequest(http.MethodPatch, "/api/agents/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentHandler_Update_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Original", Status: string(agent.StatusDraft)}

	newName := "Updated"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &newName})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp agent.AgentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Updated", resp.Name)
}

func TestAgentHandler_Update_NotFound(t *testing.T) {
	r, _ := setupAgent()
	name := "x"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &name})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentHandler_Archive_Success(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Published Agent", Status: string(agent.StatusPublished)}

	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+id.String()+"/archive", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp agent.AgentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, string(agent.StatusArchived), resp.Status)
}

func TestAgentHandler_Archive_NotFound(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+uuid.New().String()+"/archive", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- TR-01-TASK-32: mapeamento de erros de validação para 422 (P-C249-3) ---

func TestAgentHandler_Update_ValidationError_Returns422(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Agent"}
	svc.updateErr = fmt.Errorf("%w: invalid model config", agent.ErrInvalidModelConfig)

	name := "new name"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &name})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestAgentHandler_Update_SkillIDsError_Returns422(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Agent"}
	svc.updateErr = fmt.Errorf("%w: skill not found", agent.ErrInvalidSkillIDs)

	name := "new name"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &name})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestAgentHandler_Update_NestedModelConfig_Returns422(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Agent"}
	svc.updateErr = fmt.Errorf("%w: nested config", agent.ErrInvalidRequest)

	name := "new name"
	body, _ := json.Marshal(agent.UpdateAgentRequest{Name: &name})
	req := httptest.NewRequest(http.MethodPut, "/api/agents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// --- TR-01-TASK-37: publish validation (P-C278-1) ---

func TestAgentHandler_Publish_InvalidStatusTransition_Returns422(t *testing.T) {
	r, svc := setupAgent()
	id := uuid.New()
	svc.agents[id] = agent.AgentResponse{ID: id, Name: "Published Agent", Status: string(agent.StatusPublished)}
	svc.publishErr = fmt.Errorf("%w: agent is already published", agent.ErrInvalidStatusTransition)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+id.String()+"/publish", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestAgentHandler_Publish_InvalidID_Returns400(t *testing.T) {
	r, _ := setupAgent()
	req := httptest.NewRequest(http.MethodPost, "/api/agents/not-valid/publish", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Bulk delete tests (ACT-F3-19 / P-C341-1).

func TestAgentHandler_BulkDelete_Success(t *testing.T) {
	r, svc := setupAgent()
	id1 := uuid.New()
	id2 := uuid.New()
	svc.agents[id1] = agent.AgentResponse{ID: id1, Name: "a1"}
	svc.agents[id2] = agent.AgentResponse{ID: id2, Name: "a2"}

	body, _ := json.Marshal(map[string]interface{}{"ids": []uuid.UUID{id1, id2}})
	req := httptest.NewRequest(http.MethodDelete, "/api/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]int
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, 2, resp["deleted"])
	assert.NotContains(t, svc.agents, id1)
	assert.NotContains(t, svc.agents, id2)
}

func TestAgentHandler_BulkDelete_EmptyIDs_Returns400(t *testing.T) {
	r, _ := setupAgent()
	body, _ := json.Marshal(map[string]interface{}{"ids": []uuid.UUID{}})
	req := httptest.NewRequest(http.MethodDelete, "/api/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentHandler_BulkDelete_PartialSuccess_SkipsNotFound(t *testing.T) {
	r, svc := setupAgent()
	existingID := uuid.New()
	missingID := uuid.New()
	svc.agents[existingID] = agent.AgentResponse{ID: existingID, Name: "exists"}

	body, _ := json.Marshal(map[string]interface{}{"ids": []uuid.UUID{existingID, missingID}})
	req := httptest.NewRequest(http.MethodDelete, "/api/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]int
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, 1, resp["deleted"])
}
