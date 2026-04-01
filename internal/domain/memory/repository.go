package memory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// ErrNotFound is returned when a memory entry is not found.
var ErrNotFound = errors.New("memory: not found")

// Repository provides data access for agent_memory.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new memory Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ListByAgent returns all memory entries for the given agent, optionally filtered by userID.
func (r *Repository) ListByAgent(ctx context.Context, agentID uuid.UUID, userID *string) ([]AgentMemory, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	const query = `
		SELECT id, agent_id, user_id, key, value, expires_at, created_at, updated_at
		  FROM agent_memory
		 WHERE agent_id = $1
		   AND ($2::VARCHAR IS NULL OR user_id = $2)
		 ORDER BY key`

	rows, err := conn.Query(ctx, query, agentID, userID)
	if err != nil {
		return nil, fmt.Errorf("memory: list by agent: %w", err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// Upsert creates or updates a memory entry identified by (agentID, userID, key).
func (r *Repository) Upsert(ctx context.Context, m AgentMemory) (AgentMemory, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return AgentMemory{}, err
	}
	defer release()

	const query = `
		INSERT INTO agent_memory (agent_id, user_id, key, value, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (agent_id, user_id, key) DO UPDATE
		   SET value = EXCLUDED.value,
		       expires_at = EXCLUDED.expires_at,
		       updated_at = NOW()
		RETURNING id, agent_id, user_id, key, value, expires_at, created_at, updated_at`

	row := conn.QueryRow(ctx, query, m.AgentID, m.UserID, m.Key, m.Value, m.ExpiresAt)
	return scanRow(row)
}

// GetByKey returns a single memory entry by agentID, userID and key.
func (r *Repository) GetByKey(ctx context.Context, agentID uuid.UUID, userID *string, key string) (AgentMemory, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return AgentMemory{}, err
	}
	defer release()

	const query = `
		SELECT id, agent_id, user_id, key, value, expires_at, created_at, updated_at
		  FROM agent_memory
		 WHERE agent_id = $1
		   AND ($2::VARCHAR IS NULL OR user_id = $2)
		   AND key = $3`

	row := conn.QueryRow(ctx, query, agentID, userID, key)
	return scanRow(row)
}

// DeleteByKey removes a memory entry by agentID, userID and key.
func (r *Repository) DeleteByKey(ctx context.Context, agentID uuid.UUID, userID *string, key string) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	const query = `DELETE FROM agent_memory WHERE agent_id = $1 AND ($2::VARCHAR IS NULL OR user_id = $2) AND key = $3`
	ct, err := conn.Exec(ctx, query, agentID, userID, key)
	if err != nil {
		return fmt.Errorf("memory: delete by key: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ClearByAgent removes all memory entries for the given agent.
func (r *Repository) ClearByAgent(ctx context.Context, agentID uuid.UUID) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	_, err = conn.Exec(ctx, `DELETE FROM agent_memory WHERE agent_id = $1`, agentID)
	if err != nil {
		return fmt.Errorf("memory: clear by agent: %w", err)
	}
	return nil
}

func scanRow(row pgx.Row) (AgentMemory, error) {
	var m AgentMemory
	var expiresAt *time.Time
	err := row.Scan(&m.ID, &m.AgentID, &m.UserID, &m.Key, &m.Value, &expiresAt, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentMemory{}, ErrNotFound
	}
	if err != nil {
		return AgentMemory{}, fmt.Errorf("memory: scan: %w", err)
	}
	m.ExpiresAt = expiresAt
	return m, nil
}

func scanRows(rows pgx.Rows) ([]AgentMemory, error) {
	var items []AgentMemory
	for rows.Next() {
		var m AgentMemory
		var expiresAt *time.Time
		if err := rows.Scan(&m.ID, &m.AgentID, &m.UserID, &m.Key, &m.Value, &expiresAt, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("memory: scan row: %w", err)
		}
		m.ExpiresAt = expiresAt
		items = append(items, m)
	}
	return items, rows.Err()
}
