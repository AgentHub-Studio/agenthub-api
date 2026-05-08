package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines persistence operations for Channel.
type Repository interface {
	List(ctx context.Context) ([]Channel, error)
	GetByID(ctx context.Context, id uuid.UUID) (Channel, error)
	GetByToken(ctx context.Context, token string) (Channel, error)
	Create(ctx context.Context, ch Channel) (Channel, error)
	Update(ctx context.Context, ch Channel) (Channel, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

const selectChannelCols = `id, name, type, agent_id, config, token, enabled, created_at, updated_at`

type repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Channel repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

func (r *repository) acquire(ctx context.Context) (*pgxpool.Conn, func(), error) {
	tenantID := tenant.FromContext(ctx)
	return database.AcquireWithTenant(ctx, r.pool, tenantID)
}

func scanChannel(row pgx.Row) (Channel, error) {
	var ch Channel
	var configBytes []byte
	err := row.Scan(
		&ch.ID, &ch.Name, &ch.Type, &ch.AgentID,
		&configBytes, &ch.Token, &ch.Enabled, &ch.CreatedAt, &ch.UpdatedAt,
	)
	if err != nil {
		return Channel{}, err
	}
	if len(configBytes) > 0 {
		ch.Config = json.RawMessage(configBytes)
	}
	return ch, nil
}

func (r *repository) List(ctx context.Context) ([]Channel, error) {
	q := `SELECT ` + selectChannelCols + ` FROM channel ORDER BY name ASC`
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("channel acquire: %w", err)
	}
	defer release()
	rows, err := conn.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("channel list: %w", err)
	}
	defer rows.Close()

	var out []Channel
	for rows.Next() {
		ch, err := scanChannel(rows)
		if err != nil {
			return nil, fmt.Errorf("channel scan: %w", err)
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (Channel, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return Channel{}, fmt.Errorf("channel acquire: %w", err)
	}
	defer release()
	q := `SELECT ` + selectChannelCols + ` FROM channel WHERE id = $1`
	row := conn.QueryRow(ctx, q, id)
	ch, err := scanChannel(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Channel{}, ErrNotFound
		}
		return Channel{}, fmt.Errorf("channel get by id: %w", err)
	}
	return ch, nil
}

func (r *repository) GetByToken(ctx context.Context, token string) (Channel, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return Channel{}, fmt.Errorf("channel acquire: %w", err)
	}
	defer release()
	q := `SELECT ` + selectChannelCols + ` FROM channel WHERE token = $1`
	row := conn.QueryRow(ctx, q, token)
	ch, err := scanChannel(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Channel{}, ErrTokenNotFound
		}
		return Channel{}, fmt.Errorf("channel get by token: %w", err)
	}
	return ch, nil
}

func (r *repository) Create(ctx context.Context, ch Channel) (Channel, error) {
	q := `INSERT INTO channel (name, type, agent_id, config, token, enabled)
		  VALUES ($1, $2, $3, $4, $5, $6)
		  RETURNING ` + selectChannelCols

	cfgBytes := ch.Config
	if len(cfgBytes) == 0 {
		cfgBytes = json.RawMessage(`{}`)
	}

	conn, release, err := r.acquire(ctx)
	if err != nil {
		return Channel{}, fmt.Errorf("channel acquire: %w", err)
	}
	defer release()
	row := conn.QueryRow(ctx, q,
		ch.Name, string(ch.Type), ch.AgentID, []byte(cfgBytes), ch.Token, ch.Enabled,
	)
	created, scanErr := scanChannel(row)
	if scanErr != nil {
		if isUniqueViolation(scanErr) {
			return Channel{}, ErrSlugConflict
		}
		return Channel{}, fmt.Errorf("channel create: %w", scanErr)
	}
	return created, nil
}

func (r *repository) Update(ctx context.Context, ch Channel) (Channel, error) {
	q := `UPDATE channel
		  SET name = $1, agent_id = $2, config = $3, enabled = $4, updated_at = now()
		  WHERE id = $5
		  RETURNING ` + selectChannelCols

	cfgBytes := ch.Config
	if len(cfgBytes) == 0 {
		cfgBytes = json.RawMessage(`{}`)
	}

	conn, release, err := r.acquire(ctx)
	if err != nil {
		return Channel{}, fmt.Errorf("channel acquire: %w", err)
	}
	defer release()
	row := conn.QueryRow(ctx, q, ch.Name, ch.AgentID, []byte(cfgBytes), ch.Enabled, ch.ID)
	updated, scanErr := scanChannel(row)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return Channel{}, ErrNotFound
		}
		return Channel{}, fmt.Errorf("channel update: %w", scanErr)
	}
	return updated, nil
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return fmt.Errorf("channel acquire: %w", err)
	}
	defer release()
	q := `DELETE FROM channel WHERE id = $1`
	tag, err := conn.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("channel delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// isUniqueViolation detects PostgreSQL unique-constraint violations (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	type pge interface{ SQLState() string }
	var pe pge
	if errors.As(err, &pe) {
		return pe.SQLState() == "23505"
	}
	return false
}

var _ Repository = (*repository)(nil)
