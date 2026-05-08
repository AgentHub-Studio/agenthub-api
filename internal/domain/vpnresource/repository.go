package vpnresource

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Repository handles persistence for VpnResource.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ListAll returns a paginated list of VPN resources for the given tenant.
func (r *Repository) ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]VpnResource, int, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var total int
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM vpn_resource`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("vpnresource: count: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, name, description, enabled, ovpn_config_path, auth_file_path, secret_name, created_at, updated_at
		 FROM vpn_resource ORDER BY name LIMIT $1 OFFSET $2`,
		pr.Size, pr.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("vpnresource: list: %w", err)
	}
	defer rows.Close()

	items := []VpnResource{}
	for rows.Next() {
		var v VpnResource
		if err := rows.Scan(
			&v.ID, &v.Name, &v.Description, &v.Enabled, &v.OvpnConfigPath,
			&v.AuthFilePath, &v.SecretName, &v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("vpnresource: scan: %w", err)
		}
		items = append(items, v)
	}
	return items, total, rows.Err()
}

// GetByID retrieves a VPN resource by ID.
func (r *Repository) GetByID(ctx context.Context, tenantID string, id uuid.UUID) (VpnResource, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return VpnResource{}, err
	}
	defer release()

	var v VpnResource
	err = conn.QueryRow(ctx,
		`SELECT id, name, description, enabled, ovpn_config_path, auth_file_path, secret_name, created_at, updated_at
		 FROM vpn_resource WHERE id=$1`,
		id,
	).Scan(&v.ID, &v.Name, &v.Description, &v.Enabled, &v.OvpnConfigPath,
		&v.AuthFilePath, &v.SecretName, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return VpnResource{}, ErrNotFound
	}
	return v, err
}

// ExistsByName retorna true se já existir VpnResource com mesmo
// nome no tenant. Detecção de duplicatas antes do INSERT.
func (r *Repository) ExistsByName(ctx context.Context, tenantID, name string) (bool, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return false, err
	}
	defer release()
	var exists bool
	err = conn.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM vpn_resource WHERE name = $1)`,
		name,
	).Scan(&exists)
	return exists, err
}

// Create inserts a new VPN resource.
func (r *Repository) Create(ctx context.Context, tenantID string, v VpnResource) (VpnResource, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return VpnResource{}, err
	}
	defer release()

	var created VpnResource
	err = conn.QueryRow(ctx,
		`INSERT INTO vpn_resource (name, description, enabled, ovpn_config_path, auth_file_path, secret_name)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, name, description, enabled, ovpn_config_path, auth_file_path, secret_name, created_at, updated_at`,
		v.Name, v.Description, v.Enabled, v.OvpnConfigPath, v.AuthFilePath, v.SecretName,
	).Scan(&created.ID, &created.Name, &created.Description, &created.Enabled, &created.OvpnConfigPath,
		&created.AuthFilePath, &created.SecretName, &created.CreatedAt, &created.UpdatedAt)
	return created, err
}

// Update updates an existing VPN resource.
func (r *Repository) Update(ctx context.Context, tenantID string, id uuid.UUID, v VpnResource) (VpnResource, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return VpnResource{}, err
	}
	defer release()

	var updated VpnResource
	err = conn.QueryRow(ctx,
		`UPDATE vpn_resource SET name=$1, description=$2, enabled=$3, ovpn_config_path=$4,
		   auth_file_path=$5, secret_name=$6, updated_at=NOW()
		 WHERE id=$7
		 RETURNING id, name, description, enabled, ovpn_config_path, auth_file_path, secret_name, created_at, updated_at`,
		v.Name, v.Description, v.Enabled, v.OvpnConfigPath, v.AuthFilePath, v.SecretName, id,
	).Scan(&updated.ID, &updated.Name, &updated.Description, &updated.Enabled, &updated.OvpnConfigPath,
		&updated.AuthFilePath, &updated.SecretName, &updated.CreatedAt, &updated.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return VpnResource{}, ErrNotFound
	}
	return updated, err
}

// Delete removes a VPN resource by ID.
func (r *Repository) Delete(ctx context.Context, tenantID string, id uuid.UUID) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	ct, err := conn.Exec(ctx, `DELETE FROM vpn_resource WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("vpnresource: delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
