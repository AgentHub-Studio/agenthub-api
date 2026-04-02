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

// MemoryRepository defines the persistence interface for AgentMemory.
type MemoryRepository interface {
	ListByAgent(ctx context.Context, agentID uuid.UUID, userID *string) ([]AgentMemory, error)
	Upsert(ctx context.Context, m AgentMemory) (AgentMemory, error)
	GetByKey(ctx context.Context, agentID uuid.UUID, userID *string, key string) (AgentMemory, error)
	DeleteByKey(ctx context.Context, agentID uuid.UUID, userID *string, key string) error
	ClearByAgent(ctx context.Context, agentID uuid.UUID) error
	// Recall returns the top-N most semantically similar memories via pgvector cosine distance.
	// It also updates last_accessed_at for each returned entry.
	Recall(ctx context.Context, agentID uuid.UUID, userID *string, embedding []float32, limit int) ([]AgentMemory, error)
}

// Repository provides data access for agent_memory.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new memory Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// columns shared by all SELECT statements.
const memoryColumns = `id, agent_id, user_id, key, value, embedding::text,
	last_accessed_at, expires_at, created_at, updated_at`

// ListByAgent returns all memory entries for the given agent, optionally filtered by userID.
func (r *Repository) ListByAgent(ctx context.Context, agentID uuid.UUID, userID *string) ([]AgentMemory, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	query := `
		SELECT ` + memoryColumns + `
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

	// Convert []float32 to a pgvector-compatible string "[x,y,...]" or NULL.
	var embeddingExpr interface{}
	if len(m.Embedding) > 0 {
		embeddingExpr = float32SliceToVector(m.Embedding)
	}

	query := `
		INSERT INTO agent_memory (agent_id, user_id, key, value, embedding, expires_at)
		VALUES ($1, $2, $3, $4, $5::vector, $6)
		ON CONFLICT (agent_id, user_id, key) DO UPDATE
		   SET value = EXCLUDED.value,
		       embedding = EXCLUDED.embedding,
		       expires_at = EXCLUDED.expires_at,
		       updated_at = NOW()
		RETURNING ` + memoryColumns

	row := conn.QueryRow(ctx, query, m.AgentID, m.UserID, m.Key, m.Value, embeddingExpr, m.ExpiresAt)
	return scanRow(row)
}

// Recall returns the top-N memories closest to the given embedding vector using pgvector cosine distance.
// Each recalled entry has its last_accessed_at updated as a side effect.
func (r *Repository) Recall(ctx context.Context, agentID uuid.UUID, userID *string, embedding []float32, limit int) ([]AgentMemory, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("memory: recall requires a non-empty embedding")
	}
	if limit <= 0 {
		limit = 5
	}

	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	vec := float32SliceToVector(embedding)

	// Select top-N by cosine distance and update last_accessed_at atomically.
	query := `
		WITH recalled AS (
			SELECT id
			  FROM agent_memory
			 WHERE agent_id = $1
			   AND ($2::VARCHAR IS NULL OR user_id = $2)
			   AND embedding IS NOT NULL
			 ORDER BY embedding <=> $3::vector
			 LIMIT $4
		)
		UPDATE agent_memory m
		   SET last_accessed_at = NOW()
		  FROM recalled
		 WHERE m.id = recalled.id
		RETURNING ` + memoryColumns

	rows, err := conn.Query(ctx, query, agentID, userID, vec, limit)
	if err != nil {
		return nil, fmt.Errorf("memory: recall: %w", err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// GetByKey returns a single memory entry by agentID, userID and key.
func (r *Repository) GetByKey(ctx context.Context, agentID uuid.UUID, userID *string, key string) (AgentMemory, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return AgentMemory{}, err
	}
	defer release()

	query := `
		SELECT ` + memoryColumns + `
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
	var embText *string
	var expiresAt *time.Time
	err := row.Scan(&m.ID, &m.AgentID, &m.UserID, &m.Key, &m.Value, &embText,
		&m.LastAccessedAt, &expiresAt, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentMemory{}, ErrNotFound
	}
	if err != nil {
		return AgentMemory{}, fmt.Errorf("memory: scan: %w", err)
	}
	m.ExpiresAt = expiresAt
	if embText != nil {
		m.Embedding = vectorTextToFloat32Slice(*embText)
	}
	return m, nil
}

func scanRows(rows pgx.Rows) ([]AgentMemory, error) {
	items := []AgentMemory{}
	for rows.Next() {
		var m AgentMemory
		var embText *string
		var expiresAt *time.Time
		if err := rows.Scan(&m.ID, &m.AgentID, &m.UserID, &m.Key, &m.Value, &embText,
			&m.LastAccessedAt, &expiresAt, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("memory: scan row: %w", err)
		}
		m.ExpiresAt = expiresAt
		if embText != nil {
			m.Embedding = vectorTextToFloat32Slice(*embText)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

// float32SliceToVector converts a []float32 to the pgvector string representation "[x,y,...]".
func float32SliceToVector(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	b := make([]byte, 0, 2+len(v)*8)
	b = append(b, '[')
	for i, f := range v {
		if i > 0 {
			b = append(b, ',')
		}
		b = fmt.Appendf(b, "%g", f)
	}
	b = append(b, ']')
	return string(b)
}

// vectorTextToFloat32Slice parses a pgvector text representation "[x,y,...]" into []float32.
func vectorTextToFloat32Slice(s string) []float32 {
	if len(s) < 2 {
		return nil
	}
	s = s[1 : len(s)-1] // strip brackets
	if s == "" {
		return nil
	}
	var result []float32
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			var f float32
			_, _ = fmt.Sscanf(s[start:i], "%g", &f)
			result = append(result, f)
			start = i + 1
		}
	}
	return result
}
