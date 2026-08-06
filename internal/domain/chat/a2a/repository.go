package a2a

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
)

type Repository interface {
	UpsertGrant(ctx context.Context, ownerTenant string, g Grant) (Grant, error)
	HasGrant(ctx context.Context, ownerTenant string, subjectTenant string, agentID uuid.UUID, action string) (bool, error)
	ConsumeRateLimit(ctx context.Context, targetTenant string, sourceTenant string, now time.Time, window time.Duration, limit int) (bool, int, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) acquire(ctx context.Context, tenantID string) (*pgxpool.Conn, func(), error) {
	return database.AcquireWithTenant(ctx, r.pool, tenantID)
}

func (r *pgRepository) UpsertGrant(ctx context.Context, ownerTenant string, g Grant) (Grant, error) {
	conn, release, err := r.acquire(ctx, ownerTenant)
	if err != nil {
		return Grant{}, fmt.Errorf("a2a.UpsertGrant: acquire: %w", err)
	}
	defer release()

	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	row := conn.QueryRow(ctx, `
		INSERT INTO a2a_grant (id, subject_tenant, agent_id, actions)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (subject_tenant, agent_id)
		DO UPDATE SET actions = EXCLUDED.actions, updated_at = NOW()
		RETURNING id, subject_tenant, agent_id, actions, created_at, updated_at`,
		g.ID, g.SubjectTenant, g.AgentID, g.Actions,
	)
	return scanGrant(row)
}

func (r *pgRepository) HasGrant(ctx context.Context, ownerTenant string, subjectTenant string, agentID uuid.UUID, action string) (bool, error) {
	conn, release, err := r.acquire(ctx, ownerTenant)
	if err != nil {
		return false, fmt.Errorf("a2a.HasGrant: acquire: %w", err)
	}
	defer release()

	var ok bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM a2a_grant
			WHERE subject_tenant = $1
			  AND agent_id = $2
			  AND ($3 = ANY(actions) OR '*' = ANY(actions))
		)`,
		subjectTenant, agentID, action,
	).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("a2a.HasGrant: %w", err)
	}
	return ok, nil
}

func (r *pgRepository) ConsumeRateLimit(ctx context.Context, targetTenant string, sourceTenant string, now time.Time, window time.Duration, limit int) (bool, int, error) {
	conn, release, err := r.acquire(ctx, targetTenant)
	if err != nil {
		return false, 0, fmt.Errorf("a2a.ConsumeRateLimit: acquire: %w", err)
	}
	defer release()

	windowStart := rateLimitWindowStart(now, window)
	var count int
	err = conn.QueryRow(ctx, `
		INSERT INTO a2a_rate_limit (source_tenant, window_start, request_count)
		VALUES ($1, $2, 1)
		ON CONFLICT (source_tenant, window_start)
		DO UPDATE SET
			request_count = a2a_rate_limit.request_count + 1,
			updated_at = NOW()
		RETURNING request_count`, pgx.QueryExecModeExec,
		sourceTenant, windowStart,
	).Scan(&count)
	if err != nil {
		return false, 0, fmt.Errorf("a2a.ConsumeRateLimit: %w", err)
	}
	return count <= normalizePairLimit(limit), count, nil
}

func rateLimitWindowStart(now time.Time, window time.Duration) time.Time {
	if window <= 0 {
		window = defaultPairLimitWindow
	}
	return now.UTC().Truncate(window)
}

func normalizePairLimit(limit int) int {
	if limit <= 0 {
		return defaultPairLimit
	}
	return limit
}

func scanGrant(row pgx.Row) (Grant, error) {
	var g Grant
	if err := row.Scan(
		&g.ID,
		&g.SubjectTenant,
		&g.AgentID,
		&g.Actions,
		&g.CreatedAt,
		&g.UpdatedAt,
	); err != nil {
		return Grant{}, fmt.Errorf("a2a.scanGrant: %w", err)
	}
	return g, nil
}
