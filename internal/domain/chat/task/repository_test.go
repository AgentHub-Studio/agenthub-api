package task_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/task"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockTaskRepo is a minimal in-memory Repository for unit testing.
type mockTaskRepo struct {
	tasks         map[string]task.Task
	notifications map[string][]task.Notification // taskID → []Notification
}

func newMockTaskRepo() *mockTaskRepo {
	return &mockTaskRepo{
		tasks:         make(map[string]task.Task),
		notifications: make(map[string][]task.Notification),
	}
}

func (m *mockTaskRepo) CreateTask(_ context.Context, t task.Task) error {
	m.tasks[t.ID] = t
	return nil
}

func (m *mockTaskRepo) UpdateTask(_ context.Context, t task.Task) error {
	existing, ok := m.tasks[t.ID]
	if !ok {
		return nil
	}
	existing.Status = t.Status
	existing.AssignedTo = t.AssignedTo
	existing.CompletedAt = t.CompletedAt
	m.tasks[t.ID] = existing
	return nil
}

func (m *mockTaskRepo) GetTask(_ context.Context, id string) (task.Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return task.Task{}, nil
	}
	return t, nil
}

func (m *mockTaskRepo) ListBySession(_ context.Context, sessionID uuid.UUID, req pagination.PageRequest) ([]task.Task, int64, error) {
	var out []task.Task
	for _, t := range m.tasks {
		if t.SessionID == sessionID {
			out = append(out, t)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockTaskRepo) CreateNotification(_ context.Context, n task.Notification) error {
	m.notifications[n.TaskID] = append(m.notifications[n.TaskID], n)
	return nil
}

func (m *mockTaskRepo) ListNotificationsByTask(_ context.Context, taskID string) ([]task.Notification, error) {
	return m.notifications[taskID], nil
}

// Verify mockTaskRepo satisfies the interface at compile time.
var _ task.Repository = (*mockTaskRepo)(nil)

// --- Model tests ---

func TestTask_ZeroValue(t *testing.T) {
	var tk task.Task
	assert.Equal(t, "", tk.ID)
	assert.Equal(t, uuid.Nil, tk.SessionID)
	assert.Nil(t, tk.CompletedAt)
}

func TestNotification_ZeroValue(t *testing.T) {
	var n task.Notification
	assert.Equal(t, "", n.ID)
	assert.Nil(t, n.Error)
	assert.Nil(t, n.Findings)
}

func TestTask_RoundTrip(t *testing.T) {
	sessionID := uuid.New()
	tk := task.Task{
		ID:          "task-1",
		SessionID:   sessionID,
		Description: "Research the codebase",
		Status:      "pending",
		Phase:       "research",
		DependsOn:   []string{},
	}
	assert.Equal(t, "task-1", tk.ID)
	assert.Equal(t, sessionID, tk.SessionID)
	assert.Equal(t, "research", tk.Phase)
}

// --- Repository mock tests ---

func TestMockTaskRepo_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	repo := newMockTaskRepo()
	sessionID := uuid.New()

	tk := task.Task{
		ID:          "task-abc",
		SessionID:   sessionID,
		Description: "Write tests",
		Status:      "pending",
		Phase:       "implementation",
		CreatedAt:   time.Now(),
	}
	require.NoError(t, repo.CreateTask(ctx, tk))

	got, err := repo.GetTask(ctx, "task-abc")
	require.NoError(t, err)
	assert.Equal(t, "task-abc", got.ID)
	assert.Equal(t, "pending", got.Status)
}

func TestMockTaskRepo_UpdateTask(t *testing.T) {
	ctx := context.Background()
	repo := newMockTaskRepo()
	sessionID := uuid.New()

	tk := task.Task{
		ID:        "task-xyz",
		SessionID: sessionID,
		Status:    "pending",
		Phase:     "research",
		CreatedAt: time.Now(),
	}
	require.NoError(t, repo.CreateTask(ctx, tk))

	now := time.Now()
	require.NoError(t, repo.UpdateTask(ctx, task.Task{
		ID:          "task-xyz",
		Status:      "completed",
		CompletedAt: &now,
	}))

	got, _ := repo.GetTask(ctx, "task-xyz")
	assert.Equal(t, "completed", got.Status)
	require.NotNil(t, got.CompletedAt)
}

func TestMockTaskRepo_ListBySession(t *testing.T) {
	ctx := context.Background()
	repo := newMockTaskRepo()
	sessionID := uuid.New()
	otherSessionID := uuid.New()

	for i, phase := range []string{"research", "implementation"} {
		tk := task.Task{
			ID:        fmt.Sprintf("task-%d", i),
			SessionID: sessionID,
			Phase:     phase,
			Status:    "pending",
			CreatedAt: time.Now(),
		}
		require.NoError(t, repo.CreateTask(ctx, tk))
	}
	// task in another session — should not appear
	require.NoError(t, repo.CreateTask(ctx, task.Task{
		ID:        "task-other",
		SessionID: otherSessionID,
		Status:    "pending",
		CreatedAt: time.Now(),
	}))

	tasks, total, err := repo.ListBySession(ctx, sessionID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, tasks, 2)
}

func TestMockTaskRepo_CreateAndListNotifications(t *testing.T) {
	ctx := context.Background()
	repo := newMockTaskRepo()

	n := task.Notification{
		ID:       "notif-1",
		TaskID:   "task-1",
		WorkerID: "worker-abc",
		Status:   "completed",
		Summary:  "Done!",
	}
	require.NoError(t, repo.CreateNotification(ctx, n))

	notifs, err := repo.ListNotificationsByTask(ctx, "task-1")
	require.NoError(t, err)
	require.Len(t, notifs, 1)
	assert.Equal(t, "worker-abc", notifs[0].WorkerID)
}
