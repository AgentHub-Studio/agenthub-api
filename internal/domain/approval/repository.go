package approval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// Repository defines persistence operations for PendingApproval.
type Repository interface {
	Create(ctx context.Context, req CreateApprovalRequest) (PendingApproval, error)
	GetByID(ctx context.Context, id uuid.UUID) (PendingApproval, error)
	List(ctx context.Context, offset, limit int) ([]PendingApproval, int64, error)
	PendingCount(ctx context.Context) (int64, error)
	Respond(ctx context.Context, id uuid.UUID, respondedBy string, req RespondRequest) (PendingApproval, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL-backed approval repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

const approvalColumns = `id, execution_id, node_id, title, description, details, status,
	responded_by, responded_at, comment, timeout_at, callback_url, notify_channels, created_at`

func (r *pgRepository) setTenant(ctx context.Context, conn *pgxpool.Conn) error {
	tenantID := tenant.FromContext(ctx)
	_, err := conn.Exec(ctx, "SET search_path TO "+approvalTenantSearchPath(tenantID))
	return err
}

func approvalTenantSearchPath(tenantID string) string {
	schema := fmt.Sprintf("ah_%s", tenantID)
	return pgx.Identifier{schema}.Sanitize() + ", public"
}

func (r *pgRepository) acquire(ctx context.Context) (*pgxpool.Conn, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("approval: acquire connection: %w", err)
	}
	if err = r.setTenant(ctx, conn); err != nil {
		conn.Release()
		return nil, fmt.Errorf("approval: set search_path: %w", err)
	}
	return conn, nil
}

func scanApproval(row pgx.Row) (PendingApproval, error) {
	var a PendingApproval
	var channelsJSON []byte
	err := row.Scan(
		&a.ID, &a.ExecutionID, &a.NodeID, &a.Title, &a.Description, &a.Details, &a.Status,
		&a.RespondedBy, &a.RespondedAt, &a.Comment, &a.TimeoutAt, &a.CallbackURL,
		&channelsJSON, &a.CreatedAt,
	)
	if err != nil {
		return PendingApproval{}, err
	}
	if len(channelsJSON) > 0 {
		_ = json.Unmarshal(channelsJSON, &a.NotifyChannels)
	}
	if a.NotifyChannels == nil {
		a.NotifyChannels = []string{}
	}
	return a, nil
}

func scanApprovalRow(rows pgx.Rows) (PendingApproval, error) {
	var a PendingApproval
	var channelsJSON []byte
	err := rows.Scan(
		&a.ID, &a.ExecutionID, &a.NodeID, &a.Title, &a.Description, &a.Details, &a.Status,
		&a.RespondedBy, &a.RespondedAt, &a.Comment, &a.TimeoutAt, &a.CallbackURL,
		&channelsJSON, &a.CreatedAt,
	)
	if err != nil {
		return PendingApproval{}, err
	}
	if len(channelsJSON) > 0 {
		_ = json.Unmarshal(channelsJSON, &a.NotifyChannels)
	}
	if a.NotifyChannels == nil {
		a.NotifyChannels = []string{}
	}
	return a, nil
}

func (r *pgRepository) Create(ctx context.Context, req CreateApprovalRequest) (PendingApproval, error) {
	conn, err := r.acquire(ctx)
	if err != nil {
		return PendingApproval{}, err
	}
	defer conn.Release()

	channels := req.NotifyChannels
	if channels == nil {
		channels = []string{}
	}
	channelsJSON, _ := json.Marshal(channels)
	var callbackURL any
	if req.CallbackURL != nil {
		callbackURL = *req.CallbackURL
	}

	row := conn.QueryRow(ctx, `
		INSERT INTO pending_approval
			(execution_id, node_id, title, description, details, status, timeout_at, callback_url, notify_channels)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+approvalColumns,
		req.ExecutionID, req.NodeID, req.Title, req.Description, req.Details,
		StatusPending, req.TimeoutAt, callbackURL, channelsJSON,
	)
	return scanApproval(row)
}

func (r *pgRepository) GetByID(ctx context.Context, id uuid.UUID) (PendingApproval, error) {
	conn, err := r.acquire(ctx)
	if err != nil {
		return PendingApproval{}, err
	}
	defer conn.Release()

	row := conn.QueryRow(ctx,
		`SELECT `+approvalColumns+` FROM pending_approval WHERE id = $1`, id)
	a, err := scanApproval(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return PendingApproval{}, ErrNotFound
	}
	return a, err
}

func (r *pgRepository) List(ctx context.Context, offset, limit int) ([]PendingApproval, int64, error) {
	conn, err := r.acquire(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()

	var total int64
	if err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM pending_approval`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := conn.Query(ctx,
		`SELECT `+approvalColumns+` FROM pending_approval ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []PendingApproval{}
	for rows.Next() {
		a, err := scanApprovalRow(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, a)
	}
	return list, total, rows.Err()
}

func (r *pgRepository) PendingCount(ctx context.Context) (int64, error) {
	conn, err := r.acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()

	var count int64
	err = conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM pending_approval WHERE status = $1`, StatusPending,
	).Scan(&count)
	return count, err
}

func (r *pgRepository) Delete(ctx context.Context, id uuid.UUID) error {
	conn, err := r.acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tag, err := conn.Exec(ctx, `DELETE FROM pending_approval WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("approval: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepository) Respond(ctx context.Context, id uuid.UUID, respondedBy string, req RespondRequest) (PendingApproval, error) {
	conn, err := r.acquire(ctx)
	if err != nil {
		return PendingApproval{}, err
	}
	defer conn.Release()

	status := StatusApproved
	if !req.Approved {
		status = StatusRejected
	}
	now := time.Now()
	var comment any
	if req.Comment != nil {
		comment = *req.Comment
	}

	row := conn.QueryRow(ctx, `
		UPDATE pending_approval
		SET status = $1, responded_by = $2, responded_at = $3, comment = $4
		WHERE id = $5 AND status = $6
		RETURNING `+approvalColumns,
		status, respondedBy, now, comment, id, StatusPending,
	)
	a, err := scanApproval(row)
	if errors.Is(err, pgx.ErrNoRows) {
		// Disambiguate: not found vs already resolved
		var existing PendingApproval
		fetchRow := conn.QueryRow(ctx,
			`SELECT `+approvalColumns+` FROM pending_approval WHERE id = $1`, id)
		existing, ferr := scanApproval(fetchRow)
		if errors.Is(ferr, pgx.ErrNoRows) || existing.ID == uuid.Nil {
			return PendingApproval{}, ErrNotFound
		}
		return PendingApproval{}, ErrAlreadyResolved
	}
	return a, err
}
