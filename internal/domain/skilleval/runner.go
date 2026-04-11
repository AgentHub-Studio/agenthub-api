package skilleval

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Evaluator produces output for a given skill and input text.
// The concrete implementation calls the agent runner; tests inject a stub.
type Evaluator interface {
	Evaluate(ctx context.Context, skillID uuid.UUID, input string) (EvalOutput, error)
}

// Runner executes an EvalSuite against a given Evaluator and persists results.
type Runner struct {
	repo      Repository
	evaluator Evaluator
	llm       LLMClient
}

// NewRunner creates a new Runner.
func NewRunner(repo Repository, evaluator Evaluator, llm LLMClient) *Runner {
	return &Runner{repo: repo, evaluator: evaluator, llm: llm}
}

// RunSuite executes all cases in the given suite, persists a run record with
// per-case results, and returns the completed EvalRun.
func (r *Runner) RunSuite(ctx context.Context, suiteID uuid.UUID) (EvalRun, []CaseResult, error) {
	suite, err := r.repo.GetSuiteByID(ctx, suiteID)
	if err != nil {
		return EvalRun{}, nil, fmt.Errorf("runner: get suite: %w", err)
	}

	cases, err := r.repo.ListCases(ctx, suiteID)
	if err != nil {
		return EvalRun{}, nil, fmt.Errorf("runner: list cases: %w", err)
	}

	run := EvalRun{
		SuiteID:    suiteID,
		Status:     RunStatusRunning,
		TotalCases: len(cases),
		StartedAt:  time.Now(),
	}
	run, err = r.repo.CreateRun(ctx, run)
	if err != nil {
		return EvalRun{}, nil, fmt.Errorf("runner: create run: %w", err)
	}

	results := make([]CaseResult, 0, len(cases))
	passed, failed := 0, 0

	for _, ec := range cases {
		result := r.executeCase(ctx, run.ID, suite.SkillID, ec)
		if result.Passed {
			passed++
		} else {
			failed++
		}
		persisted, err := r.repo.CreateCaseResult(ctx, result)
		if err != nil {
			// Non-fatal: record what we can.
			result.ErrorMsg = fmt.Sprintf("persist error: %v", err)
		} else {
			result = persisted
		}
		results = append(results, result)
	}

	// Finalise the run.
	now := time.Now()
	run.PassedCases = passed
	run.FailedCases = failed
	run.FinishedAt = &now
	run.Status = RunStatusPassed
	if failed > 0 {
		run.Status = RunStatusFailed
	}

	run, err = r.repo.UpdateRun(ctx, run)
	if err != nil {
		return run, results, fmt.Errorf("runner: update run: %w", err)
	}

	return run, results, nil
}

// executeCase runs a single eval case and returns an unpersisted CaseResult.
func (r *Runner) executeCase(ctx context.Context, runID, skillID uuid.UUID, ec EvalCase) CaseResult {
	start := time.Now()
	res := CaseResult{
		RunID:  runID,
		CaseID: ec.ID,
	}

	output, err := r.evaluator.Evaluate(ctx, skillID, ec.InputText)
	durationMs := int(time.Since(start).Milliseconds())
	res.DurationMs = durationMs

	if err != nil {
		res.ErrorMsg = err.Error()
		return res
	}

	res.ActualOutput = output.Text

	// Trigger check: if ShouldTrigger is set and ExpectedTool is given, verify the tool was called.
	if ec.ExpectedTool != "" {
		toolMatch := output.ToolCalled == ec.ExpectedTool
		if ec.ShouldTrigger && !toolMatch {
			res.Passed = false
			res.ErrorMsg = fmt.Sprintf("expected tool %q to be called, got %q", ec.ExpectedTool, output.ToolCalled)
			return res
		}
		if !ec.ShouldTrigger && toolMatch {
			res.Passed = false
			res.ErrorMsg = fmt.Sprintf("tool %q was called but should not have triggered", ec.ExpectedTool)
			return res
		}
	}

	// If no expected output, just the trigger check matters.
	if ec.ExpectedOutput == "" {
		res.Passed = true
		score := 1.0
		res.Score = &score
		return res
	}

	grader, err := NewGrader(ec.GraderType, ec.GraderConfig, r.llm)
	if err != nil {
		res.ErrorMsg = err.Error()
		return res
	}

	grade, err := grader.Grade(ctx, ec.ExpectedOutput, output.Text)
	if err != nil {
		res.ErrorMsg = err.Error()
		return res
	}

	res.Passed = grade.Passed
	res.Score = &grade.Score
	res.ErrorMsg = grade.Reason
	return res
}
