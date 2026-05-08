package prompttemplate

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

const columns = `id, agent_id, name, slug, description, content, category, model_override, allowed_tools, created_at, updated_at`

// Repository provides data access for prompt_template.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new prompt template Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// FindByID returns a single prompt template by ID.
func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (PromptTemplate, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return PromptTemplate{}, err
	}
	defer release()

	query := `SELECT ` + columns + ` FROM prompt_template WHERE id = $1`
	return scanRow(conn.QueryRow(ctx, query, id))
}

// FindByAgentAndSlug returns a template for the given agent and slug.
func (r *Repository) FindByAgentAndSlug(ctx context.Context, agentID uuid.UUID, slug string) (PromptTemplate, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return PromptTemplate{}, err
	}
	defer release()

	query := `SELECT ` + columns + ` FROM prompt_template WHERE agent_id = $1 AND slug = $2`
	return scanRow(conn.QueryRow(ctx, query, agentID, slug))
}

// FindEffectiveByAgentAndSlug returns the best matching template for an agent:
// first an agent-scoped template, then a global template (agent_id IS NULL).
func (r *Repository) FindEffectiveByAgentAndSlug(ctx context.Context, agentID uuid.UUID, slug string) (PromptTemplate, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return PromptTemplate{}, err
	}
	defer release()

	query := `SELECT ` + columns + ` FROM prompt_template
		WHERE slug = $2 AND (agent_id = $1 OR agent_id IS NULL)
		ORDER BY CASE WHEN agent_id = $1 THEN 0 ELSE 1 END, updated_at DESC
		LIMIT 1`
	return scanRow(conn.QueryRow(ctx, query, agentID, slug))
}

// ListByAgent returns paginated templates for an agent (includes global templates where agent_id IS NULL).
func (r *Repository) ListByAgent(ctx context.Context, agentID uuid.UUID, req pagination.PageRequest) ([]PromptTemplate, int64, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	countQuery := `SELECT COUNT(*) FROM prompt_template WHERE agent_id = $1 OR agent_id IS NULL`
	var total int64
	if err := conn.QueryRow(ctx, countQuery, agentID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("prompt template: count: %w", err)
	}

	query := `SELECT ` + columns + ` FROM prompt_template
		WHERE agent_id = $1 OR agent_id IS NULL
		ORDER BY category, name
		LIMIT $2 OFFSET $3`

	rows, err := conn.Query(ctx, query, agentID, req.Size, req.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("prompt template: list: %w", err)
	}
	defer rows.Close()

	templates, err := scanRows(rows)
	return templates, total, err
}

// ListAll returns paginated templates (all for the tenant).
func (r *Repository) ListAll(ctx context.Context, category string, req pagination.PageRequest) ([]PromptTemplate, int64, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var total int64
	if category != "" {
		if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM prompt_template WHERE category = $1`, category).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("prompt template: count: %w", err)
		}
	} else {
		if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM prompt_template`).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("prompt template: count: %w", err)
		}
	}

	var query string
	var args []any
	if category != "" {
		query = `SELECT ` + columns + ` FROM prompt_template WHERE category = $1 ORDER BY name LIMIT $2 OFFSET $3`
		args = []any{category, req.Size, req.Offset()}
	} else {
		query = `SELECT ` + columns + ` FROM prompt_template ORDER BY category, name LIMIT $1 OFFSET $2`
		args = []any{req.Size, req.Offset()}
	}

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("prompt template: list all: %w", err)
	}
	defer rows.Close()

	templates, err := scanRows(rows)
	return templates, total, err
}

// Create inserts a new prompt template.
// ExistsBySlug checks if a prompt template with the given (agentID, slug) exists.
// Handles NULL agent_id correctly using IS NOT DISTINCT FROM.
func (r *Repository) ExistsBySlug(ctx context.Context, agentID *uuid.UUID, slug string) (bool, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return false, err
	}
	defer release()

	var exists bool
	err = conn.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM prompt_template
			WHERE agent_id IS NOT DISTINCT FROM $1 AND slug = $2
		)`,
		agentID, slug,
	).Scan(&exists)
	return exists, err
}

func (r *Repository) Create(ctx context.Context, t PromptTemplate) (PromptTemplate, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return PromptTemplate{}, err
	}
	defer release()

	query := `INSERT INTO prompt_template (agent_id, name, slug, description, content, category, model_override, allowed_tools)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING ` + columns

	row := conn.QueryRow(ctx, query,
		t.AgentID, t.Name, t.Slug, t.Description, t.Content,
		string(t.Category), t.ModelOverride, t.AllowedTools,
	)
	created, err := scanRow(row)
	if err != nil && isDuplicateSlug(err) {
		return PromptTemplate{}, ErrDuplicateSlug
	}
	return created, err
}

// Update modifies an existing prompt template.
func (r *Repository) Update(ctx context.Context, t PromptTemplate) (PromptTemplate, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return PromptTemplate{}, err
	}
	defer release()

	query := `UPDATE prompt_template
		SET name = $2, slug = $3, description = $4, content = $5, category = $6,
		    model_override = $7, allowed_tools = $8, updated_at = NOW()
		WHERE id = $1
		RETURNING ` + columns

	row := conn.QueryRow(ctx, query,
		t.ID, t.Name, t.Slug, t.Description, t.Content,
		string(t.Category), t.ModelOverride, t.AllowedTools,
	)
	updated, err := scanRow(row)
	if err != nil && isDuplicateSlug(err) {
		return PromptTemplate{}, ErrDuplicateSlug
	}
	return updated, err
}

// Delete removes a prompt template by ID.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	ct, err := conn.Exec(ctx, `DELETE FROM prompt_template WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("prompt template: delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRow(row pgx.Row) (PromptTemplate, error) {
	var t PromptTemplate
	var category string
	err := row.Scan(
		&t.ID, &t.AgentID, &t.Name, &t.Slug, &t.Description,
		&t.Content, &category, &t.ModelOverride, &t.AllowedTools,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PromptTemplate{}, ErrNotFound
	}
	if err != nil {
		return PromptTemplate{}, fmt.Errorf("prompt template: scan: %w", err)
	}
	t.Category = Category(category)
	return t, nil
}

func scanRows(rows pgx.Rows) ([]PromptTemplate, error) {
	var items []PromptTemplate
	for rows.Next() {
		var t PromptTemplate
		var category string
		if err := rows.Scan(
			&t.ID, &t.AgentID, &t.Name, &t.Slug, &t.Description,
			&t.Content, &category, &t.ModelOverride, &t.AllowedTools,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("prompt template: scan row: %w", err)
		}
		t.Category = Category(category)
		items = append(items, t)
	}
	if items == nil {
		items = []PromptTemplate{}
	}
	return items, rows.Err()
}

// isDuplicateSlug checks if the error is a unique constraint violation on (agent_id, slug).
func isDuplicateSlug(err error) bool {
	if err == nil {
		return false
	}
	return errors.As(err, new(*duplicateKeyError)) || containsDuplicateKey(err.Error())
}

type duplicateKeyError struct{}

func (e *duplicateKeyError) Error() string { return "duplicate key" }

func containsDuplicateKey(msg string) bool {
	return len(msg) > 0 && (contains(msg, "duplicate key") || contains(msg, "unique constraint"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
