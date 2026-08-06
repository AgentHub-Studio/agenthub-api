package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a PostgreSQL workflow repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

const workflowColumns = `id, slug, name, description, start_step_id, steps, created_at, updated_at`
const executionColumns = `id, workflow_id, workflow_slug, state, input, current_step_id, suspended_at,
	resumed_at, resume_data, output, created_at, updated_at`

func (r *postgresRepository) Create(ctx context.Context, wf Workflow) (Workflow, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return Workflow{}, err
	}
	defer release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return Workflow{}, fmt.Errorf("workflow: begin create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	stepsJSON, err := json.Marshal(wf.Steps)
	if err != nil {
		return Workflow{}, fmt.Errorf("workflow: marshal steps: %w", err)
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO workflow (id, slug, name, description, start_step_id, steps)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+workflowColumns,
		wf.ID, wf.Slug, wf.Name, wf.Description, wf.Start, stepsJSON,
	)
	created, err := scanWorkflow(row)
	if err != nil {
		if database.IsPgError(err, database.PgErrUniqueViolation) {
			return Workflow{}, ErrConflict
		}
		return Workflow{}, fmt.Errorf("workflow: create: %w", err)
	}

	for i, step := range wf.Steps {
		configJSON, _ := json.Marshal(stepConfig(step))
		casesJSON, _ := json.Marshal(step.Cases)
		if len(casesJSON) == 0 {
			casesJSON = []byte(`{}`)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO workflow_step (workflow_id, step_id, type, config, next_step_id, cases, position)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			created.ID, step.ID, string(step.Kind), configJSON, nullableString(step.Next), casesJSON, i,
		); err != nil {
			return Workflow{}, fmt.Errorf("workflow: create step: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Workflow{}, fmt.Errorf("workflow: commit create: %w", err)
	}
	return created, nil
}

func (r *postgresRepository) GetBySlug(ctx context.Context, slug string) (Workflow, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return Workflow{}, err
	}
	defer release()

	row := conn.QueryRow(ctx, `SELECT `+workflowColumns+` FROM workflow WHERE slug = $1`, slug)
	wf, err := scanWorkflow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workflow{}, ErrNotFound
	}
	if err != nil {
		return Workflow{}, fmt.Errorf("workflow: get by slug: %w", err)
	}
	return wf, nil
}

func (r *postgresRepository) CreateExecution(ctx context.Context, ex Execution) (Execution, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return Execution{}, err
	}
	defer release()

	inputJSON, _ := json.Marshal(defaultMap(ex.Input))
	resumeJSON, _ := json.Marshal(defaultMap(ex.ResumeData))
	outputJSON, _ := json.Marshal(defaultMap(ex.Output))
	var suspendedAt any
	if ex.State == ExecutionStateSuspended {
		suspendedAt = time.Now()
	}
	row := conn.QueryRow(ctx, `
		INSERT INTO workflow_execution
			(id, workflow_id, workflow_slug, state, input, current_step_id, suspended_at, resume_data, output)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+executionColumns,
		ex.ID, ex.WorkflowID, ex.WorkflowSlug, string(ex.State), inputJSON, ex.CurrentStepID, suspendedAt, resumeJSON, outputJSON,
	)
	created, err := scanExecution(row)
	if err != nil {
		return Execution{}, fmt.Errorf("workflow: create execution: %w", err)
	}
	return created, nil
}

func (r *postgresRepository) GetExecution(ctx context.Context, id uuid.UUID) (Execution, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return Execution{}, err
	}
	defer release()

	row := conn.QueryRow(ctx, `SELECT `+executionColumns+` FROM workflow_execution WHERE id = $1`, id)
	ex, err := scanExecution(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Execution{}, ErrNotFound
	}
	if err != nil {
		return Execution{}, fmt.Errorf("workflow: get execution: %w", err)
	}
	return ex, nil
}

func (r *postgresRepository) ResumeExecution(ctx context.Context, id uuid.UUID, data map[string]any) (Execution, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return Execution{}, err
	}
	defer release()

	dataJSON, _ := json.Marshal(defaultMap(data))
	row := conn.QueryRow(ctx, `
		UPDATE workflow_execution
		SET state = $2,
		    resume_data = $3,
		    resumed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND state = $4
		RETURNING `+executionColumns,
		id, string(ExecutionStateCompleted), dataJSON, string(ExecutionStateSuspended),
	)
	ex, err := scanExecution(row)
	if errors.Is(err, pgx.ErrNoRows) {
		currentRow := conn.QueryRow(ctx, `SELECT `+executionColumns+` FROM workflow_execution WHERE id = $1`, id)
		_, getErr := scanExecution(currentRow)
		if errors.Is(getErr, pgx.ErrNoRows) {
			return Execution{}, ErrNotFound
		}
		if getErr != nil {
			return Execution{}, getErr
		}
		return Execution{}, ErrAlreadyResolved
	}
	if err != nil {
		return Execution{}, fmt.Errorf("workflow: resume execution: %w", err)
	}
	return ex, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanWorkflow(row scannable) (Workflow, error) {
	var wf Workflow
	var stepsRaw []byte
	if err := row.Scan(&wf.ID, &wf.Slug, &wf.Name, &wf.Description, &wf.Start, &stepsRaw, &wf.CreatedAt, &wf.UpdatedAt); err != nil {
		return Workflow{}, err
	}
	if len(stepsRaw) > 0 {
		if err := json.Unmarshal(stepsRaw, &wf.Steps); err != nil {
			return Workflow{}, fmt.Errorf("workflow: unmarshal steps: %w", err)
		}
	}
	if wf.Steps == nil {
		wf.Steps = []Step{}
	}
	return wf, nil
}

func scanExecution(row scannable) (Execution, error) {
	var ex Execution
	var state string
	var inputRaw, resumeRaw, outputRaw []byte
	if err := row.Scan(
		&ex.ID, &ex.WorkflowID, &ex.WorkflowSlug, &state, &inputRaw, &ex.CurrentStepID, &ex.SuspendedAt,
		&ex.ResumedAt, &resumeRaw, &outputRaw, &ex.CreatedAt, &ex.UpdatedAt,
	); err != nil {
		return Execution{}, err
	}
	ex.State = ExecutionState(state)
	ex.Input = decodeMap(inputRaw)
	ex.ResumeData = decodeMap(resumeRaw)
	ex.Output = decodeMap(outputRaw)
	return ex, nil
}

func stepConfig(step Step) map[string]any {
	cfg := map[string]any{}
	for k, v := range step.Config {
		cfg[k] = v
	}
	if step.AgentID != "" {
		cfg["agentId"] = step.AgentID
	}
	if step.ToolID != "" {
		cfg["toolId"] = step.ToolID
	}
	if step.Condition != "" {
		cfg["condition"] = step.Condition
	}
	return cfg
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func defaultMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	return values
}

func decodeMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}
