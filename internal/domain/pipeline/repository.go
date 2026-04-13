package pipeline

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

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a postgres-backed read-only pipeline repository.
func NewRepository(pool *pgxpool.Pool) pipelineRepository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) List(ctx context.Context, req pagination.PageRequest) (pagination.Page[PipelineResponse], error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return pagination.Page[PipelineResponse]{}, err
	}
	defer release()

	var total int64
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM pipeline`).Scan(&total); err != nil {
		return pagination.Page[PipelineResponse]{}, fmt.Errorf("pipeline: count: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, name, description, agent_id, status, config, created_at, updated_at
		 FROM pipeline
		 ORDER BY name
		 LIMIT $1 OFFSET $2`,
		req.Size, req.Offset(),
	)
	if err != nil {
		return pagination.Page[PipelineResponse]{}, fmt.Errorf("pipeline: list: %w", err)
	}
	defer rows.Close()

	var pipelines []PipelineResponse
	for rows.Next() {
		p, scanErr := scanPipeline(rows)
		if scanErr != nil {
			return pagination.Page[PipelineResponse]{}, scanErr
		}
		pipelines = append(pipelines, p)
	}
	if rows.Err() != nil {
		return pagination.Page[PipelineResponse]{}, fmt.Errorf("pipeline: rows: %w", rows.Err())
	}

	return pagination.NewPage(pipelines, total, req), nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (PipelineResponse, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return PipelineResponse{}, err
	}
	defer release()

	row := conn.QueryRow(ctx,
		`SELECT id, name, description, agent_id, status, config, created_at, updated_at
		 FROM pipeline WHERE id = $1`,
		id,
	)
	p, err := scanPipeline(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PipelineResponse{}, errors.New("not found")
		}
		return PipelineResponse{}, fmt.Errorf("pipeline: get: %w", err)
	}
	return p, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanPipeline(row scannable) (PipelineResponse, error) {
	var p PipelineResponse
	var config []byte
	var createdAt, updatedAt time.Time
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.AgentID, &p.Status, &config, &createdAt, &updatedAt); err != nil {
		return PipelineResponse{}, err
	}
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	p.Config = string(config)
	return p, nil
}
