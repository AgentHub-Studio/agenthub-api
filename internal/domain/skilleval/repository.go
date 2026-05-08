package skilleval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines persistence for the skill evaluation framework.
type Repository interface {
	// Suite operations
	CreateSuite(ctx context.Context, suite EvalSuite) (EvalSuite, error)
	SuiteExistsByName(ctx context.Context, skillID uuid.UUID, name string) (bool, error)
	ListSuites(ctx context.Context, skillID *uuid.UUID) ([]EvalSuite, error)
	GetSuiteByID(ctx context.Context, id uuid.UUID) (EvalSuite, error)
	DeleteSuite(ctx context.Context, id uuid.UUID) error

	// Case operations
	CreateCase(ctx context.Context, ec EvalCase) (EvalCase, error)
	ListCases(ctx context.Context, suiteID uuid.UUID) ([]EvalCase, error)
	GetCaseByID(ctx context.Context, id uuid.UUID) (EvalCase, error)
	DeleteCase(ctx context.Context, id uuid.UUID) error

	// Run operations
	CreateRun(ctx context.Context, run EvalRun) (EvalRun, error)
	UpdateRun(ctx context.Context, run EvalRun) (EvalRun, error)
	GetRunByID(ctx context.Context, id uuid.UUID) (EvalRun, error)
	ListRuns(ctx context.Context, suiteID uuid.UUID) ([]EvalRun, error)
	CreateCaseResult(ctx context.Context, r CaseResult) (CaseResult, error)
	ListCaseResults(ctx context.Context, runID uuid.UUID) ([]CaseResult, error)
}

type repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository backed by pgxpool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

// --- suites ---

func (r *repository) CreateSuite(ctx context.Context, s EvalSuite) (EvalSuite, error) {
	q := `INSERT INTO skill_eval_suite (skill_id, name, description)
	      VALUES ($1, $2, $3)
	      RETURNING id, skill_id, name, description, created_at, updated_at`
	row := r.pool.QueryRow(ctx, q, s.SkillID, s.Name, s.Description)
	return scanSuite(row)
}

func (r *repository) SuiteExistsByName(ctx context.Context, skillID uuid.UUID, name string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM skill_eval_suite WHERE skill_id = $1 AND name = $2)`,
		skillID, name,
	).Scan(&exists)
	return exists, err
}

func (r *repository) ListSuites(ctx context.Context, skillID *uuid.UUID) ([]EvalSuite, error) {
	var rows pgx.Rows
	var err error
	if skillID != nil {
		rows, err = r.pool.Query(ctx,
			`SELECT id, skill_id, name, description, created_at, updated_at
			 FROM skill_eval_suite WHERE skill_id = $1 ORDER BY name ASC`, *skillID)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, skill_id, name, description, created_at, updated_at
			 FROM skill_eval_suite ORDER BY name ASC`)
	}
	if err != nil {
		return nil, fmt.Errorf("skilleval: list suites: %w", err)
	}
	defer rows.Close()
	var out []EvalSuite
	for rows.Next() {
		s, err := scanSuite(rows)
		if err != nil {
			return nil, fmt.Errorf("skilleval: scan suite: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *repository) GetSuiteByID(ctx context.Context, id uuid.UUID) (EvalSuite, error) {
	q := `SELECT id, skill_id, name, description, created_at, updated_at
	      FROM skill_eval_suite WHERE id = $1`
	row := r.pool.QueryRow(ctx, q, id)
	s, err := scanSuite(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvalSuite{}, ErrSuiteNotFound
		}
		return EvalSuite{}, fmt.Errorf("skilleval: get suite: %w", err)
	}
	return s, nil
}

func (r *repository) DeleteSuite(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM skill_eval_suite WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("skilleval: delete suite: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSuiteNotFound
	}
	return nil
}

func scanSuite(row pgx.Row) (EvalSuite, error) {
	var s EvalSuite
	return s, row.Scan(&s.ID, &s.SkillID, &s.Name, &s.Description, &s.CreatedAt, &s.UpdatedAt)
}

// --- cases ---

func (r *repository) CreateCase(ctx context.Context, ec EvalCase) (EvalCase, error) {
	cfg := ec.GraderConfig
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}
	q := `INSERT INTO skill_eval_case
	        (suite_id, description, input_text, expected_tool, expected_output, grader_type, grader_config, should_trigger)
	      VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	      RETURNING id, suite_id, description, input_text, expected_tool, expected_output, grader_type, grader_config, should_trigger, created_at`
	row := r.pool.QueryRow(ctx, q,
		ec.SuiteID, ec.Description, ec.InputText, ec.ExpectedTool,
		ec.ExpectedOutput, string(ec.GraderType), []byte(cfg), ec.ShouldTrigger)
	return scanCase(row)
}

