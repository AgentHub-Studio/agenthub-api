// Package skilleval provides the Skill Evaluation Framework: test suites, graders,
// and a runner that executes test cases against skill outputs to measure quality.
// Grader types: exact_match, contains, regex, llm_judge.
package skilleval

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrSuiteNotFound is returned when an eval suite cannot be found.
var ErrSuiteNotFound = errors.New("skilleval: suite not found")

// ErrCaseNotFound is returned when an eval case cannot be found.
var ErrCaseNotFound = errors.New("skilleval: case not found")

// ErrRunNotFound is returned when an eval run cannot be found.
var ErrRunNotFound = errors.New("skilleval: run not found")

// GraderType identifies the comparison strategy for an eval case.
type GraderType string

const (
	// GraderExactMatch passes when actual_output == expected_output (case-sensitive).
	GraderExactMatch GraderType = "exact_match"
	// GraderContains passes when actual_output contains expected_output.
	GraderContains GraderType = "contains"
	// GraderRegex passes when actual_output matches the expected_output as a regex pattern.
	GraderRegex GraderType = "regex"
	// GraderLLMJudge delegates the pass/fail decision to an LLM using a prompt template.
	GraderLLMJudge GraderType = "llm_judge"
	// GraderSemanticSimilarity passes when the cosine similarity between expected and actual
	// embeddings meets or exceeds the configured threshold (default 0.8).
	GraderSemanticSimilarity GraderType = "semantic_similarity"
)

// RunStatus represents the lifecycle state of an eval run.
type RunStatus string

const (
	RunStatusRunning RunStatus = "running"
	RunStatusPassed  RunStatus = "passed"
	RunStatusFailed  RunStatus = "failed"
	RunStatusError   RunStatus = "error"
)

// EvalSuite is a named collection of test cases targeting a single skill.
type EvalSuite struct {
	ID          uuid.UUID `db:"id"`
	SkillID     uuid.UUID `db:"skill_id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// EvalCase is a single test case within a suite.
type EvalCase struct {
	ID             uuid.UUID       `db:"id"`
	SuiteID        uuid.UUID       `db:"suite_id"`
	Description    string          `db:"description"`
	InputText      string          `db:"input_text"`
	ExpectedTool   string          `db:"expected_tool"`
	ExpectedOutput string          `db:"expected_output"`
	GraderType     GraderType      `db:"grader_type"`
	GraderConfig   json.RawMessage `db:"grader_config"`
	ShouldTrigger  bool            `db:"should_trigger"`
	CreatedAt      time.Time       `db:"created_at"`
}

// EvalRun records one execution of a suite.
type EvalRun struct {
	ID          uuid.UUID  `db:"id"`
	SuiteID     uuid.UUID  `db:"suite_id"`
	Status      RunStatus  `db:"status"`
	TotalCases  int        `db:"total_cases"`
	PassedCases int        `db:"passed_cases"`
	FailedCases int        `db:"failed_cases"`
	StartedAt   time.Time  `db:"started_at"`
	FinishedAt  *time.Time `db:"finished_at"`
}

// CaseResult holds the outcome of a single case within a run.
type CaseResult struct {
	ID           uuid.UUID `db:"id"`
	RunID        uuid.UUID `db:"run_id"`
	CaseID       uuid.UUID `db:"case_id"`
	Passed       bool      `db:"passed"`
	ActualOutput string    `db:"actual_output"`
	Score        *float64  `db:"score"`
	ErrorMsg     string    `db:"error_msg"`
	DurationMs   int       `db:"duration_ms"`
	CreatedAt    time.Time `db:"created_at"`
}

// EvalOutput is the result of invoking the evaluator for a single input.
type EvalOutput struct {
	// Text is the text response produced for the input.
	Text string
	// ToolCalled is the tool that was invoked, if any.
	// Empty when no tool was selected by the LLM.
	ToolCalled string
}
