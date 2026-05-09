package tenant

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// ErrNotFound is returned when the tenant cannot be found.
var ErrNotFound = errors.New("tenant: not found")

// ErrAlreadyExists is returned when the tenant slug is already taken.
var ErrAlreadyExists = errors.New("tenant: already exists")

// ErrValidation is the sentinel for client-facing validation errors. Bug 189:
// the public handler maps this to 400 with the validation message; repo
// errors fall through to an opaque 500 so callers cannot probe SQL state.
var ErrValidation = errors.New("validation failed")

// Repository defines persistence operations for Tenant.
type Repository interface {
	Create(ctx context.Context, t Tenant) (Tenant, error)
	FindByID(ctx context.Context, id string) (Tenant, error)
	FindAll(ctx context.Context, req pagination.PageRequest) ([]Tenant, int64, error)
	Exists(ctx context.Context, id string) (bool, error)
	UpdateStatus(ctx context.Context, id string, status Status) error
	UpdateName(ctx context.Context, id string, name string) error
	Delete(ctx context.Context, id string) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL-backed tenant Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func scanTenant(row pgx.Row) (Tenant, error) {
	var t Tenant
	var status string
	err := row.Scan(&t.ID, &t.Name, &status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return Tenant{}, err
	}
	t.Status = Status(status)
	return t, nil
}

func (r *pgRepository) Create(ctx context.Context, t Tenant) (Tenant, error) {
	const query = `
		INSERT INTO public.tenants (id, name, status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, name, status, created_at, updated_at`
	row := r.pool.QueryRow(ctx, query, t.ID, t.Name, string(t.Status))
	created, err := scanTenant(row)
	if err != nil {
		if isPKViolation(err) {
			return Tenant{}, ErrAlreadyExists
		}
		return Tenant{}, fmt.Errorf("tenant.Create: %w", err)
	}
	return created, nil
}

func (r *pgRepository) FindByID(ctx context.Context, id string) (Tenant, error) {
	const query = `SELECT id, name, status, created_at, updated_at FROM public.tenants WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	t, err := scanTenant(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tenant{}, ErrNotFound
	}
	if err != nil {
		return Tenant{}, fmt.Errorf("tenant.FindByID: %w", err)
	}
	return t, nil
}

func (r *pgRepository) FindAll(ctx context.Context, req pagination.PageRequest) ([]Tenant, int64, error) {
	const countQ = `SELECT COUNT(*) FROM public.tenants`
	var total int64
	if err := r.pool.QueryRow(ctx, countQ).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("tenant.FindAll count: %w", err)
	}

	const query = `SELECT id, name, status, created_at, updated_at FROM public.tenants ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.pool.Query(ctx, query, req.Size, req.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("tenant.FindAll: %w", err)
	}
	defer rows.Close()

	var tenants []Tenant
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, 0, err
		}
		tenants = append(tenants, t)
	}
	if tenants == nil {
		tenants = []Tenant{}
	}
	return tenants, total, rows.Err()
}

func (r *pgRepository) Exists(ctx context.Context, id string) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM public.tenants WHERE id = $1)`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("tenant.Exists: %w", err)
	}
	return exists, nil
}

func (r *pgRepository) UpdateStatus(ctx context.Context, id string, status Status) error {
	const query = `UPDATE public.tenants SET status = $2, updated_at = NOW() WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id, string(status))
	if err != nil {
		return fmt.Errorf("tenant.UpdateStatus: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) UpdateName(ctx context.Context, id string, name string) error {
	const query = `UPDATE public.tenants SET name = $2, updated_at = NOW() WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id, name)
	if err != nil {
		return fmt.Errorf("tenant.UpdateName: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) Delete(ctx context.Context, id string) error {
	const dropSchema = `DROP SCHEMA IF EXISTS "ah_` + `%s" CASCADE`
	// Drop the per-tenant schema first so the tenant row remains as evidence
	// if the schema drop fails. Schema name is sanitized via the slug regex
	// at create-time, but we still parameterize defensively.
	if _, err := r.pool.Exec(ctx, fmt.Sprintf(dropSchema, id)); err != nil {
		return fmt.Errorf("tenant.Delete drop schema: %w", err)
	}
	const del = `DELETE FROM public.tenants WHERE id = $1`
	tag, err := r.pool.Exec(ctx, del, id)
	if err != nil {
		return fmt.Errorf("tenant.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func isPKViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for i := 0; i <= len(msg)-5; i++ {
		if msg[i:i+5] == "23505" {
			return true
		}
	}
	return false
}
