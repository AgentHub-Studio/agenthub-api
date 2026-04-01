package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// VersionRepository defines persistence operations for AgentVersion.
type VersionRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (AgentVersion, error)
	FindByAgentID(ctx context.Context, agentID uuid.UUID, req pagination.PageRequest) ([]AgentVersion, int64, error)
	FindDraft(ctx context.Context, agentID uuid.UUID) (AgentVersion, error)
	FindLatestPublished(ctx context.Context, agentID uuid.UUID) (AgentVersion, error)
	Create(ctx context.Context, v AgentVersion) (AgentVersion, error)
	Update(ctx context.Context, v AgentVersion) (AgentVersion, error)
	Publish(ctx context.Context, id uuid.UUID) (AgentVersion, error)
	NextVersionNumber(ctx context.Context, agentID uuid.UUID) (int, error)
}

type pgVersionRepository struct {
	pool *pgxpool.Pool
}

// NewVersionRepository creates a new PostgreSQL-backed VersionRepository.
func NewVersionRepository(pool *pgxpool.Pool) VersionRepository {
	return &pgVersionRepository{pool: pool}
}

const versionColumns = `id, agent_id, version_number, status, description, definition_json, config_json, created_at, updated_at, published_at`

func scanVersion(row pgx.Row) (AgentVersion, error) {
	var v AgentVersion
	var status string
	var defBytes, cfgBytes []byte
	var publishedAt *time.Time
	err := row.Scan(
		&v.ID, &v.AgentID, &v.VersionNumber, &status, &v.Description,
		&defBytes, &cfgBytes, &v.CreatedAt, &v.UpdatedAt, &publishedAt,
	)
	if err != nil {
		return AgentVersion{}, err
	}
	v.Status = VersionStatus(status)
	v.PublishedAt = publishedAt
	if len(defBytes) > 0 {
		v.DefinitionJSON = json.RawMessage(defBytes)
	}
	if len(cfgBytes) > 0 {
		v.ConfigJSON = json.RawMessage(cfgBytes)
	}
	return v, nil
}

func (r *pgVersionRepository) acquire(ctx context.Context) (*pgxpool.Conn, func(), error) {
	tenantID := tenant.FromContext(ctx)
	return database.AcquireWithTenant(ctx, r.pool, tenantID)
}

func (r *pgVersionRepository) FindByID(ctx context.Context, id uuid.UUID) (AgentVersion, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return AgentVersion{}, fmt.Errorf("agentVersion.FindByID: acquire: %w", err)
	}
	defer release()

	q := fmt.Sprintf(`SELECT %s FROM agent_version WHERE id = $1`, versionColumns)
	v, err := scanVersion(conn.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentVersion{}, ErrVersionNotFound
	}
	if err != nil {
		return AgentVersion{}, fmt.Errorf("agentVersion.FindByID: %w", err)
	}
	return v, nil
}

