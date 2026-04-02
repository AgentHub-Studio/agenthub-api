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

const selectColumns = `id, name, description, status, embedding_model, search_mode, context_window, created_at, updated_at`

func scanKB(row pgx.Row, kb *KnowledgeBase) error {
	return row.Scan(
		&kb.ID, &kb.Name, &kb.Description, &kb.Status,
		&kb.EmbeddingModel, &kb.SearchMode, &kb.ContextWindow,
		&kb.CreatedAt, &kb.UpdatedAt,
	)
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
		`SELECT kb.`+selectColumns+`,
		        (SELECT COUNT(*) FROM document d WHERE d.knowledge_base_id = kb.id) AS document_count
		 FROM knowledge_base kb
		 ORDER BY kb.name
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
		if err := rows.Scan(
			&kb.ID, &kb.Name, &kb.Description, &kb.Status,
			&kb.EmbeddingModel, &kb.SearchMode, &kb.ContextWindow,
			&kb.CreatedAt, &kb.UpdatedAt, &kb.DocumentCount,
		); err != nil {
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
		`SELECT `+selectColumns+` FROM knowledge_base WHERE id = $1`,
		id,
	).Scan(
		&kb.ID, &kb.Name, &kb.Description, &kb.Status,
		&kb.EmbeddingModel, &kb.SearchMode, &kb.ContextWindow,
		&kb.CreatedAt, &kb.UpdatedAt,
	)
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

	if k.EmbeddingModel == "" {
		k.EmbeddingModel = "intfloat/multilingual-e5-large"
	}
	if k.SearchMode == "" {
		k.SearchMode = "HYBRID"
	}

	err = conn.QueryRow(ctx,
		`INSERT INTO knowledge_base (id, name, description, status, embedding_model, search_mode, context_window, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING `+selectColumns,
		k.ID, k.Name, k.Description, k.Status,
		k.EmbeddingModel, k.SearchMode, k.ContextWindow,
		k.CreatedAt, k.UpdatedAt,
	).Scan(
		&k.ID, &k.Name, &k.Description, &k.Status,
		&k.EmbeddingModel, &k.SearchMode, &k.ContextWindow,
		&k.CreatedAt, &k.UpdatedAt,
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
	err = conn.QueryRow(ctx,
		`UPDATE knowledge_base
		 SET name = $1, description = $2, embedding_model = $3, search_mode = $4, context_window = $5, updated_at = $6
		 WHERE id = $7
		 RETURNING `+selectColumns,
		k.Name, k.Description, k.EmbeddingModel, k.SearchMode, k.ContextWindow, k.UpdatedAt, k.ID,
	).Scan(
		&k.ID, &k.Name, &k.Description, &k.Status,
		&k.EmbeddingModel, &k.SearchMode, &k.ContextWindow,
		&k.CreatedAt, &k.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return KnowledgeBase{}, ErrNotFound
		}
		return KnowledgeBase{}, fmt.Errorf("knowledgebase: update: %w", err)
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
		 RETURNING `+selectColumns,
		status, now, id,
	).Scan(
		&kb.ID, &kb.Name, &kb.Description, &kb.Status,
		&kb.EmbeddingModel, &kb.SearchMode, &kb.ContextWindow,
		&kb.CreatedAt, &kb.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return KnowledgeBase{}, ErrNotFound
		}
		return KnowledgeBase{}, fmt.Errorf("knowledgebase: update status: %w", err)
	}

	return kb, nil
}
