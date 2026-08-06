package abtest

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// Repository provides persistence for ABTest and Assignment.
type Repository interface {
	List(ctx context.Context, agentID uuid.UUID, req pagination.PageRequest) (pagination.Page[ABTest], error)
	GetByID(ctx context.Context, id uuid.UUID) (ABTest, error)
	Create(ctx context.Context, t ABTest) (ABTest, error)
	Update(ctx context.Context, t ABTest) (ABTest, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// GetActiveByAgent returns the single ACTIVE test for the given agent, if any.
	GetActiveByAgent(ctx context.Context, agentID uuid.UUID) (ABTest, error)

	// RecordAssignment persists a session variant assignment.
	RecordAssignment(ctx context.Context, a Assignment) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a Repository backed by PostgreSQL.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) acquire(ctx context.Context) (*pgxpool.Conn, func(), error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return nil, nil, fmt.Errorf("abtest: acquire tenant connection: %w", err)
	}
	return conn, release, nil
}

func (r *pgRepository) List(ctx context.Context, agentID uuid.UUID, req pagination.PageRequest) (pagination.Page[ABTest], error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return pagination.Page[ABTest]{}, err
	}
	defer release()

	offset := req.Page * req.Size
	rows, err := conn.Query(ctx, `
		SELECT id, agent_id, name, description,
		       control_version_id, variant_version_id,
		       traffic_percent, status, started_at, ended_at, created_at, updated_at
		FROM agent_ab_test
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		agentID, req.Size, offset,
	)
	if err != nil {
		return pagination.Page[ABTest]{}, fmt.Errorf("abtest: list: %w", err)
	}
	defer rows.Close()

	var items []ABTest
	for rows.Next() {
		t, err := scanTest(rows)
		if err != nil {
			return pagination.Page[ABTest]{}, fmt.Errorf("abtest: list scan: %w", err)
		}
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		return pagination.Page[ABTest]{}, fmt.Errorf("abtest: list: %w", err)
	}

	var total int64
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM agent_ab_test WHERE agent_id = $1`, agentID).Scan(&total); err != nil {
		return pagination.Page[ABTest]{}, fmt.Errorf("abtest: list count: %w", err)
	}

	return pagination.NewPage(items, total, req), nil
}

func (r *pgRepository) GetByID(ctx context.Context, id uuid.UUID) (ABTest, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return ABTest{}, err
	}
	defer release()

	row := conn.QueryRow(ctx, `
		SELECT id, agent_id, name, description,
		       control_version_id, variant_version_id,
		       traffic_percent, status, started_at, ended_at, created_at, updated_at
		FROM agent_ab_test WHERE id = $1`, id)
	t, err := scanTest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ABTest{}, ErrNotFound
		}
		return ABTest{}, fmt.Errorf("abtest: get: %w", err)
	}
	return t, nil
}

func (r *pgRepository) Create(ctx context.Context, t ABTest) (ABTest, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return ABTest{}, err
	}
	defer release()

	t.ID = uuid.New()
	err = conn.QueryRow(ctx, `
		INSERT INTO agent_ab_test
		  (id, agent_id, name, description, control_version_id, variant_version_id,
		   traffic_percent, status, started_at, ended_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW(),NOW())
		RETURNING id, agent_id, name, description,
		          control_version_id, variant_version_id,
		          traffic_percent, status, started_at, ended_at, created_at, updated_at`,
		t.ID, t.AgentID, t.Name, t.Description,
		t.ControlVersionID, t.VariantVersionID, t.TrafficPercent,
		t.Status, t.StartedAt, t.EndedAt,
	).Scan(
		&t.ID, &t.AgentID, &t.Name, &t.Description,
		&t.ControlVersionID, &t.VariantVersionID,
		&t.TrafficPercent, &t.Status, &t.StartedAt, &t.EndedAt,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if isActiveTestUniqueViolation(err) {
			return ABTest{}, ErrActiveTestConflict
		}
		if isUniqueViolation(err) {
			return ABTest{}, ErrNameConflict
		}
		return ABTest{}, fmt.Errorf("abtest: create: %w", err)
	}
	return t, nil
}

func (r *pgRepository) Update(ctx context.Context, t ABTest) (ABTest, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return ABTest{}, err
	}
	defer release()

	err = conn.QueryRow(ctx, `
		UPDATE agent_ab_test SET
		  name=$2, description=$3, control_version_id=$4, variant_version_id=$5,
		  traffic_percent=$6, status=$7, ended_at=$8, updated_at=NOW()
		WHERE id=$1
		RETURNING id, agent_id, name, description,
		          control_version_id, variant_version_id,
		          traffic_percent, status, started_at, ended_at, created_at, updated_at`,
		t.ID, t.Name, t.Description,
		t.ControlVersionID, t.VariantVersionID,
		t.TrafficPercent, t.Status, t.EndedAt,
	).Scan(
		&t.ID, &t.AgentID, &t.Name, &t.Description,
		&t.ControlVersionID, &t.VariantVersionID,
		&t.TrafficPercent, &t.Status, &t.StartedAt, &t.EndedAt,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ABTest{}, ErrNotFound
		}
		if isActiveTestUniqueViolation(err) {
			return ABTest{}, ErrActiveTestConflict
		}
		return ABTest{}, fmt.Errorf("abtest: update: %w", err)
	}
	return t, nil
}

func (r *pgRepository) Delete(ctx context.Context, id uuid.UUID) error {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return err
	}
	defer release()

	tag, err := conn.Exec(ctx, `DELETE FROM agent_ab_test WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("abtest: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) GetActiveByAgent(ctx context.Context, agentID uuid.UUID) (ABTest, error) {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return ABTest{}, err
	}
	defer release()

	row := conn.QueryRow(ctx, `
		SELECT id, agent_id, name, description,
		       control_version_id, variant_version_id,
		       traffic_percent, status, started_at, ended_at, created_at, updated_at
		FROM agent_ab_test
		WHERE agent_id = $1 AND status = 'ACTIVE'
		LIMIT 1`, agentID)
	t, err := scanTest(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ABTest{}, ErrNotFound
		}
		return ABTest{}, fmt.Errorf("abtest: get active: %w", err)
	}
	return t, nil
}

func (r *pgRepository) RecordAssignment(ctx context.Context, a Assignment) error {
	conn, release, err := r.acquire(ctx)
	if err != nil {
		return err
	}
	defer release()

	a.ID = uuid.New()
	_, err = conn.Exec(ctx, `
		INSERT INTO agent_ab_assignment (id, test_id, session_id, variant, assigned_at)
		VALUES ($1,$2,$3,$4,NOW())`,
		a.ID, a.TestID, a.SessionID, string(a.Variant),
	)
	if err != nil {
		return fmt.Errorf("abtest: record assignment: %w", err)
	}
	return nil
}

// scanTest reads one row into an ABTest.
type scanner interface {
	Scan(dest ...any) error
}

func scanTest(row scanner) (ABTest, error) {
	var t ABTest
	err := row.Scan(
		&t.ID, &t.AgentID, &t.Name, &t.Description,
		&t.ControlVersionID, &t.VariantVersionID,
		&t.TrafficPercent, &t.Status, &t.StartedAt, &t.EndedAt,
		&t.CreatedAt, &t.UpdatedAt,
	)
	return t, err
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}

func isActiveTestUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_agent_ab_test_active_per_agent"
}
