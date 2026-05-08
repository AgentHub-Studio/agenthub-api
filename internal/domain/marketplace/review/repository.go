package review

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Repository handles persistence for marketplace reviews.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// FindByListing returns paginated reviews for a listing.
func (r *Repository) FindByListing(ctx context.Context, listingID uuid.UUID, req pagination.PageRequest) ([]Review, int64, error) {
	const q = `SELECT id, listing_id, tenant_id, rating, comment, created_at
	           FROM marketplace_rating WHERE listing_id = $1
	           ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	const cq = `SELECT COUNT(*) FROM marketplace_rating WHERE listing_id = $1`
	rows, err := r.db.Query(ctx, q, listingID, req.Size, req.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	reviews, err := scanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRow(ctx, cq, listingID).Scan(&total); err != nil {
		return nil, 0, err
	}
	return reviews, total, nil
}

// FindByID returns a review by UUID.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (Review, error) {
	const q = `SELECT id, listing_id, tenant_id, rating, comment, created_at
	           FROM marketplace_rating WHERE id = $1`
	row := r.db.QueryRow(ctx, q, id)
	return scanRow(row)
}

// Create inserts a new review.
func (r *Repository) Create(ctx context.Context, rev Review) (Review, error) {
	const q = `INSERT INTO marketplace_rating (id, listing_id, tenant_id, rating, comment, created_at)
	           VALUES ($1,$2,$3,$4,$5,NOW())
	           RETURNING id, listing_id, tenant_id, rating, comment, created_at`
	row := r.db.QueryRow(ctx, q, rev.ID, rev.ListingID, rev.TenantID, rev.Rating, rev.Comment)
	created, err := scanRow(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Review{}, ErrDuplicate
		}
		return Review{}, err
	}
	return created, nil
}

// Delete removes a review.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM marketplace_rating WHERE id = $1`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetRatingStats returns aggregated rating stats for a listing.
func (r *Repository) GetRatingStats(ctx context.Context, listingID uuid.UUID) (RatingStats, error) {
	const q = `SELECT COALESCE(AVG(rating), 0), COUNT(*) FROM marketplace_rating WHERE listing_id = $1`
	var stats RatingStats
	err := r.db.QueryRow(ctx, q, listingID).Scan(&stats.Avg, &stats.Count)
	return stats, err
}

func scanRow(row pgx.Row) (Review, error) {
	var rev Review
	err := row.Scan(&rev.ID, &rev.ListingID, &rev.TenantID, &rev.Rating, &rev.Comment, &rev.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Review{}, ErrNotFound
		}
		return Review{}, err
	}
	return rev, nil
}

func scanRows(rows pgx.Rows) ([]Review, error) {
	var reviews []Review
	for rows.Next() {
		var rev Review
		if err := rows.Scan(&rev.ID, &rev.ListingID, &rev.TenantID, &rev.Rating, &rev.Comment, &rev.CreatedAt); err != nil {
			return nil, fmt.Errorf("review: scan row: %w", err)
		}
		reviews = append(reviews, rev)
	}
	return reviews, rows.Err()
}
