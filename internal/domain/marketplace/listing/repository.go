// Package listing provides marketplace listing domain logic.
package listing

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Repository handles persistence for marketplace listings.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// FindAll returns all active listings paginated.
func (r *Repository) FindAll(ctx context.Context, req pagination.PageRequest) ([]Listing, int64, error) {
	const q = `SELECT id, tenant_id, package_id, name, slug, description, type, category,
	             status, avg_rating, review_count, created_at, updated_at
	           FROM marketplace_listing
	           WHERE status != 'REMOVED'
	           ORDER BY name ASC
	           LIMIT $1 OFFSET $2`
	const cq = `SELECT COUNT(*) FROM marketplace_listing WHERE status != 'REMOVED'`
	return r.query(ctx, q, cq, req, nil)
}

// FindByType returns listings filtered by package type.
func (r *Repository) FindByType(ctx context.Context, t PackageType, req pagination.PageRequest) ([]Listing, int64, error) {
	const q = `SELECT id, tenant_id, package_id, name, slug, description, type, category,
	             status, avg_rating, review_count, created_at, updated_at
	           FROM marketplace_listing
	           WHERE type = $3 AND status != 'REMOVED'
	           ORDER BY name ASC LIMIT $1 OFFSET $2`
	const cq = `SELECT COUNT(*) FROM marketplace_listing WHERE type = $1 AND status != 'REMOVED'`
	rows, err := r.db.Query(ctx, q, req.Size, req.Offset(), string(t))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	listings, err := scanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRow(ctx, cq, string(t)).Scan(&total); err != nil {
		return nil, 0, err
	}
	return listings, total, nil
}

// FindByCategory returns listings filtered by category.
func (r *Repository) FindByCategory(ctx context.Context, cat string, req pagination.PageRequest) ([]Listing, int64, error) {
	const q = `SELECT id, tenant_id, package_id, name, slug, description, type, category,
	             status, avg_rating, review_count, created_at, updated_at
	           FROM marketplace_listing
	           WHERE category = $3 AND status != 'REMOVED'
	           ORDER BY name ASC LIMIT $1 OFFSET $2`
	const cq = `SELECT COUNT(*) FROM marketplace_listing WHERE category = $1 AND status != 'REMOVED'`
	rows, err := r.db.Query(ctx, q, req.Size, req.Offset(), cat)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	listings, err := scanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRow(ctx, cq, cat).Scan(&total); err != nil {
		return nil, 0, err
	}
	return listings, total, nil
}

// FindBySlug finds a listing by its slug.
func (r *Repository) FindBySlug(ctx context.Context, slug string) (Listing, error) {
	const q = `SELECT id, tenant_id, package_id, name, slug, description, type, category,
	             status, avg_rating, review_count, created_at, updated_at
	           FROM marketplace_listing WHERE slug = $1 AND status != 'REMOVED'`
	row := r.db.QueryRow(ctx, q, slug)
	return scanRow(row)
}

// FindByTenant returns listings owned by the given tenant.
func (r *Repository) FindByTenant(ctx context.Context, tenantID string, req pagination.PageRequest) ([]Listing, int64, error) {
	const q = `SELECT id, tenant_id, package_id, name, slug, description, type, category,
	             status, avg_rating, review_count, created_at, updated_at
	           FROM marketplace_listing
	           WHERE tenant_id = $3 AND status != 'REMOVED'
	           ORDER BY name ASC LIMIT $1 OFFSET $2`
	const cq = `SELECT COUNT(*) FROM marketplace_listing WHERE tenant_id = $1 AND status != 'REMOVED'`
	rows, err := r.db.Query(ctx, q, req.Size, req.Offset(), tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	listings, err := scanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRow(ctx, cq, tenantID).Scan(&total); err != nil {
		return nil, 0, err
	}
	return listings, total, nil
}

// FindByID finds a listing by UUID.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (Listing, error) {
	const q = `SELECT id, tenant_id, package_id, name, slug, description, type, category,
	             status, avg_rating, review_count, created_at, updated_at
	           FROM marketplace_listing WHERE id = $1`
	row := r.db.QueryRow(ctx, q, id)
	return scanRow(row)
}

// Create inserts a new listing.
func (r *Repository) Create(ctx context.Context, l Listing) (Listing, error) {
	const q = `INSERT INTO marketplace_listing
	             (id, tenant_id, package_id, name, slug, description, type, category, status,
	              avg_rating, review_count, created_at, updated_at)
	           VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW(),NOW())
	           RETURNING id, tenant_id, package_id, name, slug, description, type, category,
	             status, avg_rating, review_count, created_at, updated_at`
	row := r.db.QueryRow(ctx, q,
		l.ID, l.TenantID, l.PackageID, l.Name, l.Slug, l.Description,
		string(l.Type), l.Category, string(l.Status), l.AvgRating, l.ReviewCount)
	return scanRow(row)
}

// Update updates a listing.
func (r *Repository) Update(ctx context.Context, l Listing) (Listing, error) {
	const q = `UPDATE marketplace_listing
	           SET name=$2, description=$3, category=$4, updated_at=NOW()
	           WHERE id=$1
	           RETURNING id, tenant_id, package_id, name, slug, description, type, category,
	             status, avg_rating, review_count, created_at, updated_at`
	row := r.db.QueryRow(ctx, q, l.ID, l.Name, l.Description, l.Category)
	return scanRow(row)
}

// SoftDelete marks a listing as REMOVED.
func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE marketplace_listing SET status='REMOVED', updated_at=NOW() WHERE id=$1`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateRatingStats updates the aggregated rating for a listing.
func (r *Repository) UpdateRatingStats(ctx context.Context, id uuid.UUID, avg float64, count int) error {
	const q = `UPDATE marketplace_listing SET avg_rating=$2, review_count=$3, updated_at=NOW() WHERE id=$1`
	_, err := r.db.Exec(ctx, q, id, avg, count)
	return err
}

// query is a helper for list queries without additional filters.
func (r *Repository) query(ctx context.Context, q, cq string, req pagination.PageRequest, _ interface{}) ([]Listing, int64, error) {
	rows, err := r.db.Query(ctx, q, req.Size, req.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	listings, err := scanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRow(ctx, cq).Scan(&total); err != nil {
		return nil, 0, err
	}
	return listings, total, nil
}
