package installation

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Repository handles persistence for marketplace installations.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// columns shared by SELECT statements.
const installColumns = `id, tenant_id, package_id, package_version, status, hydrated, installed_at`

// FindByTenant returns paginated installations for a tenant.
func (r *Repository) FindByTenant(ctx context.Context, tenantID string, req pagination.PageRequest) ([]Installation, int64, error) {
	q := `SELECT ` + installColumns + `
	           FROM tenant_package_installation
	           WHERE tenant_id = $1 ORDER BY installed_at DESC LIMIT $2 OFFSET $3`
	const cq = `SELECT COUNT(*) FROM tenant_package_installation WHERE tenant_id = $1`
	rows, err := r.db.Query(ctx, q, tenantID, req.Size, req.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	installations, err := scanRows(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRow(ctx, cq, tenantID).Scan(&total); err != nil {
		return nil, 0, err
	}
	return installations, total, nil
}

// FindByID returns an installation by UUID.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (Installation, error) {
	q := `SELECT ` + installColumns + ` FROM tenant_package_installation WHERE id = $1`
	row := r.db.QueryRow(ctx, q, id)
	return scanRow(row)
}

// Create inserts a new installation record.
func (r *Repository) Create(ctx context.Context, i Installation) (Installation, error) {
	q := `INSERT INTO tenant_package_installation
	             (id, tenant_id, package_id, package_version, status, hydrated, installed_at)
	           VALUES ($1,$2,$3,$4,$5,false,NOW())
	           RETURNING ` + installColumns
	row := r.db.QueryRow(ctx, q, i.ID, i.TenantID, i.PackageID, i.PackageVersion, string(i.Status))
	return scanRow(row)
}

// Uninstall marks an installation as UNINSTALLED.
func (r *Repository) Uninstall(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE tenant_package_installation SET status='UNINSTALLED' WHERE id=$1`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetHydrated marks an installation as fully hydrated (agent/skills provisioned).
func (r *Repository) SetHydrated(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE tenant_package_installation SET hydrated=true WHERE id=$1`
	tag, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("installation: set hydrated: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRow(row pgx.Row) (Installation, error) {
	var i Installation
	var status string
	err := row.Scan(&i.ID, &i.TenantID, &i.PackageID, &i.PackageVersion, &status, &i.Hydrated, &i.InstalledAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Installation{}, ErrNotFound
		}
		return Installation{}, err
	}
	i.Status = InstallStatus(status)
	return i, nil
}

func scanRows(rows pgx.Rows) ([]Installation, error) {
	var installations []Installation
	for rows.Next() {
		var i Installation
		var status string
		if err := rows.Scan(&i.ID, &i.TenantID, &i.PackageID, &i.PackageVersion, &status, &i.Hydrated, &i.InstalledAt); err != nil {
			return nil, fmt.Errorf("installation: scan row: %w", err)
		}
		i.Status = InstallStatus(status)
		installations = append(installations, i)
	}
	return installations, rows.Err()
}
