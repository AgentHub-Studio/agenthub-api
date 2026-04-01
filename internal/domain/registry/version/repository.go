package version

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a version is not found.
var ErrNotFound = errors.New("version not found")

// Repository provides data access for package_version.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new version Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ListByPackage returns all versions for a package, ordered by published_at DESC.
func (r *Repository) ListByPackage(ctx context.Context, packageID uuid.UUID) ([]PackageVersion, error) {
	const query = `
		SELECT id, package_id, version, COALESCE(changelog,''), COALESCE(storage_path,''),
		       COALESCE(checksum,''), download_count, published_at, published_by
		  FROM public.package_version
		 WHERE package_id = $1
		 ORDER BY published_at DESC`

	rows, err := r.pool.Query(ctx, query, packageID)
	if err != nil {
		return nil, fmt.Errorf("version: list by package: %w", err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// GetByVersion returns a specific version of a package.
func (r *Repository) GetByVersion(ctx context.Context, packageID uuid.UUID, versionStr string) (PackageVersion, error) {
	const query = `
		SELECT id, package_id, version, COALESCE(changelog,''), COALESCE(storage_path,''),
		       COALESCE(checksum,''), download_count, published_at, published_by
		  FROM public.package_version
		 WHERE package_id = $1 AND version = $2`

	row := r.pool.QueryRow(ctx, query, packageID, versionStr)
	return scanRow(row)
}

// Create inserts a new version record.
func (r *Repository) Create(ctx context.Context, v PackageVersion) (PackageVersion, error) {
	const query = `
		INSERT INTO public.package_version (package_id, version, changelog, storage_path, checksum, published_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, package_id, version, COALESCE(changelog,''), COALESCE(storage_path,''),
		          COALESCE(checksum,''), download_count, published_at, published_by`

	row := r.pool.QueryRow(ctx, query,
		v.PackageID, v.Version, v.Changelog, v.StoragePath, v.Checksum, v.PublishedBy,
	)
	created, err := scanRow(row)
	if err != nil {
		return PackageVersion{}, fmt.Errorf("version: create: %w", err)
	}
	return created, nil
}

// Delete removes a specific version.
func (r *Repository) Delete(ctx context.Context, packageID uuid.UUID, versionStr string) error {
	const query = `DELETE FROM public.package_version WHERE package_id = $1 AND version = $2`
	tag, err := r.pool.Exec(ctx, query, packageID, versionStr)
	if err != nil {
		return fmt.Errorf("version: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRow(row pgx.Row) (PackageVersion, error) {
	var v PackageVersion
	err := row.Scan(
		&v.ID, &v.PackageID, &v.Version, &v.Changelog,
		&v.StoragePath, &v.Checksum, &v.DownloadCount,
		&v.PublishedAt, &v.PublishedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PackageVersion{}, ErrNotFound
	}
	if err != nil {
		return PackageVersion{}, fmt.Errorf("version: scan: %w", err)
	}
	return v, nil
}

func scanRows(rows pgx.Rows) ([]PackageVersion, error) {
	var versions []PackageVersion
	for rows.Next() {
		var v PackageVersion
		if err := rows.Scan(
			&v.ID, &v.PackageID, &v.Version, &v.Changelog,
			&v.StoragePath, &v.Checksum, &v.DownloadCount,
			&v.PublishedAt, &v.PublishedBy,
		); err != nil {
			return nil, fmt.Errorf("version: scan row: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}
