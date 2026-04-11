package skill

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// ErrNotFound is returned when a skill is not found.
var ErrNotFound = errors.New("skill: not found")

// SkillRepository defines the persistence interface for Skill.
type SkillRepository interface {
	List(ctx context.Context, category *string, req pagination.PageRequest) ([]Skill, int64, error)
	Create(ctx context.Context, s Skill) (Skill, error)
	GetByID(ctx context.Context, id uuid.UUID) (Skill, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Skill, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SlugExists(ctx context.Context, slug string) (bool, error)
	// ListByAgentID returns all skills linked to an agent via the agent_skill table.
	ListByAgentID(ctx context.Context, agentID uuid.UUID) ([]Skill, error)
	// ListByIDs returns the skills with the given IDs, preserving the order of the
	// IDs slice. Used to load a session's snapshotted skill bindings. P-C115-1.
	ListByIDs(ctx context.Context, ids []uuid.UUID) ([]Skill, error)
	// CountAgentBindings returns the number of agents that reference this skill via agent_skill.
	// Used by Service.Delete to block deletion of skills that are still in use.
	CountAgentBindings(ctx context.Context, skillID uuid.UUID) (int64, error)
}

// Repository handles persistence for skills.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// List returns a paginated list of skills for the current tenant, optionally filtered by category.
func (r *Repository) List(ctx context.Context, category *string, req pagination.PageRequest) ([]Skill, int64, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	// Normalize empty string to nil so the SQL null-check works correctly.
	if category != nil && *category == "" {
		category = nil
	}

	var total int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM skill WHERE ($1::text IS NULL OR category = $1)`,
		category,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("skill: count: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, name, slug, description, category, created_at, updated_at, instructions, allowed_tools, disable_model_invocation, context_mode, when_to_use, argument_hint
		 FROM skill
		 WHERE ($1::text IS NULL OR category = $1)
		 ORDER BY name
		 LIMIT $2 OFFSET $3`,
		category, req.Size, req.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("skill: list: %w", err)
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		s, err := scanSkillFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		skills = append(skills, s)
	}
	return skills, total, rows.Err()
}

// Create inserts a new skill.
func (r *Repository) Create(ctx context.Context, s Skill) (Skill, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return Skill{}, err
	}
	defer release()

	row := conn.QueryRow(ctx,
		`INSERT INTO skill (name, slug, description, category, instructions, allowed_tools, disable_model_invocation, context_mode, when_to_use, argument_hint)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, name, slug, description, category, created_at, updated_at, instructions, allowed_tools, disable_model_invocation, context_mode, when_to_use, argument_hint`,
		s.Name, s.Slug, s.Description, s.Category, s.Instructions, s.AllowedTools, s.DisableModelInvocation, s.ContextMode, s.WhenToUse, s.ArgumentHint,
	)
	return scanSkill(row)
}

// GetByID returns a skill by ID.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (Skill, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return Skill{}, err
	}
	defer release()

	row := conn.QueryRow(ctx,
		`SELECT id, name, slug, description, category, created_at, updated_at, instructions, allowed_tools, disable_model_invocation, context_mode, when_to_use, argument_hint
		 FROM skill WHERE id=$1`, id,
	)
	s, err := scanSkill(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Skill{}, ErrNotFound
		}
		return Skill{}, err
	}
	return s, nil
}

// Update modifies an existing skill.
func (r *Repository) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (Skill, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return Skill{}, err
	}
	defer release()

	row := conn.QueryRow(ctx,
		`UPDATE skill SET name=$1, description=$2, category=$3, instructions=$4, allowed_tools=$5, disable_model_invocation=$6, context_mode=$7, when_to_use=$8, argument_hint=$9, updated_at=NOW()
		 WHERE id=$10
		 RETURNING id, name, slug, description, category, created_at, updated_at, instructions, allowed_tools, disable_model_invocation, context_mode, when_to_use, argument_hint`,
		req.Name, req.Description, req.Category, req.Instructions, req.AllowedTools, req.DisableModelInvocation, req.ContextMode, req.WhenToUse, req.ArgumentHint, id,
	)
	s, err := scanSkill(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Skill{}, ErrNotFound
		}
		return Skill{}, err
	}
	return s, nil
}

