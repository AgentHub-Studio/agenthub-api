package device

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// Repository provides persistence for Device records.
type Repository interface {
	List(ctx context.Context, req pagination.PageRequest) (pagination.Page[Device], error)
	GetByID(ctx context.Context, id uuid.UUID) (Device, error)
	Create(ctx context.Context, d Device) (Device, error)
	Update(ctx context.Context, d Device) (Device, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// Heartbeat updates last_seen_at and status for a device.
	Heartbeat(ctx context.Context, id uuid.UUID, status DeviceStatus, now time.Time) error

	// ListByAgent returns all devices linked to an agent.
	ListByAgent(ctx context.Context, agentID uuid.UUID) ([]Device, error)
	// AttachToAgent creates an agent_device binding.
	AttachToAgent(ctx context.Context, agentID, deviceID uuid.UUID) error
	// DetachFromAgent removes an agent_device binding.
	DetachFromAgent(ctx context.Context, agentID, deviceID uuid.UUID) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by PostgreSQL.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) acquire(ctx context.Context) (*pgxpool.Conn, func(), error) {
	tenantID := tenant.FromContext(ctx)
	return database.AcquireWithTenant(ctx, r.pool, tenantID)
}

const listQuery = `
	SELECT id, name, type, description,
	       mcp_server_config_id, resource_uri,
	       capabilities, last_seen_at, status, metadata,
	       enabled, created_at, updated_at
	FROM device`

func (r *pgRepository) List(ctx context.Context, req pagination.PageRequest) (pagination.Page[Device], error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return pagination.Page[Device]{}, fmt.Errorf("device: acquire: %w", err)
	}
	defer release()
	rows, err := conn.Query(ctx, listQuery+`
		ORDER BY name ASC LIMIT $1 OFFSET $2`,
		req.Size, req.Offset(),
	)
	if err != nil {
		return pagination.Page[Device]{}, fmt.Errorf("device: list: %w", err)
	}
	defer rows.Close()

	var items []Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return pagination.Page[Device]{}, fmt.Errorf("device: list scan: %w", err)
		}
		items = append(items, d)
	}

	var total int64
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM device`).Scan(&total); err != nil {
		return pagination.Page[Device]{}, fmt.Errorf("device: list count: %w", err)
	}
	return pagination.NewPage(items, total, req), nil
}

func (r *pgRepository) GetByID(ctx context.Context, id uuid.UUID) (Device, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return Device{}, fmt.Errorf("device: acquire: %w", err)
	}
	defer release()
	row := conn.QueryRow(ctx, listQuery+` WHERE id = $1`, id)
	d, err := scanDevice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Device{}, ErrNotFound
		}
		return Device{}, fmt.Errorf("device: get: %w", err)
	}
	return d, nil
}

func (r *pgRepository) Create(ctx context.Context, d Device) (Device, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return Device{}, fmt.Errorf("device: acquire: %w", err)
	}
	defer release()
	d.ID = uuid.New()
	caps, _ := capabilitiesJSON(d.Capabilities)
	meta := d.Metadata
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	err = conn.QueryRow(ctx, `
		INSERT INTO device
		  (id, name, type, description, mcp_server_config_id, resource_uri,
		   capabilities, status, metadata, enabled, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9::jsonb,$10,NOW(),NOW())
		RETURNING id, name, type, description,
		          mcp_server_config_id, resource_uri,
		          capabilities, last_seen_at, status, metadata,
		          enabled, created_at, updated_at`,
		d.ID, d.Name, string(d.Type), d.Description,
		d.MCPServerConfigID, d.ResourceURI,
		caps, string(d.Status), meta, d.Enabled,
	).Scan(
		&d.ID, &d.Name, &d.Type, &d.Description,
		&d.MCPServerConfigID, &d.ResourceURI,
		&d.Capabilities, &d.LastSeenAt, &d.Status, &d.Metadata,
		&d.Enabled, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			return Device{}, ErrNameConflict
		}
		return Device{}, fmt.Errorf("device: create: %w", err)
	}
	return d, nil
}

func (r *pgRepository) Update(ctx context.Context, d Device) (Device, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return Device{}, fmt.Errorf("device: acquire: %w", err)
	}
	defer release()
	caps, _ := capabilitiesJSON(d.Capabilities)
	meta := d.Metadata
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	err = conn.QueryRow(ctx, `
		UPDATE device SET
		  name=$2, description=$3, resource_uri=$4,
		  capabilities=$5::jsonb, status=$6, metadata=$7::jsonb,
		  enabled=$8, updated_at=NOW()
		WHERE id=$1
		RETURNING id, name, type, description,
		          mcp_server_config_id, resource_uri,
		          capabilities, last_seen_at, status, metadata,
		          enabled, created_at, updated_at`,
		d.ID, d.Name, d.Description, d.ResourceURI,
		caps, string(d.Status), meta, d.Enabled,
	).Scan(
		&d.ID, &d.Name, &d.Type, &d.Description,
		&d.MCPServerConfigID, &d.ResourceURI,
		&d.Capabilities, &d.LastSeenAt, &d.Status, &d.Metadata,
		&d.Enabled, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Device{}, ErrNotFound
		}
		return Device{}, fmt.Errorf("device: update: %w", err)
	}
	return d, nil
}

func (r *pgRepository) Delete(ctx context.Context, id uuid.UUID) error {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return fmt.Errorf("device: acquire: %w", err)
	}
	defer release()
	tag, err := conn.Exec(ctx, `DELETE FROM device WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("device: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) Heartbeat(ctx context.Context, id uuid.UUID, status DeviceStatus, now time.Time) error {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return fmt.Errorf("device: acquire: %w", err)
	}
	defer release()
	tag, err := conn.Exec(ctx,
		`UPDATE device SET status=$2, last_seen_at=$3, updated_at=NOW() WHERE id=$1`,
		id, string(status), now,
	)
	if err != nil {
		return fmt.Errorf("device: heartbeat: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) ListByAgent(ctx context.Context, agentID uuid.UUID) ([]Device, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("device: acquire: %w", err)
	}
	defer release()
	rows, err := conn.Query(ctx, listQuery+`
		JOIN agent_device ad ON ad.device_id = device.id
		WHERE ad.agent_id = $1
		ORDER BY device.name ASC`, agentID)
	if err != nil {
		return nil, fmt.Errorf("device: list by agent: %w", err)
	}
	defer rows.Close()

	var items []Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, fmt.Errorf("device: list by agent scan: %w", err)
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (r *pgRepository) AttachToAgent(ctx context.Context, agentID, deviceID uuid.UUID) error {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return fmt.Errorf("device: acquire: %w", err)
	}
	defer release()
	_, err = conn.Exec(ctx,
		`INSERT INTO agent_device (agent_id, device_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		agentID, deviceID,
	)
	return err
}

func (r *pgRepository) DetachFromAgent(ctx context.Context, agentID, deviceID uuid.UUID) error {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return fmt.Errorf("device: acquire: %w", err)
	}
	defer release()
	_, err = conn.Exec(ctx,
		`DELETE FROM agent_device WHERE agent_id=$1 AND device_id=$2`,
		agentID, deviceID,
	)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDevice(row rowScanner) (Device, error) {
	var d Device
	err := row.Scan(
		&d.ID, &d.Name, &d.Type, &d.Description,
		&d.MCPServerConfigID, &d.ResourceURI,
		&d.Capabilities, &d.LastSeenAt, &d.Status, &d.Metadata,
		&d.Enabled, &d.CreatedAt, &d.UpdatedAt,
	)
	return d, err
}

func capabilitiesJSON(caps []string) (string, error) {
	if len(caps) == 0 {
		return "[]", nil
	}
	b := strings.Builder{}
	b.WriteString("[")
	for i, c := range caps {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"`)
		b.WriteString(strings.ReplaceAll(c, `"`, `\"`))
		b.WriteString(`"`)
	}
	b.WriteString("]")
	return b.String(), nil
}