func (r *pgVersionRepository) FindByAgentID(ctx context.Context, agentID uuid.UUID, req pagination.PageRequest) ([]AgentVersion, int64, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("agentVersion.FindByAgentID: acquire: %w", err)
	}
	defer release()

	var total int64
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM agent_version WHERE agent_id = $1`, agentID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("agentVersion.FindByAgentID count: %w", err)
	}

	q := fmt.Sprintf(`SELECT %s FROM agent_version WHERE agent_id = $1 ORDER BY version_number DESC LIMIT $2 OFFSET $3`, versionColumns)
	rows, err := conn.Query(ctx, q, agentID, req.Size, req.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("agentVersion.FindByAgentID: %w", err)
	}
	defer rows.Close()

	var versions []AgentVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, 0, err
		}
		versions = append(versions, v)
	}
	if versions == nil {
		versions = []AgentVersion{}
	}
	return versions, total, rows.Err()
}

func (r *pgVersionRepository) FindDraft(ctx context.Context, agentID uuid.UUID) (AgentVersion, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return AgentVersion{}, fmt.Errorf("agentVersion.FindDraft: acquire: %w", err)
	}
	defer release()

	q := fmt.Sprintf(`SELECT %s FROM agent_version WHERE agent_id = $1 AND status = 'DRAFT' LIMIT 1`, versionColumns)
	v, err := scanVersion(conn.QueryRow(ctx, q, agentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentVersion{}, ErrVersionNotFound
	}
	if err != nil {
		return AgentVersion{}, fmt.Errorf("agentVersion.FindDraft: %w", err)
	}
	return v, nil
}

func (r *pgVersionRepository) FindLatestPublished(ctx context.Context, agentID uuid.UUID) (AgentVersion, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return AgentVersion{}, fmt.Errorf("agentVersion.FindLatestPublished: acquire: %w", err)
	}
	defer release()

	q := fmt.Sprintf(`SELECT %s FROM agent_version WHERE agent_id = $1 AND status = 'PUBLISHED' ORDER BY version_number DESC LIMIT 1`, versionColumns)
	v, err := scanVersion(conn.QueryRow(ctx, q, agentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentVersion{}, ErrVersionNotFound
	}
	if err != nil {
		return AgentVersion{}, fmt.Errorf("agentVersion.FindLatestPublished: %w", err)
	}
	return v, nil
}

func (r *pgVersionRepository) Create(ctx context.Context, v AgentVersion) (AgentVersion, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return AgentVersion{}, fmt.Errorf("agentVersion.Create: acquire: %w", err)
	}
	defer release()

	def := v.DefinitionJSON
	if len(def) == 0 {
		def = json.RawMessage(`{}`)
	}
	cfg := v.ConfigJSON
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}

	q := fmt.Sprintf(`
		INSERT INTO agent_version (id, agent_id, version_number, status, description, definition_json, config_json, created_at, updated_at, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW(), NULL)
		RETURNING %s`, versionColumns)
	created, err := scanVersion(conn.QueryRow(ctx, q,
		v.ID, v.AgentID, v.VersionNumber, string(v.Status), v.Description, []byte(def), []byte(cfg),
	))
	if err != nil {
		return AgentVersion{}, fmt.Errorf("agentVersion.Create: %w", err)
	}
	return created, nil
}

func (r *pgVersionRepository) Update(ctx context.Context, v AgentVersion) (AgentVersion, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return AgentVersion{}, fmt.Errorf("agentVersion.Update: acquire: %w", err)
	}
	defer release()

	def := v.DefinitionJSON
	if len(def) == 0 {
		def = json.RawMessage(`{}`)
	}
	cfg := v.ConfigJSON
	if len(cfg) == 0 {
		cfg = json.RawMessage(`{}`)
	}

	q := fmt.Sprintf(`
		UPDATE agent_version
		SET description=$2, definition_json=$3, config_json=$4, updated_at=NOW()
		WHERE id=$1
		RETURNING %s`, versionColumns)
	updated, err := scanVersion(conn.QueryRow(ctx, q, v.ID, v.Description, []byte(def), []byte(cfg)))
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentVersion{}, ErrVersionNotFound
	}
	if err != nil {
		return AgentVersion{}, fmt.Errorf("agentVersion.Update: %w", err)
	}
	return updated, nil
}

func (r *pgVersionRepository) Publish(ctx context.Context, id uuid.UUID) (AgentVersion, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return AgentVersion{}, fmt.Errorf("agentVersion.Publish: acquire: %w", err)
	}
	defer release()

	q := fmt.Sprintf(`
		UPDATE agent_version
		SET status='PUBLISHED', published_at=NOW(), updated_at=NOW()
		WHERE id=$1
		RETURNING %s`, versionColumns)
	v, err := scanVersion(conn.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentVersion{}, ErrVersionNotFound
	}
	if err != nil {
		return AgentVersion{}, fmt.Errorf("agentVersion.Publish: %w", err)
	}
	return v, nil
}

func (r *pgVersionRepository) NextVersionNumber(ctx context.Context, agentID uuid.UUID) (int, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("agentVersion.NextVersionNumber: acquire: %w", err)
	}
	defer release()

	var max *int
	if err := conn.QueryRow(ctx, `SELECT MAX(version_number) FROM agent_version WHERE agent_id = $1`, agentID).Scan(&max); err != nil {
		return 0, fmt.Errorf("agentVersion.NextVersionNumber: %w", err)
	}
	if max == nil {
		return 1, nil
	}
	return *max + 1, nil
}
