// Package task provides persistence for coordinator tasks and their worker notifications.
// These records are written through by CoordinatorState as tasks are created and
// completed, enabling UI tracking and post-execution history.
package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// Task is the persisted representation of a CoordinatorTask.
type Task struct {
	ID          string
	SessionID   uuid.UUID
	Description string
	Status      string
	Phase       string
	DependsOn   []string
	AssignedTo  string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// Notification is the persisted representation of a TaskNotification.
type Notification struct {
	ID         string
	TaskID     string
	WorkerID   string
	WorkerName string
	Status     string
	Summary    string
	Findings   json.RawMessage
	Error      *string
	CreatedAt  time.Time
}

// Repository defines persistence operations for coordinator tasks and notifications.
type Repository interface {
	// CreateTask persists a new coordinator task.
	CreateTask(ctx context.Context, t Task) error
	// UpdateTask updates a task's status, assignedTo and completedAt.
	UpdateTask(ctx context.Context, t Task) error
	// GetTask returns a task by ID.
	GetTask(ctx context.Context, id string) (Task, error)
	// ListBySession returns a paginated list of tasks for a chat session,
	// ordered by created_at ascending.
	ListBySession(ctx context.Context, sessionID uuid.UUID, req pagination.PageRequest) ([]Task, int64, error)
	// CreateNotification persists a task notification from a worker.
	CreateNotification(ctx context.Context, n Notification) error
	// ListNotificationsByTask returns all notifications for a task, ordered by created_at.
	ListNotificationsByTask(ctx context.Context, taskID string) ([]Notification, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL-backed task Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) CreateTask(ctx context.Context, t Task) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()
	dependsOn := t.DependsOn
	if dependsOn == nil {
		dependsOn = []string{}
	}

	_, err = conn.Exec(ctx,
		`INSERT INTO coordinator_task (id, session_id, description, status, phase, depends_on, assigned_to, created_at, completed_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (id) DO NOTHING`,
		t.ID, t.SessionID, t.Description, t.Status, t.Phase, dependsOn,
		nullString(t.AssignedTo), t.CreatedAt, t.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("task: create: %w", err)
	}
	return nil
}

func (r *pgRepository) UpdateTask(ctx context.Context, t Task) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	_, err = conn.Exec(ctx,
		`UPDATE coordinator_task
		 SET status=$1, assigned_to=$2, completed_at=$3
		 WHERE id=$4`,
		t.Status, nullString(t.AssignedTo), t.CompletedAt, t.ID,
	)
	if err != nil {
		return fmt.Errorf("task: update: %w", err)
	}
	return nil
}

func (r *pgRepository) GetTask(ctx context.Context, id string) (Task, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return Task{}, err
	}
	defer release()

	var t Task
	var assignedTo *string
	err = conn.QueryRow(ctx,
		`SELECT id, session_id, description, status, phase, depends_on, assigned_to, created_at, completed_at
		 FROM coordinator_task WHERE id=$1`, id,
	).Scan(&t.ID, &t.SessionID, &t.Description, &t.Status, &t.Phase, &t.DependsOn,
		&assignedTo, &t.CreatedAt, &t.CompletedAt)
	if err != nil {
		return Task{}, fmt.Errorf("task: get: %w", err)
	}
	if assignedTo != nil {
		t.AssignedTo = *assignedTo
	}
	return t, nil
}

func (r *pgRepository) ListBySession(ctx context.Context, sessionID uuid.UUID, req pagination.PageRequest) ([]Task, int64, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var total int64
	if err := conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM coordinator_task WHERE session_id=$1`, sessionID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("task: list count: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, session_id, description, status, phase, depends_on, assigned_to, created_at, completed_at
		 FROM coordinator_task
		 WHERE session_id=$1
		 ORDER BY created_at ASC
		 LIMIT $2 OFFSET $3`,
		sessionID, req.Size, req.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("task: list: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var assignedTo *string
		if err := rows.Scan(&t.ID, &t.SessionID, &t.Description, &t.Status, &t.Phase, &t.DependsOn,
			&assignedTo, &t.CreatedAt, &t.CompletedAt); err != nil {
			return nil, 0, fmt.Errorf("task: list scan: %w", err)
		}
		if assignedTo != nil {
			t.AssignedTo = *assignedTo
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return tasks, total, rows.Err()
}

func (r *pgRepository) CreateNotification(ctx context.Context, n Notification) error {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	_, err = conn.Exec(ctx,
		`INSERT INTO task_notification (id, task_id, worker_id, worker_name, status, summary, findings, error, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (id) DO NOTHING`,
		n.ID, n.TaskID, n.WorkerID, n.WorkerName, n.Status, n.Summary,
		nullJSON(n.Findings), n.Error, n.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("task: create notification: %w", err)
	}
	return nil
}

func (r *pgRepository) ListNotificationsByTask(ctx context.Context, taskID string) ([]Notification, error) {
	tenantID := tenant.FromContext(ctx)
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	rows, err := conn.Query(ctx,
		`SELECT id, task_id, worker_id, worker_name, status, summary, findings, error, created_at
		 FROM task_notification
		 WHERE task_id=$1
		 ORDER BY created_at ASC`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("task: list notifications: %w", err)
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.TaskID, &n.WorkerID, &n.WorkerName, &n.Status, &n.Summary,
			&n.Findings, &n.Error, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("task: list notifications scan: %w", err)
		}
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []Notification{}
	}
	return notifications, rows.Err()
}

// nullString converts an empty string to nil for nullable TEXT columns.
func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nullJSON returns nil when the JSON is empty.
func nullJSON(b json.RawMessage) *json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return &b
}
