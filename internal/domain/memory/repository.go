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

// ErrValidation marks semantic input validation errors safe to surface to the
// client. Bug 260: distinguishes user-facing validation from repository SQL
// errors that should never reach the response body.
var ErrValidation = errors.New("memory: validation failed")

// MemoryRepository defines the persistence interface for AgentMemory.
type MemoryRepository interface {
	ListByAgent(ctx context.Context, agentID uuid.UUID, userID *string) ([]AgentMemory, error)
	ListByAgentAndType(ctx context.Context, agentID uuid.UUID, userID *string, memoryType MemoryType) ([]AgentMemory, error)
	Upsert(ctx context.Context, m AgentMemory) (AgentMemory, error)
	GetByKey(ctx context.Context, agentID uuid.UUID, userID *string, key string) (AgentMemory, error)
	DeleteByKey(ctx context.Context, agentID uuid.UUID, userID *string, key string) error
	ClearByAgent(ctx context.Context, agentID uuid.UUID) error
	// Recall returns the top-N most semantically similar memories via pgvector cosine distance.
	// scope and executionID are optional — pass empty string / nil to skip filtering.
	// It also updates last_accessed_at for each returned entry.
	Recall(ctx context.Context, agentID uuid.UUID, userID *string, embedding []float32, limit int, scope string, executionID *uuid.UUID) ([]AgentMemory, error)
	// DistillExecutionMemories promotes all execution-scoped memories for the given execution
	// to workflow scope (clearing execution_id), enabling cross-run knowledge sharing.
	DistillExecutionMemories(ctx context.Context, agentID uuid.UUID, executionID uuid.UUID) error
	// SearchByText returns memories matching a text pattern in key or value.
	SearchByText(ctx context.Context, agentID uuid.UUID, query string, limit int) ([]AgentMemory, error)
	// CountByType returns memory counts grouped by memory_type for the given agent.
	CountByType(ctx context.Context, agentID uuid.UUID) (map[MemoryType]int, error)
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
const memoryColumns = `id, agent_id, user_id, key, value, memory_type, scope, execution_id,
	embedding::text, last_accessed_at, expires_at, created_at, updated_at`

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

	memType := string(m.MemoryType)
	if memType == "" {
		memType = string(MemoryTypeGeneral)
	}

	scope := string(m.Scope)
	if scope == "" {
		scope = string(MemoryScopeAgent)
	}

	query := `
		INSERT INTO agent_memory (agent_id, user_id, key, value, memory_type, scope, execution_id, embedding, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::vector, $9)
		ON CONFLICT (agent_id, user_id, key) DO UPDATE
		   SET value = EXCLUDED.value,
		       memory_type = EXCLUDED.memory_type,
		       scope = EXCLUDED.scope,
		       execution_id = EXCLUDED.execution_id,
		       embedding = EXCLUDED.embedding,
		       expires_at = EXCLUDED.expires_at,
		       updated_at = NOW()
		RETURNING ` + memoryColumns

	row := conn.QueryRow(ctx, query, m.AgentID, m.UserID, m.Key, m.Value, memType, scope, m.ExecutionID, embeddingExpr, m.ExpiresAt)
	return scanRow(row)
}

