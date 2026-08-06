package abtest_test

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

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/abtest"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

func setupABTestHandler(t *testing.T) (*chi.Mux, abtest.Service, *mockRepo) {
	t.Helper()
	svc, repo := setup()
	h := abtest.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), "admin")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)
	return r, svc, repo
}

func TestABTestHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	h := abtest.NewHandler(nil)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), "user")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)

	agentID := uuid.NewString()
	abTestID := uuid.NewString()
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/agents/" + agentID + "/ab-tests/"},
		{name: "create", method: http.MethodPost, path: "/api/agents/" + agentID + "/ab-tests/", body: `{}`},
		{name: "get", method: http.MethodGet, path: "/api/agents/" + agentID + "/ab-tests/" + abTestID},
		{name: "put", method: http.MethodPut, path: "/api/agents/" + agentID + "/ab-tests/" + abTestID, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/agents/" + agentID + "/ab-tests/" + abTestID, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/agents/" + agentID + "/ab-tests/" + abTestID},
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

func TestABTestHandler_CanonicalCollectionPathWorksWithoutTrailingSlash(t *testing.T) {
	r, _, _ := setupABTestHandler(t)
	agentID := uuid.New()
	body := `{"name":"Canonical path","variantVersionId":"` + uuid.NewString() + `","trafficPercent":25}`
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/ab-tests", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	req = httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID.String()+"/ab-tests", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var page struct {
		Content []abtest.ABTestResponse `json:"content"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Content, 1)
	assert.Equal(t, agentID, page.Content[0].AgentID)
}

func TestABTestHandler_CreateRejectsSecondActiveTestForAgent(t *testing.T) {
	r, _, repo := setupABTestHandler(t)
	agentID := uuid.New()
	path := "/api/agents/" + agentID.String() + "/ab-tests"

	for index, body := range []string{
		`{"name":"First","variantVersionId":"` + uuid.NewString() + `","trafficPercent":25}`,
		`{"name":"Second","variantVersionId":"` + uuid.NewString() + `","trafficPercent":25}`,
	} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if index == 0 {
			assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		} else {
			assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
		}
	}
	assert.Equal(t, 1, repo.createCalls)
}

func TestABTestHandler_UpdateRejectsActivatingSecondTestForAgent(t *testing.T) {
	r, svc, repo := setupABTestHandler(t)
	agentID := uuid.New()
	first, err := svc.Create(t.Context(), abtest.CreateABTestRequest{
		AgentID:          agentID,
		Name:             "Paused first",
		VariantVersionID: uuid.New(),
		TrafficPercent:   25,
	})
	require.NoError(t, err)
	paused := "PAUSED"
	_, err = svc.Update(t.Context(), first.ID, abtest.UpdateABTestRequest{Status: &paused})
	require.NoError(t, err)
	_, err = svc.Create(t.Context(), abtest.CreateABTestRequest{
		AgentID:          agentID,
		Name:             "Active second",
		VariantVersionID: uuid.New(),
		TrafficPercent:   25,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch, "/api/agents/"+agentID.String()+"/ab-tests/"+first.ID.String(), bytes.NewBufferString(`{"status":"ACTIVE"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Equal(t, 1, repo.updateCalls)
	assert.Equal(t, abtest.TestStatusPaused, repo.tests[first.ID].Status)
}

func TestABTestHandler_CreateRejectsTrailingJSONWithoutPersisting(t *testing.T) {
	r, _, repo := setupABTestHandler(t)
	agentID := uuid.New()
	body := `{"name":"first","variantVersionId":"` + uuid.NewString() + `","trafficPercent":10}{"name":"ignored"}`
	req := httptest.NewRequest(http.MethodPost, "/api/agents/"+agentID.String()+"/ab-tests", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, repo.createCalls)
	assert.Empty(t, repo.tests)
}

func TestABTestHandler_UpdateRejectsTrailingJSONWithoutPersisting(t *testing.T) {
	r, svc, repo := setupABTestHandler(t)
	agentID := uuid.New()
	created, err := svc.Create(t.Context(), abtest.CreateABTestRequest{
		AgentID:          agentID,
		Name:             "Original",
		VariantVersionID: uuid.New(),
		TrafficPercent:   10,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch, "/api/agents/"+agentID.String()+"/ab-tests/"+created.ID.String(), bytes.NewBufferString(`{"trafficPercent":20}{"status":"CONCLUDED"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, repo.updateCalls)
	assert.Equal(t, 10, repo.tests[created.ID].TrafficPercent)
}

func TestABTestHandler_NestedResourceMustBelongToPathAgent(t *testing.T) {
	r, svc, repo := setupABTestHandler(t)
	ownerID := uuid.New()
	otherAgentID := uuid.New()
	created, err := svc.Create(t.Context(), abtest.CreateABTestRequest{
		AgentID:          ownerID,
		Name:             "Owner only",
		VariantVersionID: uuid.New(),
		TrafficPercent:   10,
	})
	require.NoError(t, err)
	path := "/api/agents/" + otherAgentID.String() + "/ab-tests/" + created.ID.String()

	for _, tc := range []struct {
		name   string
		method string
		body   string
	}{
		{name: "get", method: http.MethodGet},
		{name: "update", method: http.MethodPatch, body: `{"trafficPercent":50}`},
		{name: "delete", method: http.MethodDelete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
		})
	}
	assert.Zero(t, repo.updateCalls)
	assert.Zero(t, repo.deleteCalls)
	assert.Contains(t, repo.tests, created.ID)
}

func TestABTestHandler_UpdateRejectsStatusOutsideLifecycleWithoutPersisting(t *testing.T) {
	r, svc, repo := setupABTestHandler(t)
	agentID := uuid.New()
	created, err := svc.Create(t.Context(), abtest.CreateABTestRequest{
		AgentID:          agentID,
		Name:             "Status boundary",
		VariantVersionID: uuid.New(),
		TrafficPercent:   10,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch, "/api/agents/"+agentID.String()+"/ab-tests/"+created.ID.String(), bytes.NewBufferString(`{"status":"DRAFT"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	assert.Zero(t, repo.updateCalls)
	assert.Equal(t, abtest.TestStatusActive, repo.tests[created.ID].Status)
}
