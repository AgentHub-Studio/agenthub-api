package pkg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// ErrNotFound is returned when a package is not found.
var ErrNotFound = errors.New("package not found")

// Repository provides data access for package_registry.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new package Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ListPublic returns a paginated list of PUBLIC packages.
func (r *Repository) ListPublic(ctx context.Context, req pagination.PageRequest) ([]Package, int64, error) {
	const countQuery = `SELECT COUNT(*) FROM public.package_registry WHERE visibility = 'PUBLIC'`
	const query = `
		SELECT id, name, slug, COALESCE(description,''), type, visibility,
		       author_tenant_id, download_count, COALESCE(latest_version,''), created_at, updated_at
		  FROM public.package_registry
		 WHERE visibility = 'PUBLIC'
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`

	var total int64
	if err := r.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("package: count public: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, req.Size, req.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("package: list public: %w", err)
	}
	defer rows.Close()

	pkgs, err := scanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return pkgs, total, nil
}

// GetByID returns a package by its UUID.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (Package, error) {
	const query = `
		SELECT id, name, slug, COALESCE(description,''), type, visibility,
		       author_tenant_id, download_count, COALESCE(latest_version,''), created_at, updated_at
		  FROM public.package_registry
		 WHERE id = $1`

	row := r.pool.QueryRow(ctx, query, id)
	return scanRow(row)
}

// GetBySlug returns a package by its slug.
func (r *Repository) GetBySlug(ctx context.Context, slug string) (Package, error) {
	const query = `
		SELECT id, name, slug, COALESCE(description,''), type, visibility,
		       author_tenant_id, download_count, COALESCE(latest_version,''), created_at, updated_at
		  FROM public.package_registry
		 WHERE slug = $1`

	row := r.pool.QueryRow(ctx, query, slug)
	return scanRow(row)
}

// ListByTenant returns paginated packages owned by a tenant (all visibilities).
func (r *Repository) ListByTenant(ctx context.Context, tenantID string, req pagination.PageRequest) ([]Package, int64, error) {
	const countQuery = `SELECT COUNT(*) FROM public.package_registry WHERE author_tenant_id = $1`
	const query = `
		SELECT id, name, slug, COALESCE(description,''), type, visibility,
		       author_tenant_id, download_count, COALESCE(latest_version,''), created_at, updated_at
		  FROM public.package_registry
		 WHERE author_tenant_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`

	var total int64
	if err := r.pool.QueryRow(ctx, countQuery, tenantID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("package: count by tenant: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, tenantID, req.Size, req.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("package: list by tenant: %w", err)
	}
	defer rows.Close()

	pkgs, err := scanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return pkgs, total, nil
}

// Create inserts a new package and returns the created entity.
func (r *Repository) Create(ctx context.Context, p Package) (Package, error) {
	const query = `
		INSERT INTO public.package_registry (name, slug, description, type, visibility, author_tenant_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, slug, COALESCE(description,''), type, visibility,
		          author_tenant_id, download_count, COALESCE(latest_version,''), created_at, updated_at`

	row := r.pool.QueryRow(ctx, query,
		p.Name, p.Slug, p.Description, string(p.Type), string(p.Visibility), p.AuthorTenantID,
	)
	created, err := scanRow(row)
	if err != nil {
		return Package{}, fmt.Errorf("package: create: %w", err)
	}
	return created, nil
}

// Update patches a package's mutable fields.
func (r *Repository) Update(ctx context.Context, id uuid.UUID, name, description, visibility string) (Package, error) {
	const query = `
		UPDATE public.package_registry
		   SET name = $2, description = $3, visibility = $4, updated_at = NOW()
		 WHERE id = $1
		RETURNING id, name, slug, COALESCE(description,''), type, visibility,
		          author_tenant_id, download_count, COALESCE(latest_version,''), created_at, updated_at`

	row := r.pool.QueryRow(ctx, query, id, name, description, visibility)
	updated, err := scanRow(row)
	if err != nil {
		return Package{}, fmt.Errorf("package: update: %w", err)
	}
	return updated, nil
}

// Delete removes a package by ID.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM public.package_registry WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("package: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Search performs a full-text ILIKE search on name, slug, and description
// across PUBLIC packages. An optional pkgType filter restricts the results.
// This is the backend for GET /api/registry/search?q=...&type=...
func (r *Repository) Search(ctx context.Context, query string, pkgType *string, req pagination.PageRequest) ([]Package, int64, error) {
	pattern := "%" + query + "%"
	var total int64
	var err error

	if pkgType != nil && *pkgType != "" {
		err = r.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM public.package_registry
			  WHERE visibility = 'PUBLIC' AND type = $1
			    AND (name ILIKE $2 OR slug ILIKE $2 OR description ILIKE $2)`,
			*pkgType, pattern,
		).Scan(&total)
	} else {
		err = r.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM public.package_registry
			  WHERE visibility = 'PUBLIC'
			    AND (name ILIKE $1 OR slug ILIKE $1 OR description ILIKE $1)`,
			pattern,
		).Scan(&total)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("package: search count: %w", err)
	}

	var rows pgx.Rows
	if pkgType != nil && *pkgType != "" {
		rows, err = r.pool.Query(ctx,
			`SELECT id, name, slug, COALESCE(description,''), type, visibility,
			        author_tenant_id, download_count, COALESCE(latest_version,''), created_at, updated_at
			   FROM public.package_registry
			  WHERE visibility = 'PUBLIC' AND type = $1
			    AND (name ILIKE $2 OR slug ILIKE $2 OR description ILIKE $2)
			  ORDER BY download_count DESC, created_at DESC
			  LIMIT $3 OFFSET $4`,
			*pkgType, pattern, req.Size, req.Offset(),
		)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, name, slug, COALESCE(description,''), type, visibility,
			        author_tenant_id, download_count, COALESCE(latest_version,''), created_at, updated_at
			   FROM public.package_registry
			  WHERE visibility = 'PUBLIC'
			    AND (name ILIKE $1 OR slug ILIKE $1 OR description ILIKE $1)
			  ORDER BY download_count DESC, created_at DESC
			  LIMIT $2 OFFSET $3`,
			pattern, req.Size, req.Offset(),
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("package: search: %w", err)
	}
	defer rows.Close()

	pkgs, err := scanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return pkgs, total, nil
}

// UpdateLatestVersion updates the latest_version field after a new version is published.
func (r *Repository) UpdateLatestVersion(ctx context.Context, id uuid.UUID, version string) error {
	const query = `UPDATE public.package_registry SET latest_version = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id, version)
	if err != nil {
		return fmt.Errorf("package: update latest version: %w", err)
	}
	return nil
}

// scanRow scans a single package row.
func scanRow(row pgx.Row) (Package, error) {
	var p Package
	err := row.Scan(
		&p.ID, &p.Name, &p.Slug, &p.Description,
		&p.Type, &p.Visibility, &p.AuthorTenantID,
		&p.DownloadCount, &p.LatestVersion, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Package{}, ErrNotFound
	}
	if err != nil {
		return Package{}, fmt.Errorf("package: scan: %w", err)
	}
	return p, nil
}

// scanRows scans multiple package rows.
func scanRows(rows pgx.Rows) ([]Package, error) {
	var pkgs []Package
	for rows.Next() {
		var p Package
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Slug, &p.Description,
			&p.Type, &p.Visibility, &p.AuthorTenantID,
			&p.DownloadCount, &p.LatestVersion, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("package: scan row: %w", err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, rows.Err()
}
