package llmpreset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// ErrNotFound is returned when the preset cannot be found.
var ErrNotFound = errors.New("llmpreset: not found")

// ErrDuplicateName is returned when a preset with the same name already exists for the tenant.
var ErrDuplicateName = errors.New("llmpreset: name already exists for this tenant")

// Repository defines persistence operations for LLMPreset.
type Repository interface {
	FindAll(ctx context.Context, tenantID string, req pagination.PageRequest) ([]LLMPreset, int64, error)
	FindByID(ctx context.Context, tenantID string, id uuid.UUID) (LLMPreset, error)
	FindByProvider(ctx context.Context, tenantID, provider string, req pagination.PageRequest) ([]LLMPreset, int64, error)
	ExistsByName(ctx context.Context, tenantID, name string) (bool, error)
	Create(ctx context.Context, p LLMPreset) (LLMPreset, error)
	Update(ctx context.Context, p LLMPreset) (LLMPreset, error)
	Delete(ctx context.Context, tenantID string, id uuid.UUID) error
	SetDefault(ctx context.Context, tenantID string, id uuid.UUID) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL-backed LLMPreset Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

const presetColumns = `id, tenant_id, name, description, provider, model, base_url, api_key_env,
	max_tokens, temperature, config_json, is_default, is_public, visibility, created_at, updated_at`

func scanPreset(row pgx.Row) (LLMPreset, error) {
	var p LLMPreset
	var configJSON []byte
	err := row.Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Provider, &p.Model,
		&p.BaseURL, &p.APIKeyEnv, &p.MaxTokens, &p.Temperature,
		&configJSON, &p.IsDefault, &p.IsPublic, &p.Visibility,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return LLMPreset{}, err
	}
	if len(configJSON) > 0 {
		p.ConfigJSON = json.RawMessage(configJSON)
	} else {
		p.ConfigJSON = json.RawMessage("{}")
	}
	return p, nil
}

func (r *pgRepository) FindAll(ctx context.Context, tenantID string, req pagination.PageRequest) ([]LLMPreset, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM public.llm_config_preset WHERE tenant_id = $1`, tenantID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("llmpreset.FindAll count: %w", err)
	}

	query := fmt.Sprintf(`SELECT %s FROM public.llm_config_preset WHERE tenant_id = $1 ORDER BY name LIMIT $2 OFFSET $3`, presetColumns)
	rows, err := r.pool.Query(ctx, query, tenantID, req.Size, req.Offset())
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

func (r *pgRepository) FindByProvider(ctx context.Context, tenantID, provider string, req pagination.PageRequest) ([]LLMPreset, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM public.llm_config_preset WHERE tenant_id = $1 AND provider = $2`,
		tenantID, provider,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("llmpreset.FindByProvider count: %w", err)
	}

	query := fmt.Sprintf(`SELECT %s FROM public.llm_config_preset WHERE tenant_id = $1 AND provider = $2 ORDER BY name LIMIT $3 OFFSET $4`, presetColumns)
	rows, err := r.pool.Query(ctx, query, tenantID, provider, req.Size, req.Offset())
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

func (r *pgRepository) FindByID(ctx context.Context, tenantID string, id uuid.UUID) (LLMPreset, error) {
	query := fmt.Sprintf(`SELECT %s FROM public.llm_config_preset WHERE tenant_id = $1 AND id = $2`, presetColumns)
	row := r.pool.QueryRow(ctx, query, tenantID, id)
	p, err := scanPreset(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return LLMPreset{}, ErrNotFound
	}
	if err != nil {
		return LLMPreset{}, fmt.Errorf("llmpreset.FindByID: %w", err)
	}
	return p, nil
}

func (r *pgRepository) ExistsByName(ctx context.Context, tenantID, name string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM public.llm_config_preset WHERE tenant_id = $1 AND name = $2)`,
		tenantID, name,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("llmpreset: exists by name: %w", err)
	}
	return exists, nil
}

func (r *pgRepository) Create(ctx context.Context, p LLMPreset) (LLMPreset, error) {
	configJSON := p.ConfigJSON
	if len(configJSON) == 0 {
		configJSON = json.RawMessage("{}")
	}
	visibility := p.Visibility
	if visibility == "" {
		visibility = VisibilityPrivate
	}
	query := fmt.Sprintf(`
		INSERT INTO public.llm_config_preset
			(id, tenant_id, name, description, provider, model, base_url, api_key_env,
			 max_tokens, temperature, config_json, is_default, is_public, visibility, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), NOW())
		RETURNING %s`, presetColumns)
	row := r.pool.QueryRow(ctx, query,
		p.ID, p.TenantID, p.Name, p.Description, p.Provider, p.Model,
		p.BaseURL, p.APIKeyEnv, p.MaxTokens, p.Temperature,
		[]byte(configJSON), p.IsDefault, p.IsPublic, visibility,
	)
	created, err := scanPreset(row)
	if err != nil {
		return LLMPreset{}, fmt.Errorf("llmpreset.Create: %w", err)
	}
	return created, nil
}

func (r *pgRepository) Update(ctx context.Context, p LLMPreset) (LLMPreset, error) {
	configJSON := p.ConfigJSON
	if len(configJSON) == 0 {
		configJSON = json.RawMessage("{}")
	}
	query := fmt.Sprintf(`
		UPDATE public.llm_config_preset
		SET name=$3, description=$4, provider=$5, model=$6, base_url=$7, api_key_env=$8,
		    max_tokens=$9, temperature=$10, config_json=$11, is_public=$12, visibility=$13, updated_at=NOW()
		WHERE tenant_id=$1 AND id=$2
		RETURNING %s`, presetColumns)
	row := r.pool.QueryRow(ctx, query,
		p.TenantID, p.ID, p.Name, p.Description, p.Provider, p.Model,
		p.BaseURL, p.APIKeyEnv, p.MaxTokens, p.Temperature,
		[]byte(configJSON), p.IsPublic, p.Visibility,
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

func (r *pgRepository) Delete(ctx context.Context, tenantID string, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM public.llm_config_preset WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return fmt.Errorf("llmpreset.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) SetDefault(ctx context.Context, tenantID string, id uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("llmpreset.SetDefault: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx,
		`UPDATE public.llm_config_preset SET is_default = FALSE WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return fmt.Errorf("llmpreset.SetDefault: clear: %w", err)
	}
	tag, err := tx.Exec(ctx,
		`UPDATE public.llm_config_preset SET is_default = TRUE WHERE tenant_id = $1 AND id = $2`,
		tenantID, id)
	if err != nil {
		return fmt.Errorf("llmpreset.SetDefault: set: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}
