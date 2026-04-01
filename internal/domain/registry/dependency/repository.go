package dependency

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a dependency is not found.
var ErrNotFound = errors.New("dependency not found")

// Repository provides data access for package_dependency.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new dependency Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ListByPackage returns all direct dependencies of a package.
func (r *Repository) ListByPackage(ctx context.Context, packageID uuid.UUID) ([]PackageDependency, error) {
	const query = `
		SELECT id, package_id, dependency_id, version_constraint, created_at
		  FROM public.package_dependency
		 WHERE package_id = $1
		 ORDER BY created_at`

	rows, err := r.pool.Query(ctx, query, packageID)
	if err != nil {
		return nil, fmt.Errorf("dependency: list by package: %w", err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// Create inserts a new dependency edge.
func (r *Repository) Create(ctx context.Context, d PackageDependency) (PackageDependency, error) {
	const query = `
		INSERT INTO public.package_dependency (package_id, dependency_id, version_constraint)
		VALUES ($1, $2, $3)
		RETURNING id, package_id, dependency_id, version_constraint, created_at`

	row := r.pool.QueryRow(ctx, query, d.PackageID, d.DependencyID, d.VersionConstraint)
	created, err := scanRow(row)
	if err != nil {
		return PackageDependency{}, fmt.Errorf("dependency: create: %w", err)
	}
	return created, nil
}

// Delete removes a dependency edge.
func (r *Repository) Delete(ctx context.Context, packageID, depID uuid.UUID) error {
	const query = `DELETE FROM public.package_dependency WHERE package_id = $1 AND id = $2`
	tag, err := r.pool.Exec(ctx, query, packageID, depID)
	if err != nil {
		return fmt.Errorf("dependency: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetPackageName fetches name and slug for a package ID (used during tree resolution).
func (r *Repository) GetPackageName(ctx context.Context, packageID uuid.UUID) (name, slug string, err error) {
	const query = `SELECT name, slug FROM public.package_registry WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, packageID)
	if scanErr := row.Scan(&name, &slug); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", fmt.Errorf("dependency: get package name: %w", scanErr)
	}
	return name, slug, nil
}

func scanRow(row pgx.Row) (PackageDependency, error) {
	var d PackageDependency
	err := row.Scan(&d.ID, &d.PackageID, &d.DependencyID, &d.VersionConstraint, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PackageDependency{}, ErrNotFound
	}
	if err != nil {
		return PackageDependency{}, fmt.Errorf("dependency: scan: %w", err)
	}
	return d, nil
}

func scanRows(rows pgx.Rows) ([]PackageDependency, error) {
	var deps []PackageDependency
	for rows.Next() {
		var d PackageDependency
		if err := rows.Scan(&d.ID, &d.PackageID, &d.DependencyID, &d.VersionConstraint, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("dependency: scan row: %w", err)
		}
		deps = append(deps, d)
	}
	return deps, rows.Err()
}
