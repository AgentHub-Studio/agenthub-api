package llmpreset

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// ErrNotFound is returned when the preset cannot be found.
var ErrNotFound = errors.New("llmpreset: not found")

// Repository defines persistence operations for LLMPreset.
type Repository interface {
	FindAll(ctx context.Context, req pagination.PageRequest) ([]LLMPreset, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (LLMPreset, error)
	FindByProvider(ctx context.Context, provider string, req pagination.PageRequest) ([]LLMPreset, int64, error)
	Create(ctx context.Context, p LLMPreset) (LLMPreset, error)
	Update(ctx context.Context, p LLMPreset) (LLMPreset, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SetDefault(ctx context.Context, id uuid.UUID) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL-backed LLMPreset Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

const presetColumns = `id, name, provider, model, base_url, api_key_env, max_tokens, temperature, is_default, created_at`

func scanPreset(row pgx.Row) (LLMPreset, error) {
	var p LLMPreset
	err := row.Scan(
		&p.ID, &p.Name, &p.Provider, &p.Model,
		&p.BaseURL, &p.APIKeyEnv, &p.MaxTokens, &p.Temperature,
		&p.IsDefault, &p.CreatedAt,
	)
	if err != nil {
		return LLMPreset{}, err
	}
	return p, nil
}

func (r *pgRepository) FindAll(ctx context.Context, req pagination.PageRequest) ([]LLMPreset, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.llm_config_preset`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("llmpreset.FindAll count: %w", err)
	}

	query := fmt.Sprintf(`SELECT %s FROM public.llm_config_preset ORDER BY name LIMIT $1 OFFSET $2`, presetColumns)
	rows, err := r.pool.Query(ctx, query, req.Size, req.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("llmpreset.FindAll: %w", err)
	}
	defer rows.Close()

	var presets []LLMPreset
	for rows.Next() {
		p, err := scanPreset(rows)
		if err != nil {
			return nil, 0, err
		}
		presets = append(presets, p)
	}
	if presets == nil {
		presets = []LLMPreset{}
	}
	return presets, total, rows.Err()
}

func (r *pgRepository) FindByProvider(ctx context.Context, provider string, req pagination.PageRequest) ([]LLMPreset, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.llm_config_preset WHERE provider = $1`, provider).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("llmpreset.FindByProvider count: %w", err)
	}

	query := fmt.Sprintf(`SELECT %s FROM public.llm_config_preset WHERE provider = $1 ORDER BY name LIMIT $2 OFFSET $3`, presetColumns)
	rows, err := r.pool.Query(ctx, query, provider, req.Size, req.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("llmpreset.FindByProvider: %w", err)
	}
	defer rows.Close()

	var presets []LLMPreset
	for rows.Next() {
		p, err := scanPreset(rows)
		if err != nil {
			return nil, 0, err
		}
		presets = append(presets, p)
	}
	if presets == nil {
		presets = []LLMPreset{}
	}
	return presets, total, rows.Err()
}

func (r *pgRepository) FindByID(ctx context.Context, id uuid.UUID) (LLMPreset, error) {
	query := fmt.Sprintf(`SELECT %s FROM public.llm_config_preset WHERE id = $1`, presetColumns)
	row := r.pool.QueryRow(ctx, query, id)
	p, err := scanPreset(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return LLMPreset{}, ErrNotFound
	}
	if err != nil {
		return LLMPreset{}, fmt.Errorf("llmpreset.FindByID: %w", err)
	}
	return p, nil
}

func (r *pgRepository) Create(ctx context.Context, p LLMPreset) (LLMPreset, error) {
	query := fmt.Sprintf(`
		INSERT INTO public.llm_config_preset (id, name, provider, model, base_url, api_key_env, max_tokens, temperature, is_default, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		RETURNING %s`, presetColumns)
	row := r.pool.QueryRow(ctx, query,
		p.ID, p.Name, p.Provider, p.Model,
		p.BaseURL, p.APIKeyEnv, p.MaxTokens, p.Temperature, p.IsDefault,
	)
	created, err := scanPreset(row)
	if err != nil {
		return LLMPreset{}, fmt.Errorf("llmpreset.Create: %w", err)
	}
	return created, nil
}

func (r *pgRepository) Update(ctx context.Context, p LLMPreset) (LLMPreset, error) {
	query := fmt.Sprintf(`
		UPDATE public.llm_config_preset
		SET name=$2, provider=$3, model=$4, base_url=$5, api_key_env=$6, max_tokens=$7, temperature=$8
		WHERE id=$1
		RETURNING %s`, presetColumns)
	row := r.pool.QueryRow(ctx, query,
		p.ID, p.Name, p.Provider, p.Model,
		p.BaseURL, p.APIKeyEnv, p.MaxTokens, p.Temperature,
	)
	updated, err := scanPreset(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return LLMPreset{}, ErrNotFound
	}
	if err != nil {
		return LLMPreset{}, fmt.Errorf("llmpreset.Update: %w", err)
	}
	return updated, nil
}

func (r *pgRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM public.llm_config_preset WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("llmpreset.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) SetDefault(ctx context.Context, id uuid.UUID) error {
	// Use a transaction: clear all defaults then set the requested one.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("llmpreset.SetDefault: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `UPDATE public.llm_config_preset SET is_default = FALSE`)
	if err != nil {
		return fmt.Errorf("llmpreset.SetDefault: clear: %w", err)
	}
	tag, err := tx.Exec(ctx, `UPDATE public.llm_config_preset SET is_default = TRUE WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("llmpreset.SetDefault: set: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}
