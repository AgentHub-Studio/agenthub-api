package knowledgebase

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

// Repository defines the persistence interface for KnowledgeBase.
type Repository interface {
	List(ctx context.Context, req pagination.PageRequest) ([]KnowledgeBase, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (KnowledgeBase, error)
	Create(ctx context.Context, k KnowledgeBase) (KnowledgeBase, error)
	Update(ctx context.Context, k KnowledgeBase) (KnowledgeBase, error)
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status KnowledgeBaseStatus) (KnowledgeBase, error)
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new postgres-backed Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) List(ctx context.Context, req pagination.PageRequest) ([]KnowledgeBase, int64, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var total int64
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM knowledge_base`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("knowledgebase: count: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, name, description, status, created_at, updated_at
		 FROM knowledge_base
		 ORDER BY name
		 LIMIT $1 OFFSET $2`,
		req.Size, req.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("knowledgebase: list: %w", err)
	}
	defer rows.Close()

	var items []KnowledgeBase
	for rows.Next() {
		var kb KnowledgeBase
		if err := rows.Scan(&kb.ID, &kb.Name, &kb.Description, &kb.Status, &kb.CreatedAt, &kb.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("knowledgebase: scan: %w", err)
		}
		items = append(items, kb)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("knowledgebase: rows: %w", err)
	}

	return items, total, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (KnowledgeBase, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return KnowledgeBase{}, err
	}
	defer release()

	var kb KnowledgeBase
	err = conn.QueryRow(ctx,
		`SELECT id, name, description, status, created_at, updated_at
		 FROM knowledge_base WHERE id = $1`,
		id,
	).Scan(&kb.ID, &kb.Name, &kb.Description, &kb.Status, &kb.CreatedAt, &kb.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return KnowledgeBase{}, ErrNotFound
		}
		return KnowledgeBase{}, fmt.Errorf("knowledgebase: get by id: %w", err)
	}

	return kb, nil
}

func (r *postgresRepository) Create(ctx context.Context, k KnowledgeBase) (KnowledgeBase, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return KnowledgeBase{}, err
	}
	defer release()

	now := time.Now().UTC()
	k.ID = uuid.New()
	k.CreatedAt = now
	k.UpdatedAt = now

	_, err = conn.Exec(ctx,
		`INSERT INTO knowledge_base (id, name, description, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		k.ID, k.Name, k.Description, k.Status, k.CreatedAt, k.UpdatedAt,
	)
	if err != nil {
		return KnowledgeBase{}, fmt.Errorf("knowledgebase: create: %w", err)
	}

	return k, nil
}

func (r *postgresRepository) Update(ctx context.Context, k KnowledgeBase) (KnowledgeBase, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return KnowledgeBase{}, err
	}
	defer release()

	k.UpdatedAt = time.Now().UTC()
	tag, err := conn.Exec(ctx,
		`UPDATE knowledge_base
		 SET name = $1, description = $2, updated_at = $3
		 WHERE id = $4`,
		k.Name, k.Description, k.UpdatedAt, k.ID,
	)
	if err != nil {
		return KnowledgeBase{}, fmt.Errorf("knowledgebase: update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return KnowledgeBase{}, ErrNotFound
	}

	return k, nil
}

func (r *postgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx, `DELETE FROM knowledge_base WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("knowledgebase: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *postgresRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status KnowledgeBaseStatus) (KnowledgeBase, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return KnowledgeBase{}, err
	}
	defer release()

	now := time.Now().UTC()
	var kb KnowledgeBase
	err = conn.QueryRow(ctx,
		`UPDATE knowledge_base
		 SET status = $1, updated_at = $2
		 WHERE id = $3
		 RETURNING id, name, description, status, created_at, updated_at`,
		status, now, id,
	).Scan(&kb.ID, &kb.Name, &kb.Description, &kb.Status, &kb.CreatedAt, &kb.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return KnowledgeBase{}, ErrNotFound
		}
		return KnowledgeBase{}, fmt.Errorf("knowledgebase: update status: %w", err)
	}

	return kb, nil
}
