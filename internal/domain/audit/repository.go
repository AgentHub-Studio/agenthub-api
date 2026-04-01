package audit

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

// Repository handles persistence for AuditLog entries.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ListAll returns a paginated, filtered list of audit logs for the given tenant.
func (r *Repository) ListAll(ctx context.Context, tenantID string, f ListFilter, pr pagination.PageRequest) ([]AuditLog, int, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	args := []any{}
	where := "WHERE 1=1"
	idx := 1

	if f.EntityType != "" {
		where += fmt.Sprintf(" AND entity_type = $%d", idx)
		args = append(args, f.EntityType)
		idx++
	}
	if f.EntityID != "" {
		where += fmt.Sprintf(" AND entity_id = $%d", idx)
		args = append(args, f.EntityID)
		idx++
	}
	if f.Action != "" {
		where += fmt.Sprintf(" AND action = $%d", idx)
		args = append(args, f.Action)
		idx++
	}

	countArgs := make([]any, len(args))
	copy(countArgs, args)

	var total int
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit: count: %w", err)
	}

	args = append(args, pr.Size, pr.Offset())
	rows, err := conn.Query(ctx,
		`SELECT id, entity_type, entity_id, action, actor_id, actor_email,
		        old_value, new_value, metadata, ip_address, created_at
		 FROM audit_log `+where+fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, idx, idx+1),
		args...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("audit: list: %w", err)
	}
	defer rows.Close()

	var items []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(
			&l.ID, &l.EntityType, &l.EntityID, &l.Action, &l.ActorID, &l.ActorEmail,
			&l.OldValue, &l.NewValue, &l.Metadata, &l.IPAddress, &l.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("audit: scan: %w", err)
		}
		items = append(items, l)
	}
	return items, total, rows.Err()
}

// GetByID retrieves a single audit log entry by ID.
func (r *Repository) GetByID(ctx context.Context, tenantID string, id uuid.UUID) (AuditLog, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return AuditLog{}, err
	}
	defer release()

	var l AuditLog
	err = conn.QueryRow(ctx,
		`SELECT id, entity_type, entity_id, action, actor_id, actor_email,
		        old_value, new_value, metadata, ip_address, created_at
		 FROM audit_log WHERE id = $1`,
		id,
	).Scan(
		&l.ID, &l.EntityType, &l.EntityID, &l.Action, &l.ActorID, &l.ActorEmail,
		&l.OldValue, &l.NewValue, &l.Metadata, &l.IPAddress, &l.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AuditLog{}, ErrNotFound
	}
	return l, err
}

// Record appends a new audit log entry.
func (r *Repository) Record(ctx context.Context, tenantID string, l AuditLog) (AuditLog, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return AuditLog{}, err
	}
	defer release()

	var created AuditLog
	err = conn.QueryRow(ctx,
		`INSERT INTO audit_log
		 (entity_type, entity_id, action, actor_id, actor_email, old_value, new_value, metadata, ip_address)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, entity_type, entity_id, action, actor_id, actor_email,
		           old_value, new_value, metadata, ip_address, created_at`,
		l.EntityType, l.EntityID, l.Action, l.ActorID, l.ActorEmail,
		l.OldValue, l.NewValue, l.Metadata, l.IPAddress,
	).Scan(
		&created.ID, &created.EntityType, &created.EntityID, &created.Action,
		&created.ActorID, &created.ActorEmail, &created.OldValue, &created.NewValue,
		&created.Metadata, &created.IPAddress, &created.CreatedAt,
	)
	return created, err
}
