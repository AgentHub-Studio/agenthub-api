package agentic

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// PermissionAuditRepository persists permission audit entries to the
// permission_audit_log table in the tenant schema.
type PermissionAuditRepository struct {
	pool *pgxpool.Pool
}

// NewPermissionAuditRepository creates a repository backed by the given connection pool.
func NewPermissionAuditRepository(pool *pgxpool.Pool) *PermissionAuditRepository {
	return &PermissionAuditRepository{pool: pool}
}

func (r *PermissionAuditRepository) acquire(ctx context.Context) (*pgxpool.Conn, func(), error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, nil, fmt.Errorf("permission audit: acquire tenant connection: %w", err)
	}
	return conn, release, nil
}

// LogDecision inserts one permission audit entry into the database.
// Failures are non-fatal — the caller may log and continue.
func (r *PermissionAuditRepository) LogDecision(ctx context.Context, entry PermissionAuditEntry) error {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	const q = `
		INSERT INTO permission_audit_log
			(session_id, run_id, tool_name, decision, matched_rule, input_snippet, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	snippet := truncateInput(entry.InputSnippet, permissionAuditInputMaxLen)

	var matchedRule *string
	if entry.MatchedRule != "" {
		matchedRule = &entry.MatchedRule
	}

	_, err = conn.Exec(ctx, q,
		entry.SessionID,
		entry.RunID,
		entry.ToolName,
		string(entry.Decision),
		matchedRule,
		snippet,
		createdAt,
	)
	if err != nil {
		return fmt.Errorf("permission audit log insert: %w", err)
	}
	return nil
}

// ListBySession returns the most recent audit entries for a session, newest first.
// Limit defaults to 100 when zero.
func (r *PermissionAuditRepository) ListBySession(ctx context.Context, sessionID uuid.UUID, limit int) ([]PermissionAuditEntry, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	if limit <= 0 {
		limit = 100
	}
	const q = `
		SELECT session_id, run_id, tool_name, decision, matched_rule, input_snippet, created_at
		FROM   permission_audit_log
		WHERE  session_id = $1
		ORDER  BY created_at DESC
		LIMIT  $2`

	rows, err := conn.Query(ctx, q, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("permission audit list: %w", err)
	}
	defer rows.Close()

	var entries []PermissionAuditEntry
	for rows.Next() {
		var e PermissionAuditEntry
		var decision string
		var matchedRule *string
		if err := rows.Scan(
			&e.SessionID, &e.RunID, &e.ToolName,
			&decision, &matchedRule, &e.InputSnippet, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("permission audit scan: %w", err)
		}
		e.Decision = PermissionAuditDecision(decision)
		if matchedRule != nil {
			e.MatchedRule = *matchedRule
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