// Delete removes a skill by ID.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx, `DELETE FROM skill WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("skill: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SlugExists checks whether a slug is already in use.
func (r *Repository) SlugExists(ctx context.Context, slug string) (bool, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return false, err
	}
	defer release()

	var exists bool
	err = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM skill WHERE slug=$1)`, slug).Scan(&exists)
	return exists, err
}

// CountAgentBindings returns how many agents reference the given skill via agent_skill.
func (r *Repository) CountAgentBindings(ctx context.Context, skillID uuid.UUID) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return 0, err
	}
	defer release()

	var count int64
	err = conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_skill WHERE skill_id = $1`,
		skillID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("skill: count agent bindings: %w", err)
	}
	return count, nil
}

// --- scan helpers ---

func scanSkill(row pgx.Row) (Skill, error) {
	var s Skill
	var instructions *string
	if err := row.Scan(&s.ID, &s.Name, &s.Slug, &s.Description, &s.Category, &s.CreatedAt, &s.UpdatedAt, &instructions, &s.AllowedTools, &s.DisableModelInvocation, &s.ContextMode, &s.WhenToUse, &s.ArgumentHint); err != nil {
		return Skill{}, fmt.Errorf("skill: scan: %w", err)
	}
	if instructions != nil {
		s.Instructions = *instructions
	}
	return s, nil
}

func scanSkillFromRows(rows pgx.Rows) (Skill, error) {
	var s Skill
	var instructions *string
	if err := rows.Scan(&s.ID, &s.Name, &s.Slug, &s.Description, &s.Category, &s.CreatedAt, &s.UpdatedAt, &instructions, &s.AllowedTools, &s.DisableModelInvocation, &s.ContextMode, &s.WhenToUse, &s.ArgumentHint); err != nil {
		return Skill{}, fmt.Errorf("skill: scan: %w", err)
	}
	if instructions != nil {
		s.Instructions = *instructions
	}
	return s, nil
}

// ListByAgentID returns all skills linked to the given agent via the agent_skill join table.
func (r *Repository) ListByAgentID(ctx context.Context, agentID uuid.UUID) ([]Skill, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	rows, err := conn.Query(ctx,
		// P-C340-1 (ACT-F3-18): order by binding priority ASC then name for deterministic
		// skill ordering in the system prompt. Skills with lower priority values appear first.
		`SELECT s.id, s.name, s.slug, s.description, s.category, s.created_at, s.updated_at, s.instructions, s.allowed_tools, s.disable_model_invocation, s.context_mode, s.when_to_use, s.argument_hint
		 FROM skill s
		 INNER JOIN agent_skill ags ON ags.skill_id = s.id
		 WHERE ags.agent_id = $1
		 ORDER BY ags.priority ASC, s.name ASC`,
		agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("skill: list by agent: %w", err)
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		s, err := scanSkillFromRows(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, s)
	}
	if skills == nil {
		skills = []Skill{}
	}
	return skills, rows.Err()
}

// ListByIDs returns the skills with the given IDs.
// P-C115-1: used to load skills from a session's snapshotted bindings.
func (r *Repository) ListByIDs(ctx context.Context, ids []uuid.UUID) ([]Skill, error) {
	if len(ids) == 0 {
		return []Skill{}, nil
	}
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	rows, err := conn.Query(ctx,
		`SELECT id, name, slug, description, category, created_at, updated_at,
		        instructions, allowed_tools, disable_model_invocation, context_mode, when_to_use, argument_hint
		 FROM skill
		 WHERE id = ANY($1)
		 ORDER BY name`,
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("skill: list by ids: %w", err)
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		s, err := scanSkillFromRows(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, s)
	}
	if skills == nil {
		skills = []Skill{}
	}
	return skills, rows.Err()
}
