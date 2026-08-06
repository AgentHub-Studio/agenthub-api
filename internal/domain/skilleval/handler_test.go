package skilleval_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skilleval"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

func buildTestHandler(t *testing.T) (*chi.Mux, *memRepo, *stubEvaluator) {
	return buildTestHandlerWithRoles(t, "admin")
}

func buildTestHandlerWithRoles(t *testing.T, roles ...string) (*chi.Mux, *memRepo, *stubEvaluator) {
	t.Helper()
	repo := newMemRepo()
	ev := &stubEvaluator{output: skilleval.EvalOutput{Text: "test output"}}
	runner := skilleval.NewRunner(repo, ev, nil)
	svc := skilleval.NewService(repo).WithRunner(runner)
	h := skilleval.NewHandler(svc)
	return newSkillEvalRouter(h, roles...), repo, ev
}

func newSkillEvalRouter(h *skilleval.Handler, roles ...string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)
	return r
}

func TestSkillEvalHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r := newSkillEvalRouter(skilleval.NewHandler(nil), "user")
	suiteID := uuid.NewString()
	caseID := uuid.NewString()
	runID := uuid.NewString()
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list suites", method: http.MethodGet, path: "/api/skill-evals/suites"},
		{name: "create suite", method: http.MethodPost, path: "/api/skill-evals/suites", body: `{}`},
		{name: "get suite", method: http.MethodGet, path: "/api/skill-evals/suites/" + suiteID},
		{name: "delete suite", method: http.MethodDelete, path: "/api/skill-evals/suites/" + suiteID},
		{name: "add case", method: http.MethodPost, path: "/api/skill-evals/suites/" + suiteID + "/cases", body: `{}`},
		{name: "delete case", method: http.MethodDelete, path: "/api/skill-evals/cases/" + caseID},
		{name: "run suite", method: http.MethodPost, path: "/api/skill-evals/suites/" + suiteID + "/run"},
		{name: "list runs", method: http.MethodGet, path: "/api/skill-evals/suites/" + suiteID + "/runs"},
		{name: "get run", method: http.MethodGet, path: "/api/skill-evals/runs/" + runID},
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

func seedSuite(t *testing.T, repo *memRepo) skilleval.EvalSuite {
	t.Helper()
	s, err := repo.CreateSuite(context.Background(), skilleval.EvalSuite{
		SkillID:     uuid.New(),
		Name:        "test-suite",
		Description: "seed",
	})
	require.NoError(t, err)
	return s
}

func TestHandlerListSuites_empty(t *testing.T) {
	r, _, _ := buildTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/skill-evals/suites", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var got []skilleval.SuiteResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Empty(t, got)
}

