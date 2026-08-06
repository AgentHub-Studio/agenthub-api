package pkg

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// ErrNotFound is returned when a package is not found.
var ErrNotFound = errors.New("package not found")

// ErrSlugConflict is returned when a package slug is already in use.
// Maps to HTTP 409 in handlers.
var ErrSlugConflict = errors.New("package: slug already in use")

// Repository provides data access for package_registry.
type Repository struct {
	pool          *pgxpool.Pool
	queryEmbedder QueryEmbedder
}

// NewRepository creates a new package Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// WithQueryEmbedder configures semantic query generation. Without it, Search
// deliberately falls back to lexical matching instead of mixing vector spaces.
func (r *Repository) WithQueryEmbedder(embedder QueryEmbedder) *Repository {
	r.queryEmbedder = embedder
	return r
}

// ListPublic returns a paginated list of PUBLIC packages.
func (r *Repository) ListPublic(ctx context.Context, req pagination.PageRequest) ([]Package, int64, error) {
	const countQuery = `SELECT COUNT(*) FROM public.package_registry WHERE visibility = 'PUBLIC'`
	const query = `
		SELECT id, name, slug, COALESCE(description,''), COALESCE(tags, '{}'), type, visibility,
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
		SELECT id, name, slug, COALESCE(description,''), COALESCE(tags, '{}'), type, visibility,
		       author_tenant_id, download_count, COALESCE(latest_version,''), created_at, updated_at
		  FROM public.package_registry
		 WHERE id = $1`

	row := r.pool.QueryRow(ctx, query, id)
	return scanRow(row)
}

// GetBySlug returns a package by its slug.
func (r *Repository) GetBySlug(ctx context.Context, slug string) (Package, error) {
	const query = `
		SELECT id, name, slug, COALESCE(description,''), COALESCE(tags, '{}'), type, visibility,
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
		SELECT id, name, slug, COALESCE(description,''), COALESCE(tags, '{}'), type, visibility,
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
		INSERT INTO public.package_registry (name, slug, description, tags, type, visibility, author_tenant_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, slug, COALESCE(description,''), COALESCE(tags, '{}'), type, visibility,
		          author_tenant_id, download_count, COALESCE(latest_version,''), created_at, updated_at`

	row := r.pool.QueryRow(ctx, query,
		p.Name, p.Slug, p.Description, p.Tags, string(p.Type), string(p.Visibility), p.AuthorTenantID,
	)
	created, err := scanRow(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Package{}, ErrSlugConflict
		}
		return Package{}, fmt.Errorf("package: create: %w", err)
	}
	return created, nil
}

// Update patches a package's mutable fields.
func (r *Repository) Update(ctx context.Context, id uuid.UUID, name, description, visibility string, tags []string) (Package, error) {
	const query = `
		UPDATE public.package_registry
		   SET name = $2, description = $3, visibility = $4, tags = $5, updated_at = NOW()
		 WHERE id = $1
		RETURNING id, name, slug, COALESCE(description,''), COALESCE(tags, '{}'), type, visibility,
		          author_tenant_id, download_count, COALESCE(latest_version,''), created_at, updated_at`

	row := r.pool.QueryRow(ctx, query, id, name, description, visibility, tags)
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

// Search ranks PUBLIC packages using lexical matches plus pgvector cosine
// similarity. Packages not indexed yet remain searchable by lexical matching.
// This is the backend for GET /api/registry/search?q=...&type=...
func (r *Repository) Search(ctx context.Context, query string, pkgType *string, req pagination.PageRequest) ([]Package, int64, error) {
	pattern := "%" + query + "%"
	vectorString, embeddingModel := r.semanticQuery(ctx, query)
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		  FROM public.package_registry
		 WHERE visibility = 'PUBLIC'
		   AND ($4::text IS NULL OR type = $4)
		   AND ((embedding IS NOT NULL AND $1::vector IS NOT NULL AND embedding_model = $2)
		        OR name ILIKE $3 OR slug ILIKE $3 OR description ILIKE $3
		        OR array_to_string(COALESCE(tags, '{}'), ' ') ILIKE $3)`,
		vectorString, embeddingModel, pattern, pkgType,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("package: search count: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, name, slug, COALESCE(description,''), COALESCE(tags, '{}'), type, visibility,
		       author_tenant_id, download_count, COALESCE(latest_version,''), created_at, updated_at,
		       CASE WHEN embedding IS NULL OR $1::vector IS NULL OR embedding_model IS DISTINCT FROM $2 THEN 0.0
		            ELSE 0.65 * (1 - (embedding <=> $1::vector)) END
		       + CASE
		           WHEN name ILIKE $3 THEN 0.35
		           WHEN slug ILIKE $3 THEN 0.315
		           WHEN description ILIKE $3 THEN 0.245
		           WHEN array_to_string(COALESCE(tags, '{}'), ' ') ILIKE $3 THEN 0.21
		           ELSE 0.0
		         END AS relevance
		  FROM public.package_registry
		 WHERE visibility = 'PUBLIC'
		   AND ($4::text IS NULL OR type = $4)
		   AND ((embedding IS NOT NULL AND $1::vector IS NOT NULL AND embedding_model = $2)
		        OR name ILIKE $3 OR slug ILIKE $3 OR description ILIKE $3
		        OR array_to_string(COALESCE(tags, '{}'), ' ') ILIKE $3)
		 ORDER BY relevance DESC, download_count DESC, created_at DESC
		 LIMIT $5 OFFSET $6`,
		vectorString, embeddingModel, pattern, pkgType, req.Size, req.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("package: search: %w", err)
	}
	defer rows.Close()

	pkgs, err := scanSearchRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return pkgs, total, nil
}

func (r *Repository) semanticQuery(ctx context.Context, query string) (any, string) {
	if r.queryEmbedder == nil {
		return nil, ""
	}
	embedding, err := r.queryEmbedder.Embed(ctx, query)
	if err != nil || len(embedding.Vector) == 0 || embedding.Model == "" {
		return nil, ""
	}
	return float32SliceToVector(embedding.Vector), embedding.Model
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
		&p.ID, &p.Name, &p.Slug, &p.Description, &p.Tags,
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
			&p.ID, &p.Name, &p.Slug, &p.Description, &p.Tags,
			&p.Type, &p.Visibility, &p.AuthorTenantID,
			&p.DownloadCount, &p.LatestVersion, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("package: scan row: %w", err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, rows.Err()
}

func scanSearchRows(rows pgx.Rows) ([]Package, error) {
	pkgs := make([]Package, 0)
	for rows.Next() {
		var p Package
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Slug, &p.Description, &p.Tags,
			&p.Type, &p.Visibility, &p.AuthorTenantID,
			&p.DownloadCount, &p.LatestVersion, &p.CreatedAt, &p.UpdatedAt,
			&p.Relevance,
		); err != nil {
			return nil, fmt.Errorf("package: scan search row: %w", err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, rows.Err()
}

func float32SliceToVector(vector []float32) string {
	var builder strings.Builder
	builder.Grow(len(vector) * 10)
	builder.WriteByte('[')
	for i, value := range vector {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	builder.WriteByte(']')
	return builder.String()
}
