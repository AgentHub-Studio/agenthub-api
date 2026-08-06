package integration

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

type GeneratedSkill struct {
	ID          uuid.UUID
	Name        string
	Description string
	Category    string
}

type managementRepository interface {
	ListSkillsByToolID(ctx context.Context, toolID uuid.UUID) ([]GeneratedSkill, error)
	UpdateSkillMetadata(ctx context.Context, skillID uuid.UUID, name, description, instructions string) error
	CountToolBindings(ctx context.Context, skillID uuid.UUID) (int, error)
	DeleteSkill(ctx context.Context, skillID uuid.UUID) error
}

type repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates the auxiliary repository used by generated integrations.
func NewRepository(pool *pgxpool.Pool) managementRepository {
	return &repository{pool: pool}
}

func (r *repository) ListSkillsByToolID(ctx context.Context, toolID uuid.UUID) ([]GeneratedSkill, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, err
	}
	defer release()

	rows, err := conn.Query(ctx,
		`SELECT s.id, s.name, s.description, s.category
		 FROM skill s
		 INNER JOIN skill_tool st ON st.skill_id = s.id
		 WHERE st.tool_id = $1
		 ORDER BY s.created_at`,
		toolID,
	)
	if err != nil {
		return nil, fmt.Errorf("integration repo: list skills by tool: %w", err)
	}
	defer rows.Close()

	var out []GeneratedSkill
	for rows.Next() {
		var item GeneratedSkill
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Category); err != nil {
			return nil, fmt.Errorf("integration repo: scan generated skill: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *repository) UpdateSkillMetadata(ctx context.Context, skillID uuid.UUID, name, description, instructions string) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx,
		`UPDATE skill SET name = $1, description = $2, instructions = $3, updated_at = NOW() WHERE id = $4`,
		name, description, instructions, skillID,
	)
	if err != nil {
		return fmt.Errorf("integration repo: update skill metadata: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("integration repo: generated skill not found")
	}
	return nil
}

func (r *repository) CountToolBindings(ctx context.Context, skillID uuid.UUID) (int, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return 0, err
	}
	defer release()

	var total int
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM skill_tool WHERE skill_id = $1`, skillID).Scan(&total); err != nil {
		return 0, fmt.Errorf("integration repo: count skill bindings: %w", err)
	}
	return total, nil
}

func (r *repository) DeleteSkill(ctx context.Context, skillID uuid.UUID) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx, `DELETE FROM skill WHERE id = $1`, skillID)
	if err != nil {
		return fmt.Errorf("integration repo: delete generated skill: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("integration repo: generated skill not found")
	}
	return nil
}