func (r *repository) ListCases(ctx context.Context, suiteID uuid.UUID) ([]EvalCase, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, suite_id, description, input_text, expected_tool, expected_output,
		        grader_type, grader_config, should_trigger, created_at
		 FROM skill_eval_case WHERE suite_id = $1 ORDER BY created_at ASC`, suiteID)
	if err != nil {
		return nil, fmt.Errorf("skilleval: list cases: %w", err)
	}
	defer rows.Close()
	var out []EvalCase
	for rows.Next() {
		ec, err := scanCase(rows)
		if err != nil {
			return nil, fmt.Errorf("skilleval: scan case: %w", err)
		}
		out = append(out, ec)
	}
	return out, rows.Err()
}

func (r *repository) GetCaseByID(ctx context.Context, id uuid.UUID) (EvalCase, error) {
	q := `SELECT id, suite_id, description, input_text, expected_tool, expected_output,
	             grader_type, grader_config, should_trigger, created_at
	      FROM skill_eval_case WHERE id = $1`
	row := r.pool.QueryRow(ctx, q, id)
	ec, err := scanCase(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvalCase{}, ErrCaseNotFound
		}
		return EvalCase{}, fmt.Errorf("skilleval: get case: %w", err)
	}
	return ec, nil
}

func (r *repository) DeleteCase(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM skill_eval_case WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("skilleval: delete case: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCaseNotFound
	}
	return nil
}

func scanCase(row pgx.Row) (EvalCase, error) {
	var ec EvalCase
	var cfgBytes []byte
	err := row.Scan(
		&ec.ID, &ec.SuiteID, &ec.Description, &ec.InputText,
		&ec.ExpectedTool, &ec.ExpectedOutput, &ec.GraderType,
		&cfgBytes, &ec.ShouldTrigger, &ec.CreatedAt,
	)
	if err != nil {
		return EvalCase{}, err
	}
	if len(cfgBytes) > 0 {
		ec.GraderConfig = json.RawMessage(cfgBytes)
	}
	return ec, nil
}

// --- runs ---

func (r *repository) CreateRun(ctx context.Context, run EvalRun) (EvalRun, error) {
	q := `INSERT INTO skill_eval_run (suite_id, status, total_cases, passed_cases, failed_cases)
	      VALUES ($1, $2, $3, $4, $5)
	      RETURNING id, suite_id, status, total_cases, passed_cases, failed_cases, started_at, finished_at`
	row := r.pool.QueryRow(ctx, q,
		run.SuiteID, string(run.Status), run.TotalCases, run.PassedCases, run.FailedCases)
	return scanRun(row)
}

func (r *repository) UpdateRun(ctx context.Context, run EvalRun) (EvalRun, error) {
	q := `UPDATE skill_eval_run
	      SET status = $1, passed_cases = $2, failed_cases = $3, finished_at = $4
	      WHERE id = $5
	      RETURNING id, suite_id, status, total_cases, passed_cases, failed_cases, started_at, finished_at`
	row := r.pool.QueryRow(ctx, q,
		string(run.Status), run.PassedCases, run.FailedCases, run.FinishedAt, run.ID)
	updated, err := scanRun(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvalRun{}, ErrRunNotFound
		}
		return EvalRun{}, fmt.Errorf("skilleval: update run: %w", err)
	}
	return updated, nil
}

func (r *repository) GetRunByID(ctx context.Context, id uuid.UUID) (EvalRun, error) {
	q := `SELECT id, suite_id, status, total_cases, passed_cases, failed_cases, started_at, finished_at
	      FROM skill_eval_run WHERE id = $1`
	row := r.pool.QueryRow(ctx, q, id)
	run, err := scanRun(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EvalRun{}, ErrRunNotFound
		}
		return EvalRun{}, fmt.Errorf("skilleval: get run: %w", err)
	}
	return run, nil
}

func (r *repository) ListRuns(ctx context.Context, suiteID uuid.UUID) ([]EvalRun, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, suite_id, status, total_cases, passed_cases, failed_cases, started_at, finished_at
		 FROM skill_eval_run WHERE suite_id = $1 ORDER BY started_at DESC`, suiteID)
	if err != nil {
		return nil, fmt.Errorf("skilleval: list runs: %w", err)
	}
	defer rows.Close()
	var out []EvalRun
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("skilleval: scan run: %w", err)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func scanRun(row pgx.Row) (EvalRun, error) {
	var run EvalRun
	return run, row.Scan(
		&run.ID, &run.SuiteID, &run.Status,
		&run.TotalCases, &run.PassedCases, &run.FailedCases,
		&run.StartedAt, &run.FinishedAt,
	)
}

// --- case results ---

func (r *repository) CreateCaseResult(ctx context.Context, res CaseResult) (CaseResult, error) {
	q := `INSERT INTO skill_eval_case_result (run_id, case_id, passed, actual_output, score, error_msg, duration_ms)
	      VALUES ($1, $2, $3, $4, $5, $6, $7)
	      RETURNING id, run_id, case_id, passed, actual_output, score, error_msg, duration_ms, created_at`
	row := r.pool.QueryRow(ctx, q,
		res.RunID, res.CaseID, res.Passed, res.ActualOutput, res.Score, res.ErrorMsg, res.DurationMs)
	return scanResult(row)
}

func (r *repository) ListCaseResults(ctx context.Context, runID uuid.UUID) ([]CaseResult, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, run_id, case_id, passed, actual_output, score, error_msg, duration_ms, created_at
		 FROM skill_eval_case_result WHERE run_id = $1 ORDER BY created_at ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("skilleval: list results: %w", err)
	}
	defer rows.Close()
	var out []CaseResult
	for rows.Next() {
		res, err := scanResult(rows)
		if err != nil {
			return nil, fmt.Errorf("skilleval: scan result: %w", err)
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

func scanResult(row pgx.Row) (CaseResult, error) {
	var res CaseResult
	return res, row.Scan(
		&res.ID, &res.RunID, &res.CaseID,
		&res.Passed, &res.ActualOutput, &res.Score, &res.ErrorMsg,
		&res.DurationMs, &res.CreatedAt,
	)
}

var _ Repository = (*repository)(nil)
