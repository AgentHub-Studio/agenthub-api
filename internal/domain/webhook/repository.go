package webhook

import (
	"context"
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

// ErrNotFound is returned when a webhook is not found.
var ErrNotFound = errors.New("webhook: not found")

// WebhookRepository defines the persistence interface for WebhookConfig.
type WebhookRepository interface {
	List(ctx context.Context) ([]WebhookConfig, error)
	Create(ctx context.Context, w WebhookConfig) (WebhookConfig, error)
	GetByID(ctx context.Context, id uuid.UUID) (WebhookConfig, error)
	GetBySecret(ctx context.Context, secret string) (WebhookConfig, error)
	GetByToken(ctx context.Context, token string) (WebhookConfig, error)
	Update(ctx context.Context, w WebhookConfig) (WebhookConfig, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListDeliveries(ctx context.Context, webhookID uuid.UUID, filter DeliveryFilter, req pagination.PageRequest) ([]WebhookDeliveryLog, int64, error)
	CreateDelivery(ctx context.Context, d WebhookDeliveryLog) (WebhookDeliveryLog, error)
	UpdateDelivery(ctx context.Context, d WebhookDeliveryLog) (WebhookDeliveryLog, error)
}

// Repository provides data access for webhook tables.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new webhook Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// List returns all webhooks for the current tenant.
func (r *Repository) List(ctx context.Context) ([]WebhookConfig, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	rows, err := conn.Query(ctx, `
		SELECT id, name, url, events, secret, enabled, retry_count, created_at, updated_at
		  FROM webhook_config ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("webhook: list: %w", err)
	}
	defer rows.Close()
	return scanConfigRows(rows)
}

// Create inserts a new webhook configuration.
func (r *Repository) Create(ctx context.Context, w WebhookConfig) (WebhookConfig, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return WebhookConfig{}, err
	}
	defer release()

	const query = `
		INSERT INTO webhook_config (name, url, events, secret, enabled, retry_count)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, url, events, secret, enabled, retry_count, created_at, updated_at`
	row := conn.QueryRow(ctx, query, w.Name, w.URL, w.Events, w.Secret, w.Enabled, w.RetryCount)
	return scanConfigRow(row)
}

// GetByID returns a single webhook configuration.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (WebhookConfig, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return WebhookConfig{}, err
	}
	defer release()

	row := conn.QueryRow(ctx, `
		SELECT id, name, url, events, secret, enabled, retry_count, created_at, updated_at
		  FROM webhook_config WHERE id = $1`, id)
	return scanConfigRow(row)
}

// GetBySecret returns a webhook configuration by its secret token.
func (r *Repository) GetBySecret(ctx context.Context, secret string) (WebhookConfig, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return WebhookConfig{}, err
	}
	defer release()

	row := conn.QueryRow(ctx, `
		SELECT id, name, url, events, secret, enabled, retry_count, created_at, updated_at
		  FROM webhook_config WHERE secret = $1 AND enabled = TRUE`, secret)
	return scanConfigRow(row)
}

// GetByToken returns a webhook configuration by its authentication token.
func (r *Repository) GetByToken(ctx context.Context, token string) (WebhookConfig, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return WebhookConfig{}, err
	}
	defer release()

	row := conn.QueryRow(ctx, `
		SELECT id, name, url, events, secret, token, enabled, retry_count, created_at, updated_at
		  FROM webhook_config WHERE token = $1 AND enabled = TRUE`, token)
	return scanConfigRow(row)
}

// Update updates a webhook configuration.
func (r *Repository) Update(ctx context.Context, w WebhookConfig) (WebhookConfig, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return WebhookConfig{}, err
	}
	defer release()

	const query = `
		UPDATE webhook_config
		   SET name = $2, url = $3, events = $4, secret = $5, enabled = $6, retry_count = $7, updated_at = NOW()
		 WHERE id = $1
		RETURNING id, name, url, events, secret, enabled, retry_count, created_at, updated_at`
	row := conn.QueryRow(ctx, query, w.ID, w.Name, w.URL, w.Events, w.Secret, w.Enabled, w.RetryCount)
	return scanConfigRow(row)
}

// Delete removes a webhook configuration.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	ct, err := conn.Exec(ctx, `DELETE FROM webhook_config WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("webhook: delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListDeliveries returns paginated delivery logs for a webhook, optionally filtered
// by status and/or event_type.
func (r *Repository) ListDeliveries(ctx context.Context, webhookID uuid.UUID, filter DeliveryFilter, req pagination.PageRequest) ([]WebhookDeliveryLog, int64, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	// Build dynamic args: $1 = webhookID; optional $2/$3 for status/event_type.
	args := []any{webhookID}
	where := "WHERE webhook_id = $1"
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.EventType != "" {
		args = append(args, filter.EventType)
		where += fmt.Sprintf(" AND event_type = $%d", len(args))
	}

	var total int64
	countQuery := "SELECT COUNT(*) FROM webhook_delivery_log " + where
	if err := conn.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("webhook: count deliveries: %w", err)
	}

	// Append pagination args after the filter args.
	limitArg := len(args) + 1
	offsetArg := len(args) + 2
	args = append(args, req.Size, req.Offset())

	listQuery := fmt.Sprintf(`
		SELECT id, webhook_id, event_type, payload, status, response_status, response_body,
		       attempts, delivered_at, created_at
		  FROM webhook_delivery_log
		 %s
		 ORDER BY created_at DESC
		 LIMIT $%d OFFSET $%d`, where, limitArg, offsetArg)

	rows, err := conn.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("webhook: list deliveries: %w", err)
	}
	defer rows.Close()

	items, err := scanDeliveryRows(rows)
	return items, total, err
}

// CreateDelivery inserts a delivery log entry.
func (r *Repository) CreateDelivery(ctx context.Context, d WebhookDeliveryLog) (WebhookDeliveryLog, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return WebhookDeliveryLog{}, err
	}
	defer release()

	const query = `
		INSERT INTO webhook_delivery_log (webhook_id, event_type, payload, status, response_status, response_body, attempts, delivered_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, webhook_id, event_type, payload, status, response_status, response_body, attempts, delivered_at, created_at`
	row := conn.QueryRow(ctx, query,
		d.WebhookID, d.EventType, d.Payload, d.Status, d.ResponseStatus, d.ResponseBody, d.Attempts, d.DeliveredAt)
	return scanDeliveryRow(row)
}

// UpdateDelivery updates a delivery log entry.
func (r *Repository) UpdateDelivery(ctx context.Context, d WebhookDeliveryLog) (WebhookDeliveryLog, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return WebhookDeliveryLog{}, err
	}
	defer release()

	const query = `
		UPDATE webhook_delivery_log
		   SET status = $2, response_status = $3, response_body = $4, attempts = $5, delivered_at = $6
		 WHERE id = $1
		RETURNING id, webhook_id, event_type, payload, status, response_status, response_body, attempts, delivered_at, created_at`
	row := conn.QueryRow(ctx, query, d.ID, d.Status, d.ResponseStatus, d.ResponseBody, d.Attempts, d.DeliveredAt)
	return scanDeliveryRow(row)
}

func scanConfigRow(row pgx.Row) (WebhookConfig, error) {
	var w WebhookConfig
	err := row.Scan(&w.ID, &w.Name, &w.URL, &w.Events, &w.Secret, &w.Enabled, &w.RetryCount, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return WebhookConfig{}, ErrNotFound
	}
	if err != nil {
		return WebhookConfig{}, fmt.Errorf("webhook: scan: %w", err)
	}
	return w, nil
}

func scanConfigRows(rows pgx.Rows) ([]WebhookConfig, error) {
	var items []WebhookConfig
	for rows.Next() {
		var w WebhookConfig
		if err := rows.Scan(&w.ID, &w.Name, &w.URL, &w.Events, &w.Secret, &w.Enabled, &w.RetryCount, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("webhook: scan row: %w", err)
		}
		items = append(items, w)
	}
	return items, rows.Err()
}

func scanDeliveryRow(row pgx.Row) (WebhookDeliveryLog, error) {
	var d WebhookDeliveryLog
	var deliveredAt *time.Time
	err := row.Scan(&d.ID, &d.WebhookID, &d.EventType, &d.Payload, &d.Status,
		&d.ResponseStatus, &d.ResponseBody, &d.Attempts, &deliveredAt, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return WebhookDeliveryLog{}, ErrNotFound
	}
	if err != nil {
		return WebhookDeliveryLog{}, fmt.Errorf("webhook: scan delivery: %w", err)
	}
	d.DeliveredAt = deliveredAt
	return d, nil
}

func scanDeliveryRows(rows pgx.Rows) ([]WebhookDeliveryLog, error) {
	var items []WebhookDeliveryLog
	for rows.Next() {
		var d WebhookDeliveryLog
		var deliveredAt *time.Time
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.EventType, &d.Payload, &d.Status,
			&d.ResponseStatus, &d.ResponseBody, &d.Attempts, &deliveredAt, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("webhook: scan delivery row: %w", err)
		}
		d.DeliveredAt = deliveredAt
		items = append(items, d)
	}
	return items, rows.Err()
}
