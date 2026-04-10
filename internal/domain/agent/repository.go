package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// ErrNotFound is returned when the agent cannot be found.
var ErrNotFound = errors.New("agent: not found")

// ErrSlugConflict is returned when the slug is already in use.
var ErrSlugConflict = errors.New("agent: slug conflict")

// ErrInvalidModelConfig is returned when modelConfig JSON fails validation.
var ErrInvalidModelConfig = errors.New("agent: invalid model config")

// ErrInvalidSkillIDs is returned when one or more skill IDs do not exist.
var ErrInvalidSkillIDs = errors.New("agent: invalid skill IDs")

// Repository defines persistence operations for Agent.
type Repository interface {
	FindAll(ctx context.Context, status AgentStatus, req pagination.PageRequest) ([]Agent, int64, error)
	FindByID(ctx context.Context, id uuid.UUID) (Agent, error)
	Create(ctx context.Context, a Agent) (Agent, error)
	Update(ctx context.Context, a Agent) (Agent, error)
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status AgentStatus) (Agent, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL-backed Agent Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

const agentColumns = `id, name, slug, description, status, current_version, system_prompt, model_config, permission_rules, config, enable_management, created_at, updated_at`

func scanAgent(row pgx.Row) (Agent, error) {
	var a Agent
	var status string
	var configBytes []byte
	var modelConfigBytes []byte
	var permissionRulesBytes []byte
	err := row.Scan(
		&a.ID, &a.Name, &a.Slug, &a.Description, &status,
		&a.CurrentVersion, &a.SystemPrompt, &modelConfigBytes,
		&permissionRulesBytes, &configBytes, &a.EnableManagement, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return Agent{}, err
	}
	a.Status = AgentStatus(status)
	if len(configBytes) > 0 {
		a.Config = json.RawMessage(configBytes)
	}
	if len(modelConfigBytes) > 0 {
		a.ModelConfig = json.RawMessage(modelConfigBytes)
	}
	if len(permissionRulesBytes) > 0 {
		a.PermissionRules = json.RawMessage(permissionRulesBytes)
	}
	return a, nil
}

func (r *pgRepository) acquire(ctx context.Context) (*pgxpool.Conn, func(), error) {
	tenantID := tenant.FromContext(ctx)
	return database.AcquireWithTenant(ctx, r.pool, tenantID)
}

func (r *pgRepository) FindAll(ctx context.Context, status AgentStatus, req pagination.PageRequest) ([]Agent, int64, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("agent.FindAll: acquire: %w", err)
	}
	defer release()

	var total int64
	var countQ string
	var countArgs []any
	if status != "" {
		countQ = `SELECT COUNT(*) FROM agent WHERE status = $1`
		countArgs = []any{string(status)}
	} else {
		countQ = `SELECT COUNT(*) FROM agent`
	}
	if err := conn.QueryRow(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("agent.FindAll count: %w", err)
	}

	var query string
	var args []any
	if status != "" {
		query = fmt.Sprintf(`SELECT %s FROM agent WHERE status = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, agentColumns)
		args = []any{string(status), req.Size, req.Offset()}
	} else {
		query = fmt.Sprintf(`SELECT %s FROM agent ORDER BY created_at DESC LIMIT $1 OFFSET $2`, agentColumns)
		args = []any{req.Size, req.Offset()}
	}

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("agent.FindAll: %w", err)
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, 0, err
		}
		agents = append(agents, a)
	}
	if agents == nil {
		agents = []Agent{}
	}
	return agents, total, rows.Err()
}

func (r *pgRepository) FindByID(ctx context.Context, id uuid.UUID) (Agent, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return Agent{}, fmt.Errorf("agent.FindByID: acquire: %w", err)
	}
	defer release()

	query := fmt.Sprintf(`SELECT %s FROM agent WHERE id = $1`, agentColumns)
	row := conn.QueryRow(ctx, query, id)
	a, err := scanAgent(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		return Agent{}, fmt.Errorf("agent.FindByID: %w", err)
	}
	return a, nil
}

func (r *pgRepository) Create(ctx context.Context, a Agent) (Agent, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return Agent{}, fmt.Errorf("agent.Create: acquire: %w", err)
	}
	defer release()

	config := a.Config
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}

	var modelConfig []byte
	if len(a.ModelConfig) > 0 {
		modelConfig = []byte(a.ModelConfig)
	}

	var permissionRules []byte
	if len(a.PermissionRules) > 0 {
		permissionRules = []byte(a.PermissionRules)
	}

	query := fmt.Sprintf(`
		INSERT INTO agent (id, name, slug, description, status, current_version, system_prompt, model_config, permission_rules, config, enable_management, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
		RETURNING %s`, agentColumns)
	row := conn.QueryRow(ctx, query,
		a.ID, a.Name, a.Slug, a.Description, string(a.Status),
		a.CurrentVersion, a.SystemPrompt, modelConfig, permissionRules, []byte(config), a.EnableManagement,
	)
	created, err := scanAgent(row)
	if err != nil {
		if isUniqueViolation(err) {
			return Agent{}, ErrSlugConflict
		}
		return Agent{}, fmt.Errorf("agent.Create: %w", err)
	}
	return created, nil
}

func (r *pgRepository) Update(ctx context.Context, a Agent) (Agent, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return Agent{}, fmt.Errorf("agent.Update: acquire: %w", err)
	}
	defer release()

	config := a.Config
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}

	var modelConfig []byte
	if len(a.ModelConfig) > 0 {
		modelConfig = []byte(a.ModelConfig)
	}

	var permissionRules []byte
	if len(a.PermissionRules) > 0 {
		permissionRules = []byte(a.PermissionRules)
	}

	query := fmt.Sprintf(`
		UPDATE agent
		SET name=$2, slug=$3, description=$4, system_prompt=$5, model_config=$6, permission_rules=$7, config=$8, enable_management=$9, updated_at=NOW()
		WHERE id=$1
		RETURNING %s`, agentColumns)
	row := conn.QueryRow(ctx, query,
		a.ID, a.Name, a.Slug, a.Description, a.SystemPrompt, modelConfig, permissionRules, []byte(config), a.EnableManagement,
	)
	updated, err := scanAgent(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return Agent{}, ErrSlugConflict
		}
		return Agent{}, fmt.Errorf("agent.Update: %w", err)
	}
	return updated, nil
}

func (r *pgRepository) Delete(ctx context.Context, id uuid.UUID) error {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return fmt.Errorf("agent.Delete: acquire: %w", err)
	}
	defer release()

	tag, err := conn.Exec(ctx, `DELETE FROM agent WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("agent.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status AgentStatus) (Agent, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return Agent{}, fmt.Errorf("agent.UpdateStatus: acquire: %w", err)
	}
	defer release()

	query := fmt.Sprintf(`
		UPDATE agent SET status=$2, updated_at=NOW() WHERE id=$1 RETURNING %s`, agentColumns)
	row := conn.QueryRow(ctx, query, id, string(status))
	a, err := scanAgent(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		return Agent{}, fmt.Errorf("agent.UpdateStatus: %w", err)
	}
	return a, nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for i := 0; i <= len(msg)-5; i++ {
		if msg[i:i+5] == "23505" {
			return true
		}
	}
	return false
}
