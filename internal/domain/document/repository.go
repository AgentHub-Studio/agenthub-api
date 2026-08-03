package document

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

// Repository defines the persistence interface for Document.
type Repository interface {
	FindByKnowledgeBase(ctx context.Context, kbID uuid.UUID, req pagination.PageRequest) ([]Document, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (Document, error)
	Create(ctx context.Context, d Document) (Document, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status DocumentStatus) (Document, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new postgres-backed Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) FindByKnowledgeBase(ctx context.Context, kbID uuid.UUID, req pagination.PageRequest) ([]Document, int64, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var total int64
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM document WHERE knowledge_base_id = $1`, kbID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("document: count: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, knowledge_base_id, file_name, content_type, status, storage_path, file_size, metadata, created_at, updated_at
		 FROM document
		 WHERE knowledge_base_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		kbID, req.Size, req.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("document: find by kb: %w", err)
	}
	defer rows.Close()

	var items []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.KnowledgeBaseID, &d.FileName, &d.ContentType, &d.Status, &d.StoragePath, &d.FileSize, &d.Metadata, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("document: scan: %w", err)
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("document: rows: %w", err)
	}

	return items, total, nil
}

func (r *postgresRepository) FindByID(ctx context.Context, id uuid.UUID) (Document, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return Document{}, err
	}
	defer release()

	var d Document
	err = conn.QueryRow(ctx,
		`SELECT id, knowledge_base_id, file_name, content_type, status, storage_path, file_size, metadata, created_at, updated_at
		 FROM document WHERE id = $1`,
		id,
	).Scan(&d.ID, &d.KnowledgeBaseID, &d.FileName, &d.ContentType, &d.Status, &d.StoragePath, &d.FileSize, &d.Metadata, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Document{}, ErrNotFound
		}
		return Document{}, fmt.Errorf("document: find by id: %w", err)
	}

	return d, nil
}

func (r *postgresRepository) Create(ctx context.Context, d Document) (Document, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return Document{}, err
	}
	defer release()

	now := time.Now().UTC()
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	d.CreatedAt = now
	d.UpdatedAt = now
	if len(d.Metadata) == 0 {
		d.Metadata = []byte(`{}`)
	}

	_, err = conn.Exec(ctx,
		`INSERT INTO document (id, knowledge_base_id, file_name, content_type, status, storage_path, file_size, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10)`,
		d.ID, d.KnowledgeBaseID, d.FileName, d.ContentType, d.Status, d.StoragePath, d.FileSize, string(d.Metadata), d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		return Document{}, fmt.Errorf("document: create: %w", err)
	}

	return d, nil
}

func (r *postgresRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status DocumentStatus) (Document, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return Document{}, err
	}
	defer release()

	now := time.Now().UTC()
	var d Document
	err = conn.QueryRow(ctx,
		`UPDATE document
		 SET status = $1, updated_at = $2
		 WHERE id = $3
		 RETURNING id, knowledge_base_id, file_name, content_type, status, storage_path, file_size, metadata, created_at, updated_at`,
		status, now, id,
	).Scan(&d.ID, &d.KnowledgeBaseID, &d.FileName, &d.ContentType, &d.Status, &d.StoragePath, &d.FileSize, &d.Metadata, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Document{}, ErrNotFound
		}
		return Document{}, fmt.Errorf("document: update status: %w", err)
	}

	return d, nil
}

func (r *postgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx, `DELETE FROM document WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("document: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
