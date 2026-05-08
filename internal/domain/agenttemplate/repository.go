package agenttemplate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// Repository defines persistence operations for AgentTemplate.
type Repository interface {
	ListAll(ctx context.Context) ([]AgentTemplate, error)
	ListByCategory(ctx context.Context, category string) ([]AgentTemplate, error)
	GetBySlug(ctx context.Context, slug string) (AgentTemplate, error)
	Create(ctx context.Context, t AgentTemplate) (AgentTemplate, error)
}

type repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new AgentTemplate repository backed by the given pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

const selectCols = `id, name, slug, description, category, is_builtin, definition_json, created_at, updated_at`

func scan(row pgx.Row) (AgentTemplate, error) {
	var t AgentTemplate
	var defJSON []byte
	err := row.Scan(
		&t.ID, &t.Name, &t.Slug, &t.Description, &t.Category,
		&t.IsBuiltin, &defJSON, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return AgentTemplate{}, err
	}
	if len(defJSON) > 0 {
		t.DefinitionJSON = json.RawMessage(defJSON)
	}
	return t, nil
}

func (r *repository) ListAll(ctx context.Context) ([]AgentTemplate, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, err
	}
	defer release()
	q := `SELECT ` + selectCols + ` FROM agent_template ORDER BY is_builtin DESC, name ASC`
	rows, err := conn.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("agent_template list: %w", err)
	}
	defer rows.Close()
	return collectRows(rows)
}

func (r *repository) ListByCategory(ctx context.Context, category string) ([]AgentTemplate, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, err
	}
	defer release()
	q := `SELECT ` + selectCols + ` FROM agent_template WHERE category = $1 ORDER BY is_builtin DESC, name ASC`
	rows, err := conn.Query(ctx, q, category)
	if err != nil {
		return nil, fmt.Errorf("agent_template list by category: %w", err)
	}
	defer rows.Close()
	return collectRows(rows)
}

func (r *repository) GetBySlug(ctx context.Context, slug string) (AgentTemplate, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return AgentTemplate{}, err
	}
	defer release()
	q := `SELECT ` + selectCols + ` FROM agent_template WHERE slug = $1`
	row := conn.QueryRow(ctx, q, slug)
	t, err := scan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentTemplate{}, ErrNotFound
		}
		return AgentTemplate{}, fmt.Errorf("agent_template get by slug: %w", err)
	}
	return t, nil
}

func (r *repository) Create(ctx context.Context, t AgentTemplate) (AgentTemplate, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return AgentTemplate{}, err
	}
	defer release()
	q := `INSERT INTO agent_template (name, slug, description, category, is_builtin, definition_json)
		  VALUES ($1, $2, $3, $4, $5, $6)
		  RETURNING ` + selectCols

	defJSON := t.DefinitionJSON
	if len(defJSON) == 0 {
		defJSON = json.RawMessage(`{}`)
	}

	row := conn.QueryRow(ctx, q, t.Name, t.Slug, t.Description, t.Category, t.IsBuiltin, []byte(defJSON))
	created, err := scan(row)
	if err != nil {
		if isUniqueViolation(err) {
			return AgentTemplate{}, ErrSlugConflict
		}
		return AgentTemplate{}, fmt.Errorf("agent_template create: %w", err)
	}
	return created, nil
}

func collectRows(rows pgx.Rows) ([]AgentTemplate, error) {
	var out []AgentTemplate
	for rows.Next() {
		t, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("agent_template scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// isUniqueViolation detects PostgreSQL unique-constraint violations (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	type pge interface{ SQLState() string }
	var pe pge
	if errors.As(err, &pe) {
		return pe.SQLState() == "23505"
	}
	return false
}

// Ensure *repository satisfies Repository at compile time.
var _ Repository = (*repository)(nil)
