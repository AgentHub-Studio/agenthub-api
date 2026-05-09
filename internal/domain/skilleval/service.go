package skilleval

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Service provides business logic for the Skill Evaluation Framework.
type Service struct {
	repo   Repository
	runner *Runner
}

// NewService creates a new Service.
// runner may be nil when eval execution features are not needed (e.g. read-only mode).
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// WithRunner wires the eval runner for executing suites.
func (s *Service) WithRunner(r *Runner) *Service {
	s.runner = r
	return s
}

// CreateSuite creates a new eval suite.
func (s *Service) CreateSuite(ctx context.Context, req CreateSuiteRequest) (SuiteResponse, error) {
	if req.Name == "" {
		return SuiteResponse{}, fmt.Errorf("%w: suite name is required", ErrValidation)
	}
	// Bug 132: name varchar(255) — gate length antes do INSERT.
	if len(req.Name) > 255 {
		return SuiteResponse{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Name))
	}
	if req.SkillID == uuid.Nil {
		return SuiteResponse{}, fmt.Errorf("%w: skillId is required", ErrValidation)
	}
	// Bug 178: cap description em 32KB (cross-cutting com bug 159).
	if len(req.Description) > 32000 {
		return SuiteResponse{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(req.Description))
	}
	exists, err := s.repo.SuiteExistsByName(ctx, req.SkillID, req.Name)
	if err != nil {
		return SuiteResponse{}, fmt.Errorf("skilleval: check duplicate: %w", err)
	}
	if exists {
		return SuiteResponse{}, ErrDuplicateName
	}
	suite, err := s.repo.CreateSuite(ctx, EvalSuite{
		SkillID:     req.SkillID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		return SuiteResponse{}, fmt.Errorf("skilleval: create suite: %w", err)
	}
	return SuiteResponseFrom(suite), nil
}

// ListSuites returns all suites, optionally filtered by skillID.
func (s *Service) ListSuites(ctx context.Context, skillID *uuid.UUID) ([]SuiteResponse, error) {
	suites, err := s.repo.ListSuites(ctx, skillID)
	if err != nil {
		return nil, fmt.Errorf("skilleval: list suites: %w", err)
	}
	out := make([]SuiteResponse, len(suites))
	for i, suite := range suites {
		out[i] = SuiteResponseFrom(suite)
	}
	return out, nil
}

// GetSuite returns a suite with its cases.
func (s *Service) GetSuite(ctx context.Context, id uuid.UUID) (SuiteResponse, []CaseResponse, error) {
	suite, err := s.repo.GetSuiteByID(ctx, id)
	if err != nil {
		return SuiteResponse{}, nil, err
	}
	cases, err := s.repo.ListCases(ctx, id)
	if err != nil {
		return SuiteResponse{}, nil, fmt.Errorf("skilleval: list cases: %w", err)
	}
	caseResps := make([]CaseResponse, len(cases))
	for i, c := range cases {
		caseResps[i] = CaseResponseFrom(c)
	}
	return SuiteResponseFrom(suite), caseResps, nil
}

// DeleteSuite removes a suite and all its cases.
func (s *Service) DeleteSuite(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteSuite(ctx, id)
}

// AddCase adds a test case to a suite.
func (s *Service) AddCase(ctx context.Context, suiteID uuid.UUID, req CreateCaseRequest) (CaseResponse, error) {
	if req.InputText == "" {
		return CaseResponse{}, fmt.Errorf("%w: inputText is required", ErrValidation)
	}
	graderType := req.GraderType
	if graderType == "" {
		graderType = GraderContains
	}

	shouldTrigger := true
	if req.ShouldTrigger != nil {
		shouldTrigger = *req.ShouldTrigger
	}

	ec, err := s.repo.CreateCase(ctx, EvalCase{
		SuiteID:        suiteID,
		Description:    req.Description,
		InputText:      req.InputText,
		ExpectedTool:   req.ExpectedTool,
		ExpectedOutput: req.ExpectedOutput,
		GraderType:     graderType,
		GraderConfig:   req.GraderConfig,
		ShouldTrigger:  shouldTrigger,
	})
	if err != nil {
		return CaseResponse{}, fmt.Errorf("skilleval: add case: %w", err)
	}
	return CaseResponseFrom(ec), nil
}

// DeleteCase removes a test case from a suite.
func (s *Service) DeleteCase(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteCase(ctx, id)
}

// RunSuite executes all cases in the suite synchronously and returns the run result.
func (s *Service) RunSuite(ctx context.Context, suiteID uuid.UUID) (RunResponse, error) {
	if s.runner == nil {
		return RunResponse{}, fmt.Errorf("skilleval: runner not configured")
	}
	run, results, err := s.runner.RunSuite(ctx, suiteID)
	if err != nil {
		return RunResponse{}, fmt.Errorf("skilleval: run suite: %w", err)
	}
	return RunResponseFrom(run, results), nil
}

// GetRun returns a run with its case results.
func (s *Service) GetRun(ctx context.Context, runID uuid.UUID) (RunResponse, error) {
	run, err := s.repo.GetRunByID(ctx, runID)
	if err != nil {
		return RunResponse{}, err
	}
	results, err := s.repo.ListCaseResults(ctx, runID)
	if err != nil {
		return RunResponse{}, fmt.Errorf("skilleval: list results: %w", err)
	}
	return RunResponseFrom(run, results), nil
}

// ListRuns returns all runs for a suite.
func (s *Service) ListRuns(ctx context.Context, suiteID uuid.UUID) ([]RunResponse, error) {
	runs, err := s.repo.ListRuns(ctx, suiteID)
	if err != nil {
		return nil, fmt.Errorf("skilleval: list runs: %w", err)
	}
	out := make([]RunResponse, len(runs))
	for i, run := range runs {
		out[i] = RunResponseFrom(run, nil)
	}
	return out, nil
}
