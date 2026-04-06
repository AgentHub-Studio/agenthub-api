package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// Repository defines the persistence interface for McpServerConfig.
type Repository interface {
	List(ctx context.Context) ([]McpServerConfig, error)
	GetByID(ctx context.Context, id uuid.UUID) (McpServerConfig, error)
	GetByName(ctx context.Context, name string) (McpServerConfig, error)
	Create(ctx context.Context, c McpServerConfig) (McpServerConfig, error)
	Update(ctx context.Context, c McpServerConfig) (McpServerConfig, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListAutoStart(ctx context.Context) ([]McpServerConfig, error)
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new postgres-backed Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

const selectColumns = `id, name, transport_type, http_base_url, command, args, env,
	oauth_credential_id,
	auto_start, enabled, created_at, updated_at`

func scanConfig(row pgx.Row) (McpServerConfig, error) {
	var c McpServerConfig
	var argsJSON, envJSON []byte
	err := row.Scan(
		&c.ID, &c.Name, &c.TransportType, &c.HTTPBaseURL, &c.Command,
		&argsJSON, &envJSON,
		&c.OAuthCredentialID,
		&c.AutoStart, &c.Enabled, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return McpServerConfig{}, err
	}

	if argsJSON != nil {
		if err := json.Unmarshal(argsJSON, &c.Args); err != nil {
			return McpServerConfig{}, fmt.Errorf("mcp: unmarshal args: %w", err)
		}
	}
	if envJSON != nil {
		if err := json.Unmarshal(envJSON, &c.Env); err != nil {
			return McpServerConfig{}, fmt.Errorf("mcp: unmarshal env: %w", err)
		}
	}

	return c, nil
}

func (r *postgresRepository) List(ctx context.Context) ([]McpServerConfig, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, err
	}
	defer release()

	rows, err := conn.Query(ctx,
		`SELECT `+selectColumns+` FROM mcp_server_config ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("mcp: list: %w", err)
	}
	defer rows.Close()

	var items []McpServerConfig
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("mcp: scan: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mcp: rows: %w", err)
	}

	return items, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (McpServerConfig, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return McpServerConfig{}, err
	}
	defer release()

	row := conn.QueryRow(ctx,
		`SELECT `+selectColumns+` FROM mcp_server_config WHERE id = $1`,
		id,
	)
	c, err := scanConfig(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return McpServerConfig{}, ErrNotFound
		}
		return McpServerConfig{}, fmt.Errorf("mcp: get by id: %w", err)
	}

	return c, nil
}

func (r *postgresRepository) GetByName(ctx context.Context, name string) (McpServerConfig, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return McpServerConfig{}, err
	}
	defer release()

	row := conn.QueryRow(ctx,
		`SELECT `+selectColumns+` FROM mcp_server_config WHERE name = $1`,
		name,
	)
	c, err := scanConfig(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return McpServerConfig{}, ErrNotFound
		}
		return McpServerConfig{}, fmt.Errorf("mcp: get by name: %w", err)
	}

	return c, nil
}

func (r *postgresRepository) Create(ctx context.Context, c McpServerConfig) (McpServerConfig, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return McpServerConfig{}, err
	}
	defer release()

	now := time.Now().UTC()
	c.ID = uuid.New()
	c.CreatedAt = now
	c.UpdatedAt = now

	argsJSON, _ := json.Marshal(c.Args)
	envJSON, _ := json.Marshal(c.Env)

	_, err = conn.Exec(ctx,
		`INSERT INTO mcp_server_config
		 (id, name, transport_type, http_base_url, command, args, env,
		  oauth_credential_id,
		  auto_start, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		c.ID, c.Name, c.TransportType, c.HTTPBaseURL, c.Command,
		argsJSON, envJSON,
		c.OAuthCredentialID,
		c.AutoStart, &c.Enabled, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return McpServerConfig{}, fmt.Errorf("mcp: create: %w", err)
	}

	return c, nil
}

func (r *postgresRepository) Update(ctx context.Context, c McpServerConfig) (McpServerConfig, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return McpServerConfig{}, err
	}
	defer release()

	c.UpdatedAt = time.Now().UTC()
	argsJSON, _ := json.Marshal(c.Args)
	envJSON, _ := json.Marshal(c.Env)

	tag, err := conn.Exec(ctx,
		`UPDATE mcp_server_config
		 SET name=$1, transport_type=$2, http_base_url=$3, command=$4, args=$5, env=$6,
		     oauth_credential_id=$7,
		     auto_start=$8, enabled=$9, updated_at=$10
		 WHERE id=$11`,
		c.Name, c.TransportType, c.HTTPBaseURL, c.Command, argsJSON, envJSON,
		c.OAuthCredentialID,
		c.AutoStart, c.Enabled, c.UpdatedAt, c.ID,
	)
	if err != nil {
		return McpServerConfig{}, fmt.Errorf("mcp: update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return McpServerConfig{}, ErrNotFound
	}

	return c, nil
}

func (r *postgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx, `DELETE FROM mcp_server_config WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mcp: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *postgresRepository) ListAutoStart(ctx context.Context) ([]McpServerConfig, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, err
	}
	defer release()

	rows, err := conn.Query(ctx,
		`SELECT `+selectColumns+` FROM mcp_server_config WHERE auto_start = true AND enabled = true ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("mcp: list auto-start: %w", err)
	}
	defer rows.Close()

	var items []McpServerConfig
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("mcp: scan: %w", err)
		}
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mcp: rows: %w", err)
	}

	return items, nil
}
