package skilleval_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skilleval"
)

// --- in-memory repository stub ---

type memRepo struct {
	suites  map[uuid.UUID]skilleval.EvalSuite
	cases   map[uuid.UUID]skilleval.EvalCase
	runs    map[uuid.UUID]skilleval.EvalRun
	results map[uuid.UUID][]skilleval.CaseResult
}

func newMemRepo() *memRepo {
	return &memRepo{
		suites:  make(map[uuid.UUID]skilleval.EvalSuite),
		cases:   make(map[uuid.UUID]skilleval.EvalCase),
		runs:    make(map[uuid.UUID]skilleval.EvalRun),
		results: make(map[uuid.UUID][]skilleval.CaseResult),
	}
}

func (r *memRepo) CreateSuite(_ context.Context, s skilleval.EvalSuite) (skilleval.EvalSuite, error) {
	s.ID = uuid.New()
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	r.suites[s.ID] = s
	return s, nil
}

func (r *memRepo) ListSuites(_ context.Context, skillID *uuid.UUID) ([]skilleval.EvalSuite, error) {
	out := []skilleval.EvalSuite{}
	for _, s := range r.suites {
		if skillID == nil || s.SkillID == *skillID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *memRepo) GetSuiteByID(_ context.Context, id uuid.UUID) (skilleval.EvalSuite, error) {
	s, ok := r.suites[id]
	if !ok {
		return skilleval.EvalSuite{}, skilleval.ErrSuiteNotFound
	}
	return s, nil
}

func (r *memRepo) DeleteSuite(_ context.Context, id uuid.UUID) error {
	if _, ok := r.suites[id]; !ok {
		return skilleval.ErrSuiteNotFound
	}
	delete(r.suites, id)
	return nil
}

func (r *memRepo) CreateCase(_ context.Context, ec skilleval.EvalCase) (skilleval.EvalCase, error) {
	ec.ID = uuid.New()
	ec.CreatedAt = time.Now()
	r.cases[ec.ID] = ec
	return ec, nil
}

func (r *memRepo) ListCases(_ context.Context, suiteID uuid.UUID) ([]skilleval.EvalCase, error) {
	var out []skilleval.EvalCase
	for _, c := range r.cases {
		if c.SuiteID == suiteID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *memRepo) GetCaseByID(_ context.Context, id uuid.UUID) (skilleval.EvalCase, error) {
	c, ok := r.cases[id]
	if !ok {
		return skilleval.EvalCase{}, skilleval.ErrCaseNotFound
	}
	return c, nil
}

func (r *memRepo) DeleteCase(_ context.Context, id uuid.UUID) error {
	if _, ok := r.cases[id]; !ok {
		return skilleval.ErrCaseNotFound
	}
	delete(r.cases, id)
	return nil
}

func (r *memRepo) CreateRun(_ context.Context, run skilleval.EvalRun) (skilleval.EvalRun, error) {
	run.ID = uuid.New()
	run.StartedAt = time.Now()
	r.runs[run.ID] = run
	return run, nil
}

func (r *memRepo) UpdateRun(_ context.Context, run skilleval.EvalRun) (skilleval.EvalRun, error) {
	if _, ok := r.runs[run.ID]; !ok {
		return skilleval.EvalRun{}, skilleval.ErrRunNotFound
	}
	r.runs[run.ID] = run
	return run, nil
}

func (r *memRepo) GetRunByID(_ context.Context, id uuid.UUID) (skilleval.EvalRun, error) {
	run, ok := r.runs[id]
	if !ok {
		return skilleval.EvalRun{}, skilleval.ErrRunNotFound
	}
	return run, nil
}

func (r *memRepo) ListRuns(_ context.Context, suiteID uuid.UUID) ([]skilleval.EvalRun, error) {
	var out []skilleval.EvalRun
	for _, run := range r.runs {
		if run.SuiteID == suiteID {
			out = append(out, run)
		}
	}
	return out, nil
}

func (r *memRepo) CreateCaseResult(_ context.Context, res skilleval.CaseResult) (skilleval.CaseResult, error) {
	res.ID = uuid.New()
	res.CreatedAt = time.Now()
	r.results[res.RunID] = append(r.results[res.RunID], res)
	return res, nil
}

func (r *memRepo) ListCaseResults(_ context.Context, runID uuid.UUID) ([]skilleval.CaseResult, error) {
	return r.results[runID], nil
}

// --- stub evaluator ---

type stubEvaluator struct {
	output skilleval.EvalOutput
	err    error
}

func (e *stubEvaluator) Evaluate(_ context.Context, _ uuid.UUID, _ string) (skilleval.EvalOutput, error) {
	return e.output, e.err
}

// --- tests ---

func buildService() (*skilleval.Service, *memRepo) {
	repo := newMemRepo()
	svc := skilleval.NewService(repo)
	return svc, repo
}

func TestServiceCreateSuite_success(t *testing.T) {
	svc, _ := buildService()
	resp, err := svc.CreateSuite(context.Background(), skilleval.CreateSuiteRequest{
		SkillID:     uuid.New(),
		Name:        "my-suite",
		Description: "test",
	})
	require.NoError(t, err)
	assert.Equal(t, "my-suite", resp.Name)
	assert.NotEqual(t, uuid.Nil, resp.ID)
}

func TestServiceCreateSuite_missingName(t *testing.T) {
	svc, _ := buildService()
	_, err := svc.CreateSuite(context.Background(), skilleval.CreateSuiteRequest{
		SkillID: uuid.New(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestServiceCreateSuite_missingSkillID(t *testing.T) {
	svc, _ := buildService()
	_, err := svc.CreateSuite(context.Background(), skilleval.CreateSuiteRequest{
		Name: "suite",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skillId is required")
}

func TestServiceListSuites_filterBySkill(t *testing.T) {
	svc, _ := buildService()
	skillID := uuid.New()
	svc.CreateSuite(context.Background(), skilleval.CreateSuiteRequest{SkillID: skillID, Name: "s1"})
	svc.CreateSuite(context.Background(), skilleval.CreateSuiteRequest{SkillID: uuid.New(), Name: "s2"})

	suites, err := svc.ListSuites(context.Background(), &skillID)
	require.NoError(t, err)
	assert.Len(t, suites, 1)
	assert.Equal(t, "s1", suites[0].Name)
}

func TestServiceAddCase_defaultGrader(t *testing.T) {
	svc, _ := buildService()
	suite, _ := svc.CreateSuite(context.Background(), skilleval.CreateSuiteRequest{
		SkillID: uuid.New(), Name: "s",
	})

	c, err := svc.AddCase(context.Background(), suite.ID, skilleval.CreateCaseRequest{
		InputText:      "hello",
		ExpectedOutput: "world",
	})
	require.NoError(t, err)
	assert.Equal(t, skilleval.GraderContains, c.GraderType)
	assert.True(t, c.ShouldTrigger)
}

func TestServiceAddCase_missingInputText(t *testing.T) {
	svc, _ := buildService()
	suite, _ := svc.CreateSuite(context.Background(), skilleval.CreateSuiteRequest{
		SkillID: uuid.New(), Name: "s",
	})
	_, err := svc.AddCase(context.Background(), suite.ID, skilleval.CreateCaseRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inputText is required")
}

func TestServiceDeleteCase_notFound(t *testing.T) {
	svc, _ := buildService()
	err := svc.DeleteCase(context.Background(), uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, skilleval.ErrCaseNotFound))
}

func TestServiceRunSuite_noRunner(t *testing.T) {
	svc, _ := buildService()
	suite, _ := svc.CreateSuite(context.Background(), skilleval.CreateSuiteRequest{
		SkillID: uuid.New(), Name: "s",
	})
	_, err := svc.RunSuite(context.Background(), suite.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runner not configured")
}

func TestServiceRunSuite_allPass(t *testing.T) {
	repo := newMemRepo()
	evaluator := &stubEvaluator{output: skilleval.EvalOutput{Text: "the answer is 42"}}
	runner := skilleval.NewRunner(repo, evaluator, nil)
	svc := skilleval.NewService(repo).WithRunner(runner)

	skillID := uuid.New()
	suite, _ := svc.CreateSuite(context.Background(), skilleval.CreateSuiteRequest{
		SkillID: skillID, Name: "math-suite",
	})
	svc.AddCase(context.Background(), suite.ID, skilleval.CreateCaseRequest{
		InputText:      "what is 6x7?",
		ExpectedOutput: "42",
		GraderType:     skilleval.GraderContains,
	})
	svc.AddCase(context.Background(), suite.ID, skilleval.CreateCaseRequest{
		InputText:      "what is the answer?",
		ExpectedOutput: "answer",
		GraderType:     skilleval.GraderContains,
	})

	run, err := svc.RunSuite(context.Background(), suite.ID)
	require.NoError(t, err)
	assert.Equal(t, skilleval.RunStatusPassed, run.Status)
	assert.Equal(t, 2, run.TotalCases)
	assert.Equal(t, 2, run.PassedCases)
	assert.Equal(t, 0, run.FailedCases)
	assert.Len(t, run.Results, 2)
}

func TestServiceRunSuite_someFail(t *testing.T) {
	repo := newMemRepo()
	evaluator := &stubEvaluator{output: skilleval.EvalOutput{Text: "no useful response"}}
	runner := skilleval.NewRunner(repo, evaluator, nil)
	svc := skilleval.NewService(repo).WithRunner(runner)

	skillID := uuid.New()
	suite, _ := svc.CreateSuite(context.Background(), skilleval.CreateSuiteRequest{
		SkillID: skillID, Name: "s",
	})
	svc.AddCase(context.Background(), suite.ID, skilleval.CreateCaseRequest{
		InputText:      "what is 6x7?",
		ExpectedOutput: "42",
		GraderType:     skilleval.GraderContains,
	})

	run, err := svc.RunSuite(context.Background(), suite.ID)
	require.NoError(t, err)
	assert.Equal(t, skilleval.RunStatusFailed, run.Status)
	assert.Equal(t, 1, run.FailedCases)
}

func TestServiceRunSuite_evaluatorError(t *testing.T) {
	repo := newMemRepo()
	evaluator := &stubEvaluator{err: errors.New("LLM unavailable")}
	runner := skilleval.NewRunner(repo, evaluator, nil)
	svc := skilleval.NewService(repo).WithRunner(runner)

	skillID := uuid.New()
	suite, _ := svc.CreateSuite(context.Background(), skilleval.CreateSuiteRequest{
		SkillID: skillID, Name: "s",
	})
	svc.AddCase(context.Background(), suite.ID, skilleval.CreateCaseRequest{
		InputText: "hello", ExpectedOutput: "world",
	})

	run, err := svc.RunSuite(context.Background(), suite.ID)
	require.NoError(t, err)
	assert.Equal(t, skilleval.RunStatusFailed, run.Status)
	assert.Equal(t, 1, run.FailedCases)
	assert.Contains(t, run.Results[0].ErrorMsg, "LLM unavailable")
}

func TestServiceRunSuite_toolTriggerCheck(t *testing.T) {
	repo := newMemRepo()
	evaluator := &stubEvaluator{output: skilleval.EvalOutput{
		Text:       "I searched for that",
		ToolCalled: "document_search",
	}}
	runner := skilleval.NewRunner(repo, evaluator, nil)
	svc := skilleval.NewService(repo).WithRunner(runner)

	skillID := uuid.New()
	suite, _ := svc.CreateSuite(context.Background(), skilleval.CreateSuiteRequest{
		SkillID: skillID, Name: "trigger-suite",
	})
	shouldTrigger := true
	svc.AddCase(context.Background(), suite.ID, skilleval.CreateCaseRequest{
		InputText:     "find docs about X",
		ExpectedTool:  "document_search",
		ShouldTrigger: &shouldTrigger,
	})

	run, err := svc.RunSuite(context.Background(), suite.ID)
	require.NoError(t, err)
	assert.Equal(t, skilleval.RunStatusPassed, run.Status)
	assert.True(t, run.Results[0].Passed)
}

func TestServiceGetRun_notFound(t *testing.T) {
	svc, _ := buildService()
	_, err := svc.GetRun(context.Background(), uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, skilleval.ErrRunNotFound))
}
