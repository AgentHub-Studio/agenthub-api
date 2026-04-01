package installation

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when an asset is not found.
var ErrNotFound = errors.New("asset not found")

// Repository provides data access for package_asset.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new installation Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ListByPackage returns all assets for a package, optionally filtered by version.
func (r *Repository) ListByPackage(ctx context.Context, packageID uuid.UUID, versionID *uuid.UUID) ([]PackageAsset, error) {
	var query string
	var args []any

	if versionID != nil {
		query = `
			SELECT id, package_id, version_id, filename, content_type, storage_path, size_bytes, COALESCE(checksum,''), created_at
			  FROM public.package_asset
			 WHERE package_id = $1 AND version_id = $2
			 ORDER BY created_at DESC`
		args = []any{packageID, *versionID}
	} else {
		query = `
			SELECT id, package_id, version_id, filename, content_type, storage_path, size_bytes, COALESCE(checksum,''), created_at
			  FROM public.package_asset
			 WHERE package_id = $1
			 ORDER BY created_at DESC`
		args = []any{packageID}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("asset: list by package: %w", err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// GetByID returns a single asset.
func (r *Repository) GetByID(ctx context.Context, assetID uuid.UUID) (PackageAsset, error) {
	const query = `
		SELECT id, package_id, version_id, filename, content_type, storage_path, size_bytes, COALESCE(checksum,''), created_at
		  FROM public.package_asset
		 WHERE id = $1`

	row := r.pool.QueryRow(ctx, query, assetID)
	return scanRow(row)
}

// Create inserts a new asset record.
func (r *Repository) Create(ctx context.Context, a PackageAsset) (PackageAsset, error) {
	const query = `
		INSERT INTO public.package_asset (package_id, version_id, filename, content_type, storage_path, size_bytes, checksum)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, package_id, version_id, filename, content_type, storage_path, size_bytes, COALESCE(checksum,''), created_at`

	row := r.pool.QueryRow(ctx, query,
		a.PackageID, a.VersionID, a.Filename, a.ContentType, a.StoragePath, a.SizeBytes, a.Checksum,
	)
	created, err := scanRow(row)
	if err != nil {
		return PackageAsset{}, fmt.Errorf("asset: create: %w", err)
	}
	return created, nil
}

func scanRow(row pgx.Row) (PackageAsset, error) {
	var a PackageAsset
	err := row.Scan(
		&a.ID, &a.PackageID, &a.VersionID, &a.Filename,
		&a.ContentType, &a.StoragePath, &a.SizeBytes, &a.Checksum, &a.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PackageAsset{}, ErrNotFound
	}
	if err != nil {
		return PackageAsset{}, fmt.Errorf("asset: scan: %w", err)
	}
	return a, nil
}

func scanRows(rows pgx.Rows) ([]PackageAsset, error) {
	var assets []PackageAsset
	for rows.Next() {
		var a PackageAsset
		if err := rows.Scan(
			&a.ID, &a.PackageID, &a.VersionID, &a.Filename,
			&a.ContentType, &a.StoragePath, &a.SizeBytes, &a.Checksum, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("asset: scan row: %w", err)
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}
