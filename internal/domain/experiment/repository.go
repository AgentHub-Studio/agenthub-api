package experiment

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Repository handles persistence for PromptExperiment and ExperimentResult.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ListAll returns a paginated list of experiments for the given tenant.
func (r *Repository) ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]PromptExperiment, int64, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var total int64
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM prompt_experiment`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("experiment: count: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, agent_id, name, status, traffic_split, variants, start_date, end_date, created_at
		 FROM prompt_experiment
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`,
		pr.Size, pr.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("experiment: list: %w", err)
	}
	defer rows.Close()

	var items []PromptExperiment
	for rows.Next() {
		var e PromptExperiment
		if err := rows.Scan(
			&e.ID, &e.AgentID, &e.Name, &e.Status, &e.TrafficSplit, &e.Variants,
			&e.StartDate, &e.EndDate, &e.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("experiment: scan: %w", err)
		}
		items = append(items, e)
	}
	return items, total, rows.Err()
}

// GetByID retrieves an experiment by ID.
func (r *Repository) GetByID(ctx context.Context, tenantID string, id uuid.UUID) (PromptExperiment, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return PromptExperiment{}, err
	}
	defer release()

	var e PromptExperiment
	err = conn.QueryRow(ctx,
		`SELECT id, agent_id, name, status, traffic_split, variants, start_date, end_date, created_at
		 FROM prompt_experiment WHERE id = $1`,
		id,
	).Scan(&e.ID, &e.AgentID, &e.Name, &e.Status, &e.TrafficSplit, &e.Variants, &e.StartDate, &e.EndDate, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PromptExperiment{}, ErrNotFound
	}
	return e, err
}

// Create inserts a new experiment.
func (r *Repository) Create(ctx context.Context, tenantID string, e PromptExperiment) (PromptExperiment, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return PromptExperiment{}, err
	}
	defer release()

	var created PromptExperiment
	err = conn.QueryRow(ctx,
		`INSERT INTO prompt_experiment (agent_id, name, status, traffic_split, variants, start_date, end_date)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, agent_id, name, status, traffic_split, variants, start_date, end_date, created_at`,
		e.AgentID, e.Name, e.Status, e.TrafficSplit, e.Variants, e.StartDate, e.EndDate,
	).Scan(&created.ID, &created.AgentID, &created.Name, &created.Status, &created.TrafficSplit, &created.Variants,
		&created.StartDate, &created.EndDate, &created.CreatedAt)
	return created, err
}

// Update updates an existing experiment.
func (r *Repository) Update(ctx context.Context, tenantID string, id uuid.UUID, e PromptExperiment) (PromptExperiment, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return PromptExperiment{}, err
	}
	defer release()

	var updated PromptExperiment
	err = conn.QueryRow(ctx,
		`UPDATE prompt_experiment SET
		   agent_id=$1, name=$2, traffic_split=$3, variants=$4, start_date=$5, end_date=$6
		 WHERE id=$7
		 RETURNING id, agent_id, name, status, traffic_split, variants, start_date, end_date, created_at`,
		e.AgentID, e.Name, e.TrafficSplit, e.Variants, e.StartDate, e.EndDate, id,
	).Scan(&updated.ID, &updated.AgentID, &updated.Name, &updated.Status, &updated.TrafficSplit, &updated.Variants,
		&updated.StartDate, &updated.EndDate, &updated.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PromptExperiment{}, ErrNotFound
	}
	return updated, err
}

// UpdateStatus updates only the status of an experiment.
func (r *Repository) UpdateStatus(ctx context.Context, tenantID string, id uuid.UUID, status ExperimentStatus) (PromptExperiment, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return PromptExperiment{}, err
	}
	defer release()

	var updated PromptExperiment
	err = conn.QueryRow(ctx,
		`UPDATE prompt_experiment SET status=$1 WHERE id=$2
		 RETURNING id, agent_id, name, status, traffic_split, variants, start_date, end_date, created_at`,
		status, id,
	).Scan(&updated.ID, &updated.AgentID, &updated.Name, &updated.Status, &updated.TrafficSplit, &updated.Variants,
		&updated.StartDate, &updated.EndDate, &updated.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PromptExperiment{}, ErrNotFound
	}
	return updated, err
}

// Delete removes an experiment by ID.
func (r *Repository) Delete(ctx context.Context, tenantID string, id uuid.UUID) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	ct, err := conn.Exec(ctx, `DELETE FROM prompt_experiment WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("experiment: delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RecordResult inserts a new experiment result.
func (r *Repository) RecordResult(ctx context.Context, tenantID string, res ExperimentResult) (ExperimentResult, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return ExperimentResult{}, err
	}
	defer release()

	var created ExperimentResult
	err = conn.QueryRow(ctx,
		`INSERT INTO experiment_result (experiment_id, variant_key, session_id, user_feedback, latency_ms, token_count)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, experiment_id, variant_key, session_id, user_feedback, latency_ms, token_count, created_at`,
		res.ExperimentID, res.VariantKey, res.SessionID, res.UserFeedback, res.LatencyMs, res.TokenCount,
	).Scan(&created.ID, &created.ExperimentID, &created.VariantKey, &created.SessionID,
		&created.UserFeedback, &created.LatencyMs, &created.TokenCount, &created.CreatedAt)
	return created, err
}

// GetResults returns paginated results for an experiment.
func (r *Repository) GetResults(ctx context.Context, tenantID string, experimentID uuid.UUID, pr pagination.PageRequest) ([]ExperimentResult, int64, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var total int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM experiment_result WHERE experiment_id=$1`, experimentID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("experiment: count results: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, experiment_id, variant_key, session_id, user_feedback, latency_ms, token_count, created_at
		 FROM experiment_result
		 WHERE experiment_id=$1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		experimentID, pr.Size, pr.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("experiment: list results: %w", err)
	}
	defer rows.Close()

	var items []ExperimentResult
	for rows.Next() {
		var res ExperimentResult
		if err := rows.Scan(
			&res.ID, &res.ExperimentID, &res.VariantKey, &res.SessionID,
			&res.UserFeedback, &res.LatencyMs, &res.TokenCount, &res.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("experiment: scan result: %w", err)
		}
		items = append(items, res)
	}
	return items, total, rows.Err()
}
