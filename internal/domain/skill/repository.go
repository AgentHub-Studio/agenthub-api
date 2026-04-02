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
		`SELECT id, name, slug, description, category, input_schema, output_schema, created_at, updated_at
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

	inSchema := s.InputSchema
	if len(inSchema) == 0 {
		inSchema = []byte("{}")
	}
	outSchema := s.OutputSchema
	if len(outSchema) == 0 {
		outSchema = []byte("{}")
	}

	row := conn.QueryRow(ctx,
		`INSERT INTO skill (name, slug, description, category, input_schema, output_schema)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, name, slug, description, category, input_schema, output_schema, created_at, updated_at`,
		s.Name, s.Slug, s.Description, s.Category, inSchema, outSchema,
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
		`SELECT id, name, slug, description, category, input_schema, output_schema, created_at, updated_at
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

	inSchema := []byte("{}")
	if len(req.InputSchema) > 0 {
		inSchema = req.InputSchema
	}
	outSchema := []byte("{}")
	if len(req.OutputSchema) > 0 {
		outSchema = req.OutputSchema
	}

	row := conn.QueryRow(ctx,
		`UPDATE skill SET name=$1, description=$2, category=$3, input_schema=$4, output_schema=$5, updated_at=NOW()
		 WHERE id=$6
		 RETURNING id, name, slug, description, category, input_schema, output_schema, created_at, updated_at`,
		req.Name, req.Description, req.Category, inSchema, outSchema, id,
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

// --- scan helpers ---

func scanSkill(row pgx.Row) (Skill, error) {
	var s Skill
	var in, out []byte
	if err := row.Scan(&s.ID, &s.Name, &s.Slug, &s.Description, &s.Category, &in, &out, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return Skill{}, fmt.Errorf("skill: scan: %w", err)
	}
	s.InputSchema = in
	s.OutputSchema = out
	return s, nil
}

func scanSkillFromRows(rows pgx.Rows) (Skill, error) {
	var s Skill
	var in, out []byte
	if err := rows.Scan(&s.ID, &s.Name, &s.Slug, &s.Description, &s.Category, &in, &out, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return Skill{}, fmt.Errorf("skill: scan: %w", err)
	}
	s.InputSchema = in
	s.OutputSchema = out
	return s, nil
}
