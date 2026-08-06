package acl

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// PostgresProvider stores grants in the current tenant schema.
type PostgresProvider struct {
	pool *pgxpool.Pool
}

// NewPostgresProvider creates a tenant-aware ACL provider.
func NewPostgresProvider(pool *pgxpool.Pool) *PostgresProvider {
	return &PostgresProvider{pool: pool}
}

func (p *PostgresProvider) acquire(ctx context.Context) (*pgxpool.Conn, func(), error) {
	return database.AcquireWithTenant(ctx, p.pool, tenant.FromContext(ctx))
}

func (p *PostgresProvider) CanAccess(ctx context.Context, subjectID, resourceType, resourceID, action string) (bool, error) {
	conn, release, err := p.acquire(ctx)
	if err != nil {
		return false, fmt.Errorf("acl.CanAccess: acquire: %w", err)
	}
	defer release()

	var ok bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM resource_grant
			WHERE subject_type = 'user'
			  AND subject_id = $1
			  AND resource_type = $2
			  AND (resource_id = $3 OR resource_id = '*')
			  AND ($4 = ANY(actions) OR '*' = ANY(actions))
		)`,
		subjectID, resourceType, resourceID, action,
	).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("acl.CanAccess: %w", err)
	}
	return ok, nil
}

func (p *PostgresProvider) AddGrant(ctx context.Context, g Grant) (Grant, error) {
	conn, release, err := p.acquire(ctx)
	if err != nil {
		return Grant{}, fmt.Errorf("acl.AddGrant: acquire: %w", err)
	}
	defer release()

	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	if g.SubjectType == "" {
		g.SubjectType = SubjectUser
	}
	row := conn.QueryRow(ctx, `
		INSERT INTO resource_grant (id, subject_type, subject_id, resource_type, resource_id, actions)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (subject_type, subject_id, resource_type, resource_id)
		DO UPDATE SET actions = EXCLUDED.actions, updated_at = NOW()
		RETURNING id, subject_type, subject_id, resource_type, resource_id, actions, created_at, updated_at`,
		g.ID, string(g.SubjectType), g.SubjectID, g.ResourceType, g.ResourceID, g.Actions,
	)
	return scanGrant(row, "acl.AddGrant")
}

func (p *PostgresProvider) RemoveGrant(ctx context.Context, grantID uuid.UUID) error {
	conn, release, err := p.acquire(ctx)
	if err != nil {
		return fmt.Errorf("acl.RemoveGrant: acquire: %w", err)
	}
	defer release()

	_, err = conn.Exec(ctx, `DELETE FROM resource_grant WHERE id = $1`, grantID)
	if err != nil {
		return fmt.Errorf("acl.RemoveGrant: %w", err)
	}
	return nil
}

func (p *PostgresProvider) ListGrants(ctx context.Context, filter GrantFilter) ([]Grant, error) {
	conn, release, err := p.acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acl.ListGrants: acquire: %w", err)
	}
	defer release()

	where := []string{"1=1"}
	args := []any{}
	if filter.SubjectID != "" {
		args = append(args, filter.SubjectID)
		where = append(where, fmt.Sprintf("subject_id = $%d", len(args)))
	}
	if filter.ResourceType != "" {
		args = append(args, filter.ResourceType)
		where = append(where, fmt.Sprintf("resource_type = $%d", len(args)))
	}
	if filter.ResourceID != "" {
		args = append(args, filter.ResourceID)
		where = append(where, fmt.Sprintf("resource_id = $%d", len(args)))
	}

	rows, err := conn.Query(ctx, `
		SELECT id, subject_type, subject_id, resource_type, resource_id, actions, created_at, updated_at
		FROM resource_grant
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("acl.ListGrants: %w", err)
	}
	defer rows.Close()

	grants := []Grant{}
	for rows.Next() {
		g, err := scanGrant(rows, "acl.ListGrants")
		if err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}

func scanGrant(row pgx.Row, op string) (Grant, error) {
	var g Grant
	var subjectType string
	if err := row.Scan(
		&g.ID,
		&subjectType,
		&g.SubjectID,
		&g.ResourceType,
		&g.ResourceID,
		&g.Actions,
		&g.CreatedAt,
		&g.UpdatedAt,
	); err != nil {
		return Grant{}, fmt.Errorf("%s: scan: %w", op, err)
	}
	g.SubjectType = SubjectType(subjectType)
	return g, nil
}
