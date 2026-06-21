package agentic_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/task"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// --- mock task.Repository ---

type mockPersistRepo struct {
	tasks         map[string]task.Task
	notifications []task.Notification
	createErr     error // if set, CreateTask returns this error
}

func newMockPersistRepo() *mockPersistRepo {
	return &mockPersistRepo{tasks: make(map[string]task.Task)}
}

func (m *mockPersistRepo) CreateTask(_ context.Context, t task.Task) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.tasks[t.ID] = t
	return nil
}
func (m *mockPersistRepo) UpdateTask(_ context.Context, t task.Task) error {
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
func (m *mockPersistRepo) GetTask(_ context.Context, id string) (task.Task, error) {
	return m.tasks[id], nil
}
func (m *mockPersistRepo) ListBySession(_ context.Context, sessionID uuid.UUID, _ pagination.PageRequest) ([]task.Task, int64, error) {
	var out []task.Task
	for _, t := range m.tasks {
		if t.SessionID == sessionID {
			out = append(out, t)
		}
	}
	return out, int64(len(out)), nil
}
func (m *mockPersistRepo) CreateNotification(_ context.Context, n task.Notification) error {
	m.notifications = append(m.notifications, n)
	return nil
}
func (m *mockPersistRepo) ListNotificationsByTask(_ context.Context, _ string) ([]task.Notification, error) {
	return m.notifications, nil
}

// Compile-time interface check.
var _ task.Repository = (*mockPersistRepo)(nil)

// --- Tests ---

func TestCoordinatorState_PersistsOnCreate(t *testing.T) {
	repo := newMockPersistRepo()
	sessionID := uuid.New()
	cs := agentic.NewCoordinatorState().
		WithRepository(repo, sessionID).
		WithContext(context.Background())

	taskID := cs.CreateTask("Research the codebase", agentic.PhaseResearch, nil)
	require.NotEmpty(t, taskID)

	persisted, ok := repo.tasks[taskID]
	require.True(t, ok, "task should be persisted to repo")
	assert.Equal(t, "pending", persisted.Status)
	assert.Equal(t, sessionID, persisted.SessionID)
	assert.Equal(t, "research", persisted.Phase)
}

func TestCoordinatorState_UpdatesStatusOnComplete(t *testing.T) {
	repo := newMockPersistRepo()
	cs := agentic.NewCoordinatorState().
		WithRepository(repo, uuid.New()).
		WithContext(context.Background())

	taskID := cs.CreateTask("Implement feature", agentic.PhaseImplementation, nil)
	require.NoError(t, cs.CompleteTask(taskID))

	persisted := repo.tasks[taskID]
	assert.Equal(t, "completed", persisted.Status)
	require.NotNil(t, persisted.CompletedAt)
}

func TestCoordinatorState_HydratesPersistedTasksOnRestart(t *testing.T) {
	repo := newMockPersistRepo()
	sessionID := uuid.New()
	otherSessionID := uuid.New()
	dependency := task.Task{
		ID:          "task-research",
		SessionID:   sessionID,
		Description: "Research",
		Status:      "completed",
		Phase:       "research",
	}
	repo.tasks[dependency.ID] = dependency
	repo.tasks["task-implementation"] = task.Task{
		ID:          "task-implementation",
		SessionID:   sessionID,
		Description: "Implement",
		Status:      "pending",
		Phase:       "implementation",
		DependsOn:   []string{dependency.ID},
	}
	repo.tasks["task-other-session"] = task.Task{
		ID:        "task-other-session",
		SessionID: otherSessionID,
		Status:    "pending",
		Phase:     "research",
	}

	restarted := agentic.NewCoordinatorState().
		WithRepository(repo, sessionID).
		WithContext(context.Background())

	ready := restarted.ReadyTasks()
	require.Len(t, ready, 1)
	assert.Equal(t, "task-implementation", ready[0].ID)
	assert.Equal(t, agentic.TaskStatusPending, ready[0].Status)
	assert.Equal(t, agentic.PhaseImplementation, ready[0].Phase)
	assert.Equal(t, "Tasks: 2 total, 1 pending, 1 completed. Workers: 0", restarted.Summary())
}

func TestCoordinatorState_WorksIfRepoBroken(t *testing.T) {
	repo := newMockPersistRepo()
	repo.createErr = assert.AnError // simulate DB failure
	cs := agentic.NewCoordinatorState().
		WithRepository(repo, uuid.New()).
		WithContext(context.Background())

	// CreateTask must succeed in-memory even if persistence fails.
	taskID := cs.CreateTask("Urgent task", agentic.PhaseResearch, nil)
	require.NotEmpty(t, taskID, "in-memory task should still be created")

	// The task should be in the coordinator's in-memory state.
	ready := cs.ReadyTasks()
	require.Len(t, ready, 1)
	assert.Equal(t, taskID, ready[0].ID)

	// Repo was broken — task was not persisted.
	assert.Empty(t, repo.tasks)
}

func TestCoordinatorState_NoRepo_NoOp(t *testing.T) {
	// Without a repository, CoordinatorState operates fully in-memory without error.
	cs := agentic.NewCoordinatorState()
	taskID := cs.CreateTask("Pure in-memory task", agentic.PhaseSynthesis, nil)
	require.NotEmpty(t, taskID)
	require.NoError(t, cs.CompleteTask(taskID))
}
