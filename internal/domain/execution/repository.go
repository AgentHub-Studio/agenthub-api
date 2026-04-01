package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// ErrNotFound is returned when an execution is not found.
var ErrNotFound = errors.New("execution: not found")

// ExecutionRepository defines the persistence interface for AgentExecution.
type ExecutionRepository interface {
	List(ctx context.Context, agentID *uuid.UUID, status *string, req pagination.PageRequest) ([]AgentExecution, int64, error)
	Create(ctx context.Context, e AgentExecution) (AgentExecution, error)
	GetByID(ctx context.Context, id uuid.UUID) (AgentExecution, error)
	Cancel(ctx context.Context, id uuid.UUID) error
	ListNodes(ctx context.Context, executionID uuid.UUID) ([]AgentExecutionNode, error)
	ListToolExecutions(ctx context.Context, nodeExecutionID uuid.UUID) ([]ToolExecution, error)
}

// Repository provides data access for execution tables.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new execution Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// List returns a page of agent executions with optional filters.
func (r *Repository) List(ctx context.Context, agentID *uuid.UUID, status *string, req pagination.PageRequest) ([]AgentExecution, int64, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	const countQuery = `
		SELECT COUNT(*) FROM agent_execution
		 WHERE ($1::UUID IS NULL OR agent_id = $1)
		   AND ($2::VARCHAR IS NULL OR status = $2)`
	var total int64
	if err := conn.QueryRow(ctx, countQuery, agentID, status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("execution: count: %w", err)
	}

	const query = `
		SELECT id, agent_id, pipeline_id, status, input, output, error_message,
		       started_at, finished_at, duration_ms
		  FROM agent_execution
		 WHERE ($1::UUID IS NULL OR agent_id = $1)
		   AND ($2::VARCHAR IS NULL OR status = $2)
		 ORDER BY started_at DESC
		 LIMIT $3 OFFSET $4`
	rows, err := conn.Query(ctx, query, agentID, status, req.Size, req.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("execution: list: %w", err)
	}
	defer rows.Close()

	items, err := scanExecutionRows(rows)
	return items, total, err
}

// Create inserts a new AgentExecution record.
func (r *Repository) Create(ctx context.Context, e AgentExecution) (AgentExecution, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return AgentExecution{}, err
	}
	defer release()

	const query = `
		INSERT INTO agent_execution (agent_id, pipeline_id, status, input)
		VALUES ($1, $2, $3, $4)
		RETURNING id, agent_id, pipeline_id, status, input, output, error_message,
		          started_at, finished_at, duration_ms`
	row := conn.QueryRow(ctx, query, e.AgentID, e.PipelineID, e.Status, e.Input)
	return scanExecutionRow(row)
}

// GetByID returns a single execution.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (AgentExecution, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return AgentExecution{}, err
	}
	defer release()

	const query = `
		SELECT id, agent_id, pipeline_id, status, input, output, error_message,
		       started_at, finished_at, duration_ms
		  FROM agent_execution WHERE id = $1`
	row := conn.QueryRow(ctx, query, id)
	return scanExecutionRow(row)
}

// Cancel sets execution status to CANCELLED.
func (r *Repository) Cancel(ctx context.Context, id uuid.UUID) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	ct, err := conn.Exec(ctx,
		`UPDATE agent_execution SET status = 'CANCELLED', finished_at = NOW() WHERE id = $1 AND status = 'RUNNING'`,
		id)
	if err != nil {
		return fmt.Errorf("execution: cancel: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListNodes returns node executions for a given execution.
func (r *Repository) ListNodes(ctx context.Context, executionID uuid.UUID) ([]AgentExecutionNode, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	const query = `
		SELECT id, execution_id, node_id, node_type, status, input, output, error_message,
		       started_at, finished_at, duration_ms
		  FROM agent_execution_node
		 WHERE execution_id = $1
		 ORDER BY started_at`
	rows, err := conn.Query(ctx, query, executionID)
	if err != nil {
		return nil, fmt.Errorf("execution: list nodes: %w", err)
	}
	defer rows.Close()
	return scanNodeRows(rows)
}

// ListToolExecutions returns tool executions for a given node execution.
func (r *Repository) ListToolExecutions(ctx context.Context, nodeExecutionID uuid.UUID) ([]ToolExecution, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	const query = `
		SELECT id, node_execution_id, tool_id, status, input, output, error_message,
		       started_at, finished_at, duration_ms
		  FROM tool_execution
		 WHERE node_execution_id = $1
		 ORDER BY started_at`
	rows, err := conn.Query(ctx, query, nodeExecutionID)
	if err != nil {
		return nil, fmt.Errorf("execution: list tools: %w", err)
	}
	defer rows.Close()
	return scanToolRows(rows)
}

func scanExecutionRow(row pgx.Row) (AgentExecution, error) {
	var e AgentExecution
	var finishedAt *time.Time
	err := row.Scan(
		&e.ID, &e.AgentID, &e.PipelineID, &e.Status, &e.Input, &e.Output,
		&e.ErrorMessage, &e.StartedAt, &finishedAt, &e.DurationMs,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentExecution{}, ErrNotFound
	}
	if err != nil {
		return AgentExecution{}, fmt.Errorf("execution: scan: %w", err)
	}
	e.FinishedAt = finishedAt
	return e, nil
}

func scanExecutionRows(rows pgx.Rows) ([]AgentExecution, error) {
	var items []AgentExecution
	for rows.Next() {
		var e AgentExecution
		var finishedAt *time.Time
		if err := rows.Scan(
			&e.ID, &e.AgentID, &e.PipelineID, &e.Status, &e.Input, &e.Output,
			&e.ErrorMessage, &e.StartedAt, &finishedAt, &e.DurationMs,
		); err != nil {
			return nil, fmt.Errorf("execution: scan row: %w", err)
		}
		e.FinishedAt = finishedAt
		items = append(items, e)
	}
	return items, rows.Err()
}

func scanNodeRows(rows pgx.Rows) ([]AgentExecutionNode, error) {
	var items []AgentExecutionNode
	for rows.Next() {
		var n AgentExecutionNode
		var startedAt, finishedAt *time.Time
		if err := rows.Scan(
			&n.ID, &n.ExecutionID, &n.NodeID, &n.NodeType, &n.Status, &n.Input, &n.Output,
			&n.ErrorMessage, &startedAt, &finishedAt, &n.DurationMs,
		); err != nil {
			return nil, fmt.Errorf("execution: scan node: %w", err)
		}
		n.StartedAt = startedAt
		n.FinishedAt = finishedAt
		items = append(items, n)
	}
	return items, rows.Err()
}

func scanToolRows(rows pgx.Rows) ([]ToolExecution, error) {
	var items []ToolExecution
	for rows.Next() {
		var t ToolExecution
		var finishedAt *time.Time
		if err := rows.Scan(
			&t.ID, &t.NodeExecutionID, &t.ToolID, &t.Status, &t.Input, &t.Output,
			&t.ErrorMessage, &t.StartedAt, &finishedAt, &t.DurationMs,
		); err != nil {
			return nil, fmt.Errorf("execution: scan tool: %w", err)
		}
		t.FinishedAt = finishedAt
		items = append(items, t)
	}
	return items, rows.Err()
}