func TestHandlerCreateSuite_success(t *testing.T) {
	r, _, _ := buildTestHandler(t)
	body, _ := json.Marshal(skilleval.CreateSuiteRequest{
		SkillID: uuid.New(),
		Name:    "eval-suite",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/skill-evals/suites", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	var resp skilleval.SuiteResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "eval-suite", resp.Name)
}

func TestHandlerCreateSuite_missingName(t *testing.T) {
	r, _, _ := buildTestHandler(t)
	body, _ := json.Marshal(skilleval.CreateSuiteRequest{SkillID: uuid.New()})
	req := httptest.NewRequest(http.MethodPost, "/api/skill-evals/suites", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestHandlerCreateSuiteAndAddCaseRejectTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create suite", func(t *testing.T) {
		r, repo, _ := buildTestHandler(t)
		req := httptest.NewRequest(http.MethodPost, "/api/skill-evals/suites", bytes.NewBufferString(`{"skillId":"`+uuid.NewString()+`","name":"suite"} {"name":"ignored"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, repo.suites)
	})

	t.Run("add case", func(t *testing.T) {
		r, repo, _ := buildTestHandler(t)
		suite := seedSuite(t, repo)
		req := httptest.NewRequest(http.MethodPost, "/api/skill-evals/suites/"+suite.ID.String()+"/cases", bytes.NewBufferString(`{"inputText":"question","expectedOutput":"answer"} {"inputText":"ignored"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, repo.cases)
	})
}

func TestHandlerGetSuite_success(t *testing.T) {
	r, repo, _ := buildTestHandler(t)
	s := seedSuite(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/skill-evals/suites/"+s.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "suite")
	assert.Contains(t, body, "cases")
}

func TestHandlerGetSuite_notFound(t *testing.T) {
	r, _, _ := buildTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/skill-evals/suites/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerDeleteSuite_success(t *testing.T) {
	r, repo, _ := buildTestHandler(t)
	s := seedSuite(t, repo)
	req := httptest.NewRequest(http.MethodDelete, "/api/skill-evals/suites/"+s.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHandlerAddCase_success(t *testing.T) {
	r, repo, _ := buildTestHandler(t)
	s := seedSuite(t, repo)
	body, _ := json.Marshal(skilleval.CreateCaseRequest{
		InputText:      "what is 6x7?",
		ExpectedOutput: "42",
		GraderType:     skilleval.GraderContains,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/skill-evals/suites/"+s.ID.String()+"/cases", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	var resp skilleval.CaseResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "what is 6x7?", resp.InputText)
}

func TestHandlerDeleteCase_notFound(t *testing.T) {
	r, _, _ := buildTestHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/skill-evals/cases/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerRunSuite_success(t *testing.T) {
	r, repo, ev := buildTestHandler(t)
	ev.output = skilleval.EvalOutput{Text: "42 is the answer"}
	s := seedSuite(t, repo)
	// Add a case
	_, err := repo.CreateCase(context.Background(), skilleval.EvalCase{
		SuiteID:        s.ID,
		InputText:      "6x7?",
		ExpectedOutput: "42",
		GraderType:     skilleval.GraderContains,
		ShouldTrigger:  true,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/skill-evals/suites/"+s.ID.String()+"/run", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp skilleval.RunResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, skilleval.RunStatusPassed, resp.Status)
	assert.Equal(t, 1, resp.PassedCases)
}

func TestHandlerRunSuite_notFound(t *testing.T) {
	r, _, _ := buildTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/skill-evals/suites/"+uuid.New().String()+"/run", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerListRuns_success(t *testing.T) {
	r, repo, _ := buildTestHandler(t)
	s := seedSuite(t, repo)
	req := httptest.NewRequest(http.MethodGet, "/api/skill-evals/suites/"+s.ID.String()+"/runs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var runs []skilleval.RunResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &runs))
	assert.Empty(t, runs)
}

func TestHandlerGetRun_notFound(t *testing.T) {
	r, _, _ := buildTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/skill-evals/runs/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerGetRun_RedactsSensitiveResultErrorMessage(t *testing.T) {
	r, repo, _ := buildTestHandler(t)
	run := skilleval.EvalRun{ID: uuid.New(), SuiteID: uuid.New(), Status: skilleval.RunStatusFailed}
	repo.runs[run.ID] = run
	errorMessage := "Authorization: Bearer skill-eval-authorization-secret\npassword=skill-eval-password-secret"
	repo.results[run.ID] = []skilleval.CaseResult{{
		ID:       uuid.New(),
		RunID:    run.ID,
		CaseID:   uuid.New(),
		ErrorMsg: errorMessage,
	}}

	req := httptest.NewRequest(http.MethodGet, "/api/skill-evals/runs/"+run.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "skill-eval-authorization-secret")
	assert.NotContains(t, w.Body.String(), "skill-eval-password-secret")
	assert.Contains(t, w.Body.String(), "[REDACTED]")
	assert.Equal(t, errorMessage, repo.results[run.ID][0].ErrorMsg)
}

func TestResultResponseFrom_RedactsNestedSensitiveDiagnosticKeys(t *testing.T) {
	result := skilleval.CaseResult{
		ErrorMsg: `{"provider":{"api_key":"skill-eval-nested-api-key"},"details":[{"password":"skill-eval-nested-password"}]}`,
	}

	response := skilleval.ResultResponseFrom(result)

	assert.NotContains(t, response.ErrorMsg, "skill-eval-nested-api-key")
	assert.NotContains(t, response.ErrorMsg, "skill-eval-nested-password")
	assert.NotContains(t, response.ErrorMsg, "api_key")
	assert.NotContains(t, response.ErrorMsg, "password")
	assert.Contains(t, result.ErrorMsg, "skill-eval-nested-api-key")
}

// TestHandlerRunSuite_withEvaluatorError verifies the run is recorded as failed
// when the evaluator returns an error for every case.
func TestHandlerRunSuite_withEvaluatorError(t *testing.T) {
	repo := newMemRepo()
	ev := &stubEvaluator{err: errors.New("timeout")}
	runner := skilleval.NewRunner(repo, ev, nil)
	svc := skilleval.NewService(repo).WithRunner(runner)
	h := skilleval.NewHandler(svc)
	rt := newSkillEvalRouter(h, "admin")

	s, _ := repo.CreateSuite(context.Background(), skilleval.EvalSuite{
		SkillID: uuid.New(), Name: "fail-suite",
	})
	_, err := repo.CreateCase(context.Background(), skilleval.EvalCase{
		SuiteID: s.ID, InputText: "hello", ExpectedOutput: "world",
		GraderType: skilleval.GraderContains, ShouldTrigger: true,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/skill-evals/suites/"+s.ID.String()+"/run", nil)
	w := httptest.NewRecorder()
	rt.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp skilleval.RunResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, skilleval.RunStatusFailed, resp.Status)
}
