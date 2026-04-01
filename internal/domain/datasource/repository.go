package datasource

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

// Repository handles persistence for DataSource.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const scanFields = `id, name, type, host, port, database, db_user, db_password, vpn_resource_id, created_at, updated_at`

func scanRow(row pgx.Row, d *DataSource) error {
	return row.Scan(
		&d.ID, &d.Name, &d.Type, &d.Host, &d.Port, &d.Database,
		&d.DBUser, &d.DBPassword, &d.VpnResourceID, &d.CreatedAt, &d.UpdatedAt,
	)
}

// ListAll returns a paginated list of datasources for the given tenant.
func (r *Repository) ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]DataSource, int, error) {
	conn, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()

	var total int
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM data_source`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("datasource: count: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT `+scanFields+` FROM data_source ORDER BY name LIMIT $1 OFFSET $2`,
		pr.Size, pr.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("datasource: list: %w", err)
	}
	defer rows.Close()

	var items []DataSource
	for rows.Next() {
		var d DataSource
		if err := rows.Scan(
			&d.ID, &d.Name, &d.Type, &d.Host, &d.Port, &d.Database,
			&d.DBUser, &d.DBPassword, &d.VpnResourceID, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("datasource: scan: %w", err)
		}
		items = append(items, d)
	}
	return items, total, rows.Err()
}

// GetByID retrieves a datasource by ID.
func (r *Repository) GetByID(ctx context.Context, tenantID string, id uuid.UUID) (DataSource, error) {
	conn, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return DataSource{}, err
	}
	defer conn.Release()

	var d DataSource
	err = scanRow(
		conn.QueryRow(ctx, `SELECT `+scanFields+` FROM data_source WHERE id=$1`, id),
		&d,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return DataSource{}, ErrNotFound
	}
	return d, err
}

// Create inserts a new datasource.
func (r *Repository) Create(ctx context.Context, tenantID string, d DataSource) (DataSource, error) {
	conn, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return DataSource{}, err
	}
	defer conn.Release()

	var created DataSource
	err = scanRow(conn.QueryRow(ctx,
		`INSERT INTO data_source (name, type, host, port, database, db_user, db_password, vpn_resource_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING `+scanFields,
		d.Name, d.Type, d.Host, d.Port, d.Database, d.DBUser, d.DBPassword, d.VpnResourceID,
	), &created)
	return created, err
}

// Update updates an existing datasource.
func (r *Repository) Update(ctx context.Context, tenantID string, id uuid.UUID, d DataSource) (DataSource, error) {
	conn, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return DataSource{}, err
	}
	defer conn.Release()

	var updated DataSource
	err = scanRow(conn.QueryRow(ctx,
		`UPDATE data_source SET name=$1, type=$2, host=$3, port=$4, database=$5,
		   db_user=$6, db_password=$7, vpn_resource_id=$8, updated_at=NOW()
		 WHERE id=$9
		 RETURNING `+scanFields,
		d.Name, d.Type, d.Host, d.Port, d.Database, d.DBUser, d.DBPassword, d.VpnResourceID, id,
	), &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return DataSource{}, ErrNotFound
	}
	return updated, err
}

// Delete removes a datasource by ID.
func (r *Repository) Delete(ctx context.Context, tenantID string, id uuid.UUID) error {
	conn, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer conn.Release()

	ct, err := conn.Exec(ctx, `DELETE FROM data_source WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("datasource: delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
