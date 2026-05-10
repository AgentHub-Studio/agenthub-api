package trigger

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

// TriggerRepository defines the persistence interface for agent triggers.
type TriggerRepository interface {
	Create(ctx context.Context, t AgentTrigger) (AgentTrigger, error)
	ExistsByName(ctx context.Context, agentID uuid.UUID, name string) (bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (AgentTrigger, error)
	ListByAgent(ctx context.Context, agentID uuid.UUID, page pagination.PageRequest) (pagination.Page[AgentTrigger], error)
	Update(ctx context.Context, t AgentTrigger) (AgentTrigger, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// ListDue returns triggers where next_run_at <= now and enabled = true.
	ListDue(ctx context.Context, now time.Time, limit int) ([]AgentTrigger, error)
	// MarkRun updates last_run_at, next_run_at, and increments run_count.
	MarkRun(ctx context.Context, id uuid.UUID, lastRun, nextRun time.Time) error
	// CreateRun persists a trigger run record.
	CreateRun(ctx context.Context, run AgentTriggerRun) (AgentTriggerRun, error)
	// CompleteRun updates a run with final status.
	CompleteRun(ctx context.Context, runID uuid.UUID, status RunStatus, turns, tokens *int, errMsg *string) error
	// CompleteRunBySession updates the running trigger_run linked to a chat session.
	// Bug 291: chat run finishes asynchronously and only knows sessionID; this lets the
	// run-end hook close the trigger_run without a separate sessionID→runID lookup.
	CompleteRunBySession(ctx context.Context, sessionID uuid.UUID, status RunStatus, turns, tokens *int, errMsg *string) error
	// ListRuns returns paginated runs for a trigger.
	ListRuns(ctx context.Context, triggerID uuid.UUID, page pagination.PageRequest) (pagination.Page[AgentTriggerRun], error)
}

// Repository provides data access for agent triggers.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new trigger Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const triggerColumns = `id, agent_id, name, cron_expression, enabled, input_template,
	last_run_at, next_run_at, run_count, created_at, updated_at`

const runColumns = `id, trigger_id, session_id, status, started_at, completed_at,
	total_turns, total_tokens, error`

func (r *Repository) Create(ctx context.Context, t AgentTrigger) (AgentTrigger, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return AgentTrigger{}, err
	}
	defer release()

	query := `
		INSERT INTO agent_trigger (agent_id, name, cron_expression, enabled, input_template, next_run_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + triggerColumns

	row := conn.QueryRow(ctx, query, t.AgentID, t.Name, t.CronExpression, t.Enabled, t.InputTemplate, t.NextRunAt)
	created, err := scanTrigger(row)
	if err != nil {
		msg := err.Error()
		for i := 0; i+5 <= len(msg); i++ {
			if msg[i:i+5] == "23503" {
				return AgentTrigger{}, ErrAgentNotFound
			}
		}
	}
	return created, err
}

// ExistsByName checks if a trigger with the given name exists for the agent.
func (r *Repository) ExistsByName(ctx context.Context, agentID uuid.UUID, name string) (bool, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return false, err
	}
	defer release()

	var exists bool
	err = conn.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM agent_trigger WHERE agent_id = $1 AND name = $2)`,
		agentID, name,
	).Scan(&exists)
	return exists, err
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (AgentTrigger, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return AgentTrigger{}, err
	}
	defer release()

	query := `SELECT ` + triggerColumns + ` FROM agent_trigger WHERE id = $1`
	row := conn.QueryRow(ctx, query, id)
	return scanTrigger(row)
}

func (r *Repository) ListByAgent(ctx context.Context, agentID uuid.UUID, page pagination.PageRequest) (pagination.Page[AgentTrigger], error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return pagination.Page[AgentTrigger]{}, err
	}
	defer release()

	countQuery := `SELECT COUNT(*) FROM agent_trigger WHERE agent_id = $1`
	var total int64
	if err := conn.QueryRow(ctx, countQuery, agentID).Scan(&total); err != nil {
		return pagination.Page[AgentTrigger]{}, fmt.Errorf("trigger: count: %w", err)
	}

	query := `SELECT ` + triggerColumns + `
		FROM agent_trigger
		WHERE agent_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := conn.Query(ctx, query, agentID, page.Size, page.Offset())
	if err != nil {
		return pagination.Page[AgentTrigger]{}, fmt.Errorf("trigger: list: %w", err)
	}
	defer rows.Close()

	items, err := scanTriggers(rows)
	if err != nil {
		return pagination.Page[AgentTrigger]{}, err
	}

	return pagination.NewPage(items, total, page), nil
}