// Recall returns the top-N memories closest to the given embedding vector using pgvector cosine distance.
// Each recalled entry has its last_accessed_at updated as a side effect.
// scope and executionID are optional filters; pass empty string / nil to skip.
func (r *Repository) Recall(ctx context.Context, agentID uuid.UUID, userID *string, embedding []float32, limit int, scope string, executionID *uuid.UUID) ([]AgentMemory, error) {
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
	// Scope and execution_id filters are optional ($5, $6).
	// P-MEM-1: qualify all RETURNING columns with "m." to resolve the "id is
	// ambiguous" error — both m.id and recalled.id are visible in the UPDATE FROM
	// context; unqualified "id" causes SQLSTATE 42702.
	const recallColumns = `m.id, m.agent_id, m.user_id, m.key, m.value, m.memory_type, m.scope, m.execution_id,
		m.embedding::text, m.last_accessed_at, m.expires_at, m.created_at, m.updated_at`
	query := `
		WITH recalled AS (
			SELECT id
			  FROM agent_memory
			 WHERE agent_id = $1
			   AND ($2::VARCHAR IS NULL OR user_id = $2)
			   AND embedding IS NOT NULL
			   AND ($5::TEXT IS NULL OR scope = $5)
			   AND ($6::UUID IS NULL OR execution_id = $6)
			 ORDER BY embedding <=> $3::vector
			 LIMIT $4
		)
		UPDATE agent_memory m
		   SET last_accessed_at = NOW()
		  FROM recalled
		 WHERE m.id = recalled.id
		RETURNING ` + recallColumns

	var scopeArg interface{}
	if scope != "" {
		scopeArg = scope
	}
	rows, err := conn.Query(ctx, query, agentID, userID, vec, limit, scopeArg, executionID)
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

// DeleteExpired removes memory entries whose ExpiresAt is in the past for the
// current tenant. Returns the number of rows deleted.
//
// Called by the background Pruner; safe to call manually as well. Rows without
// an ExpiresAt (NULL) are never touched.
func (r *Repository) DeleteExpired(ctx context.Context) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return 0, err
	}
	defer release()

	tag, err := conn.Exec(ctx, `DELETE FROM agent_memory WHERE expires_at IS NOT NULL AND expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("memory: delete expired: %w", err)
	}
	return tag.RowsAffected(), nil
}

// PruneStaleGeneral removes "general" memories whose LastAccessedAt is older
// than the cutoff. Structured types (user, feedback, project, reference) are
// preserved indefinitely — they hold curated, high-value context.
//
// Returns the number of rows deleted.
func (r *Repository) PruneStaleGeneral(ctx context.Context, cutoff time.Time) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return 0, err
	}
	defer release()

	tag, err := conn.Exec(ctx,
		`DELETE FROM agent_memory WHERE memory_type = 'general' AND last_accessed_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("memory: prune stale general: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ListByAgentAndType returns memories filtered by type.
func (r *Repository) ListByAgentAndType(ctx context.Context, agentID uuid.UUID, userID *string, memoryType MemoryType) ([]AgentMemory, error) {
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
		   AND memory_type = $3
		 ORDER BY key`

	rows, err := conn.Query(ctx, query, agentID, userID, string(memoryType))
	if err != nil {
		return nil, fmt.Errorf("memory: list by type: %w", err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// DistillExecutionMemories promotes all execution-scoped memories for the given execution
// to workflow scope by updating scope='workflow' and clearing execution_id.
func (r *Repository) DistillExecutionMemories(ctx context.Context, agentID uuid.UUID, executionID uuid.UUID) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	_, err = conn.Exec(ctx, `
		UPDATE agent_memory
		   SET scope        = 'workflow',
		       execution_id = NULL,
		       updated_at   = NOW()
		 WHERE agent_id     = $1
		   AND execution_id = $2
		   AND scope        = 'execution'`,
		agentID, executionID)
	if err != nil {
		return fmt.Errorf("memory: distill execution: %w", err)
	}
	return nil
}

// SearchByText returns memories matching a text pattern in key or value.
func (r *Repository) SearchByText(ctx context.Context, agentID uuid.UUID, query string, limit int) ([]AgentMemory, error) {
	if limit <= 0 {
		limit = 20
	}

	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	sql := `
		SELECT ` + memoryColumns + `
		  FROM agent_memory
		 WHERE agent_id = $1
		   AND (key ILIKE '%' || $2 || '%' OR value::text ILIKE '%' || $2 || '%')
		 ORDER BY updated_at DESC
		 LIMIT $3`

	rows, err := conn.Query(ctx, sql, agentID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("memory: search: %w", err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// CountByType returns memory counts grouped by memory_type.
func (r *Repository) CountByType(ctx context.Context, agentID uuid.UUID) (map[MemoryType]int, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	sql := `SELECT memory_type, COUNT(*) FROM agent_memory WHERE agent_id = $1 GROUP BY memory_type`
	rows, err := conn.Query(ctx, sql, agentID)
	if err != nil {
		return nil, fmt.Errorf("memory: count by type: %w", err)
	}
	defer rows.Close()

	counts := make(map[MemoryType]int)
	for rows.Next() {
		var mt string
		var count int
		if err := rows.Scan(&mt, &count); err != nil {
			return nil, fmt.Errorf("memory: count scan: %w", err)
		}
		counts[MemoryType(mt)] = count
	}
	return counts, rows.Err()
}

func scanRow(row pgx.Row) (AgentMemory, error) {
	var m AgentMemory
	var embText *string
	var expiresAt *time.Time
	var memType, scope string
	err := row.Scan(&m.ID, &m.AgentID, &m.UserID, &m.Key, &m.Value, &memType, &scope, &m.ExecutionID,
		&embText, &m.LastAccessedAt, &expiresAt, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentMemory{}, ErrNotFound
	}
	if err != nil {
		return AgentMemory{}, fmt.Errorf("memory: scan: %w", err)
	}
	m.MemoryType = MemoryType(memType)
	m.Scope = MemoryScope(scope)
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
		var memType, scope string
		if err := rows.Scan(&m.ID, &m.AgentID, &m.UserID, &m.Key, &m.Value, &memType, &scope, &m.ExecutionID,
			&embText, &m.LastAccessedAt, &expiresAt, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("memory: scan row: %w", err)
		}
		m.MemoryType = MemoryType(memType)
		m.Scope = MemoryScope(scope)
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
