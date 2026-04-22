package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines read-only access to the provider catalog.
// The catalog is seeded via migration (000015_provider_catalog.up.sql)
// and is not editable through the API — builtin only for now.
type Repository interface {
	ListAll(ctx context.Context) ([]Provider, error)
	ListByKind(ctx context.Context, kind Kind) ([]Provider, error)
	GetBySlug(ctx context.Context, slug string) (Provider, error)
}

type repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Provider repository backed by the given pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{pool: pool}
}

const selectCols = `slug, name, description, icon, category, kind, template_json, is_builtin, enabled, created_at, updated_at`

func scan(row pgx.Row) (Provider, error) {
	var p Provider
	var tmpl []byte
	err := row.Scan(
		&p.Slug, &p.Name, &p.Description, &p.Icon, &p.Category,
		&p.Kind, &tmpl, &p.IsBuiltin, &p.Enabled, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return Provider{}, err
	}
	if len(tmpl) > 0 {
		p.TemplateJSON = json.RawMessage(tmpl)
	}
	return p, nil
}

func (r *repository) ListAll(ctx context.Context) ([]Provider, error) {
	q := `SELECT ` + selectCols + ` FROM public.provider_template WHERE enabled = true ORDER BY category, name`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("provider list: %w", err)
	}
	defer rows.Close()
	return collectRows(rows)
}

func (r *repository) ListByKind(ctx context.Context, kind Kind) ([]Provider, error) {
	q := `SELECT ` + selectCols + ` FROM public.provider_template WHERE enabled = true AND kind = $1 ORDER BY category, name`
	rows, err := r.pool.Query(ctx, q, string(kind))
	if err != nil {
		return nil, fmt.Errorf("provider list by kind: %w", err)
	}
	defer rows.Close()
	return collectRows(rows)
}

func (r *repository) GetBySlug(ctx context.Context, slug string) (Provider, error) {
	q := `SELECT ` + selectCols + ` FROM public.provider_template WHERE slug = $1`
	row := r.pool.QueryRow(ctx, q, slug)
	p, err := scan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Provider{}, ErrNotFound
		}
		return Provider{}, fmt.Errorf("provider get by slug: %w", err)
	}
	return p, nil
}

func collectRows(rows pgx.Rows) ([]Provider, error) {
	var out []Provider
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("provider scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

var _ Repository = (*repository)(nil)
