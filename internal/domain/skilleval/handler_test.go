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
)

func buildTestHandler(t *testing.T) (*chi.Mux, *memRepo, *stubEvaluator) {
	t.Helper()
	repo := newMemRepo()
	ev := &stubEvaluator{output: skilleval.EvalOutput{Text: "test output"}}
	runner := skilleval.NewRunner(repo, ev, nil)
	svc := skilleval.NewService(repo).WithRunner(runner)
	h := skilleval.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, repo, ev
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
	assert.Equal(t, http.StatusInternalServerError, w.Code)
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
	repo.CreateCase(context.Background(), skilleval.EvalCase{
		SuiteID:        s.ID,
		InputText:      "6x7?",
		ExpectedOutput: "42",
		GraderType:     skilleval.GraderContains,
		ShouldTrigger:  true,
	})

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

// TestHandlerRunSuite_withEvaluatorError verifies the run is recorded as failed
// when the evaluator returns an error for every case.
func TestHandlerRunSuite_withEvaluatorError(t *testing.T) {
	repo := newMemRepo()
	ev := &stubEvaluator{err: errors.New("timeout")}
	runner := skilleval.NewRunner(repo, ev, nil)
	svc := skilleval.NewService(repo).WithRunner(runner)
	h := skilleval.NewHandler(svc)
	rt := chi.NewRouter()
	h.RegisterRoutes(rt)

	s, _ := repo.CreateSuite(context.Background(), skilleval.EvalSuite{
		SkillID: uuid.New(), Name: "fail-suite",
	})
	repo.CreateCase(context.Background(), skilleval.EvalCase{
		SuiteID: s.ID, InputText: "hello", ExpectedOutput: "world",
		GraderType: skilleval.GraderContains, ShouldTrigger: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/skill-evals/suites/"+s.ID.String()+"/run", nil)
	w := httptest.NewRecorder()
	rt.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp skilleval.RunResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, skilleval.RunStatusFailed, resp.Status)
}