func (r *Repository) Update(ctx context.Context, t AgentTrigger) (AgentTrigger, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return AgentTrigger{}, err
	}
	defer release()

	query := `
		UPDATE agent_trigger
		   SET name = $2, cron_expression = $3, enabled = $4,
		       input_template = $5, next_run_at = $6, updated_at = NOW()
		 WHERE id = $1
		RETURNING ` + triggerColumns

	row := conn.QueryRow(ctx, query, t.ID, t.Name, t.CronExpression, t.Enabled, t.InputTemplate, t.NextRunAt)
	return scanTrigger(row)
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	ct, err := conn.Exec(ctx, `DELETE FROM agent_trigger WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("trigger: delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ListDue(ctx context.Context, now time.Time, limit int) ([]AgentTrigger, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	query := `SELECT ` + triggerColumns + `
		FROM agent_trigger
		WHERE enabled = true AND next_run_at IS NOT NULL AND next_run_at <= $1
		ORDER BY next_run_at ASC
		LIMIT $2`

	rows, err := conn.Query(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("trigger: list due: %w", err)
	}
	defer rows.Close()

	return scanTriggers(rows)
}

func (r *Repository) MarkRun(ctx context.Context, id uuid.UUID, lastRun, nextRun time.Time) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	_, err = conn.Exec(ctx, `
		UPDATE agent_trigger
		   SET last_run_at = $2, next_run_at = $3, run_count = run_count + 1, updated_at = NOW()
		 WHERE id = $1`, id, lastRun, nextRun)
	if err != nil {
		return fmt.Errorf("trigger: mark run: %w", err)
	}
	return nil
}

func (r *Repository) CreateRun(ctx context.Context, run AgentTriggerRun) (AgentTriggerRun, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return AgentTriggerRun{}, err
	}
	defer release()

	query := `
		INSERT INTO agent_trigger_run (trigger_id, session_id, status)
		VALUES ($1, $2, $3)
		RETURNING ` + runColumns

	row := conn.QueryRow(ctx, query, run.TriggerID, run.SessionID, run.Status)
	return scanRun(row)
}

func (r *Repository) CompleteRun(ctx context.Context, runID uuid.UUID, status RunStatus, turns, tokens *int, errMsg *string) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	_, err = conn.Exec(ctx, `
		UPDATE agent_trigger_run
		   SET status = $2, completed_at = NOW(), total_turns = $3, total_tokens = $4, error = $5
		 WHERE id = $1`, runID, status, turns, tokens, errMsg)
	if err != nil {
		return fmt.Errorf("trigger: complete run: %w", err)
	}
	return nil
}

func (r *Repository) CompleteRunBySession(ctx context.Context, sessionID uuid.UUID, status RunStatus, turns, tokens *int, errMsg *string) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	// Only update the most recent running run for this session — protects against
	// stale rows if the session is somehow reused.
	_, err = conn.Exec(ctx, `
		UPDATE agent_trigger_run
		   SET status = $2, completed_at = NOW(), total_turns = $3, total_tokens = $4, error = $5
		 WHERE id = (
		     SELECT id FROM agent_trigger_run
		      WHERE session_id = $1 AND status = 'running'
		      ORDER BY started_at DESC
		      LIMIT 1
		 )`, sessionID, status, turns, tokens, errMsg)
	if err != nil {
		return fmt.Errorf("trigger: complete run by session: %w", err)
	}
	return nil
}

func (r *Repository) ListRuns(ctx context.Context, triggerID uuid.UUID, page pagination.PageRequest) (pagination.Page[AgentTriggerRun], error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return pagination.Page[AgentTriggerRun]{}, err
	}
	defer release()

	countQuery := `SELECT COUNT(*) FROM agent_trigger_run WHERE trigger_id = $1`
	var total int64
	if err := conn.QueryRow(ctx, countQuery, triggerID).Scan(&total); err != nil {
		return pagination.Page[AgentTriggerRun]{}, fmt.Errorf("trigger: count runs: %w", err)
	}

	query := `SELECT ` + runColumns + `
		FROM agent_trigger_run
		WHERE trigger_id = $1
		ORDER BY started_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := conn.Query(ctx, query, triggerID, page.Size, page.Offset())
	if err != nil {
		return pagination.Page[AgentTriggerRun]{}, fmt.Errorf("trigger: list runs: %w", err)
	}
	defer rows.Close()

	var items []AgentTriggerRun
	for rows.Next() {
		r, err := scanRunRow(rows)
		if err != nil {
			return pagination.Page[AgentTriggerRun]{}, err
		}
		items = append(items, r)
	}
	if items == nil {
		items = []AgentTriggerRun{}
	}

	return pagination.NewPage(items, total, page), nil
}

// --- scan helpers ---

func scanTrigger(row pgx.Row) (AgentTrigger, error) {
	var t AgentTrigger
	err := row.Scan(&t.ID, &t.AgentID, &t.Name, &t.CronExpression, &t.Enabled,
		&t.InputTemplate, &t.LastRunAt, &t.NextRunAt, &t.RunCount, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentTrigger{}, ErrNotFound
	}
	if err != nil {
		return AgentTrigger{}, fmt.Errorf("trigger: scan: %w", err)
	}
	return t, nil
}

func scanTriggers(rows pgx.Rows) ([]AgentTrigger, error) {
	var items []AgentTrigger
	for rows.Next() {
		var t AgentTrigger
		if err := rows.Scan(&t.ID, &t.AgentID, &t.Name, &t.CronExpression, &t.Enabled,
			&t.InputTemplate, &t.LastRunAt, &t.NextRunAt, &t.RunCount, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("trigger: scan row: %w", err)
		}
		items = append(items, t)
	}
	if items == nil {
		items = []AgentTrigger{}
	}
	return items, rows.Err()
}

func scanRun(row pgx.Row) (AgentTriggerRun, error) {
	var r AgentTriggerRun
	err := row.Scan(&r.ID, &r.TriggerID, &r.SessionID, &r.Status,
		&r.StartedAt, &r.CompletedAt, &r.TotalTurns, &r.TotalTokens, &r.Error)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentTriggerRun{}, ErrNotFound
	}
	if err != nil {
		return AgentTriggerRun{}, fmt.Errorf("trigger run: scan: %w", err)
	}
	return r, nil
}

func scanRunRow(rows pgx.Rows) (AgentTriggerRun, error) {
	var r AgentTriggerRun
	if err := rows.Scan(&r.ID, &r.TriggerID, &r.SessionID, &r.Status,
		&r.StartedAt, &r.CompletedAt, &r.TotalTurns, &r.TotalTokens, &r.Error); err != nil {
		return AgentTriggerRun{}, fmt.Errorf("trigger run: scan row: %w", err)
	}
	return r, nil
}

// InputTemplateFromRaw safely converts json.RawMessage to a string.
func InputTemplateFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return string(raw)
	}
	return s
}
