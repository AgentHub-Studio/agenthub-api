package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// ErrNotFound is returned when the setting cannot be found.
var ErrNotFound = errors.New("settings: not found")

// ErrValidation is returned when the setting key/value fails validation.
// Maps to HTTP 422 Unprocessable Entity.
var ErrValidation = errors.New("settings: validation failed")

// ErrUpstream is returned when an external provider (OpenAI, Anthropic,
// Ollama, OpenRouter) fails to respond. Maps to HTTP 502 Bad Gateway.
var ErrUpstream = errors.New("settings: upstream provider error")

// Repository defines persistence operations for Setting.
type Repository interface {
	FindAll(ctx context.Context) ([]Setting, error)
	FindByKey(ctx context.Context, key string) (Setting, error)
	Upsert(ctx context.Context, s Setting) (Setting, error)
	Delete(ctx context.Context, key string) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL-backed settings Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func scanSetting(row pgx.Row) (Setting, error) {
	var s Setting
	var value []byte
	var description *string
	err := row.Scan(&s.Key, &value, &description, &s.UpdatedAt)
	if err != nil {
		return Setting{}, err
	}
	s.Value = value
	if description != nil {
		s.Description = *description
	}
	return s, nil
}

func (r *pgRepository) FindAll(ctx context.Context) ([]Setting, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, fmt.Errorf("settings.FindAll: acquire: %w", err)
	}
	defer release()

	const query = `SELECT key, value, description, updated_at FROM settings ORDER BY key`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("settings.FindAll: %w", err)
	}
	defer rows.Close()

	var settings []Setting
	for rows.Next() {
		s, err := scanSetting(rows)
		if err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	if settings == nil {
		settings = []Setting{}
	}
	return settings, rows.Err()
}

func (r *pgRepository) FindByKey(ctx context.Context, key string) (Setting, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return Setting{}, fmt.Errorf("settings.FindByKey: acquire: %w", err)
	}
	defer release()

	const query = `SELECT key, value, description, updated_at FROM settings WHERE key = $1`
	row := conn.QueryRow(ctx, query, key)
	s, err := scanSetting(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Setting{}, ErrNotFound
	}
	if err != nil {
		return Setting{}, fmt.Errorf("settings.FindByKey: %w", err)
	}
	return s, nil
}

func (r *pgRepository) Upsert(ctx context.Context, s Setting) (Setting, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return Setting{}, fmt.Errorf("settings.Upsert: acquire: %w", err)
	}
	defer release()

	const query = `
		INSERT INTO settings (key, value, description, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value,
		    description = COALESCE(EXCLUDED.description, settings.description),
		    updated_at = NOW()
		RETURNING key, value, description, updated_at`
	row := conn.QueryRow(ctx, query, s.Key, []byte(s.Value), s.Description)
	result, err := scanSetting(row)
	if err != nil {
		return Setting{}, fmt.Errorf("settings.Upsert: %w", err)
	}
	return result, nil
}

func (r *pgRepository) Delete(ctx context.Context, key string) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return fmt.Errorf("settings.Delete: acquire: %w", err)
	}
	defer release()

	const query = `DELETE FROM settings WHERE key = $1`
	tag, err := conn.Exec(ctx, query, key)
	if err != nil {
		return fmt.Errorf("settings.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
