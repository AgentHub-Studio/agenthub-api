package trigger_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/trigger"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// --- mock repository ---

type mockRepo struct {
	triggers map[uuid.UUID]trigger.AgentTrigger
	runs     map[uuid.UUID]trigger.AgentTriggerRun
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		triggers: make(map[uuid.UUID]trigger.AgentTrigger),
		runs:     make(map[uuid.UUID]trigger.AgentTriggerRun),
	}
}

func (m *mockRepo) Create(_ context.Context, t trigger.AgentTrigger) (trigger.AgentTrigger, error) {
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	m.triggers[t.ID] = t
	return t, nil
}

func (m *mockRepo) GetByID(_ context.Context, id uuid.UUID) (trigger.AgentTrigger, error) {
	t, ok := m.triggers[id]
	if !ok {
		return trigger.AgentTrigger{}, trigger.ErrNotFound
	}
	return t, nil
}

func (m *mockRepo) ListByAgent(_ context.Context, agentID uuid.UUID, page pagination.PageRequest) (pagination.Page[trigger.AgentTrigger], error) {
	var items []trigger.AgentTrigger
	for _, t := range m.triggers {
		if t.AgentID == agentID {
			items = append(items, t)
		}
	}
	if items == nil {
		items = []trigger.AgentTrigger{}
	}
	return pagination.NewPage(items, int64(len(items)), page), nil
}

func (m *mockRepo) Update(_ context.Context, t trigger.AgentTrigger) (trigger.AgentTrigger, error) {
	if _, ok := m.triggers[t.ID]; !ok {
		return trigger.AgentTrigger{}, trigger.ErrNotFound
	}
	t.UpdatedAt = time.Now()
	m.triggers[t.ID] = t
	return t, nil
}

func (m *mockRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.triggers[id]; !ok {
		return trigger.ErrNotFound
	}
	delete(m.triggers, id)
	return nil
}

func (m *mockRepo) ListDue(_ context.Context, _ time.Time, _ int) ([]trigger.AgentTrigger, error) {
	return nil, nil
}

func (m *mockRepo) MarkRun(_ context.Context, _ uuid.UUID, _, _ time.Time) error {
	return nil
}

func (m *mockRepo) CreateRun(_ context.Context, run trigger.AgentTriggerRun) (trigger.AgentTriggerRun, error) {
	run.ID = uuid.New()
	run.StartedAt = time.Now()
	m.runs[run.ID] = run
	return run, nil
}

func (m *mockRepo) CompleteRun(_ context.Context, runID uuid.UUID, status trigger.RunStatus, turns, tokens *int, errMsg *string) error {
	r, ok := m.runs[runID]
	if !ok {
		return trigger.ErrNotFound
	}
	r.Status = status
	r.TotalTurns = turns
	r.TotalTokens = tokens
	r.Error = errMsg
	now := time.Now()
	r.CompletedAt = &now
	m.runs[runID] = r
	return nil
}

func (m *mockRepo) ListRuns(_ context.Context, triggerID uuid.UUID, page pagination.PageRequest) (pagination.Page[trigger.AgentTriggerRun], error) {
	var items []trigger.AgentTriggerRun
	for _, r := range m.runs {
		if r.TriggerID == triggerID {
			items = append(items, r)
		}
	}
	if items == nil {
		items = []trigger.AgentTriggerRun{}
	}
	return pagination.NewPage(items, int64(len(items)), page), nil
}

// --- mock cron parser ---

type mockCron struct {
	nextTime    time.Time
	nextErr     error
	validateErr error
}

func (c *mockCron) NextRun(_ string, _ time.Time) (time.Time, error) {
	return c.nextTime, c.nextErr
}

func (c *mockCron) Validate(_ string) error {
	return c.validateErr
}

// --- tests ---

func TestService_Create_Success(t *testing.T) {
	repo := newMockRepo()
	nextRun := time.Now().Add(time.Hour)
	cron := &mockCron{nextTime: nextRun}
	svc := trigger.NewService(repo, cron)

	agentID := uuid.New()
	result, err := svc.Create(context.Background(), agentID, trigger.CreateTriggerRequest{
		Name:           "Daily report",
		CronExpression: "0 9 * * *",
	})
	require.NoError(t, err)
	assert.Equal(t, "Daily report", result.Name)
	assert.Equal(t, "0 9 * * *", result.CronExpression)
	assert.True(t, result.Enabled)
	assert.NotNil(t, result.NextRunAt)
}

func TestService_Create_MissingName(t *testing.T) {
	svc := trigger.NewService(newMockRepo(), &mockCron{})
	_, err := svc.Create(context.Background(), uuid.New(), trigger.CreateTriggerRequest{
		CronExpression: "0 9 * * *",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestService_Create_MissingCron(t *testing.T) {
	svc := trigger.NewService(newMockRepo(), &mockCron{})
	_, err := svc.Create(context.Background(), uuid.New(), trigger.CreateTriggerRequest{
		Name: "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cron expression is required")
}

func TestService_Create_InvalidCron(t *testing.T) {
	cron := &mockCron{validateErr: assert.AnError}
	svc := trigger.NewService(newMockRepo(), cron)
	_, err := svc.Create(context.Background(), uuid.New(), trigger.CreateTriggerRequest{
		Name:           "test",
		CronExpression: "invalid",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cron expression")
}

func TestService_Create_Disabled(t *testing.T) {
	cron := &mockCron{nextTime: time.Now().Add(time.Hour)}
	svc := trigger.NewService(newMockRepo(), cron)

	enabled := false
	result, err := svc.Create(context.Background(), uuid.New(), trigger.CreateTriggerRequest{
		Name:           "test",
		CronExpression: "0 9 * * *",
		Enabled:        &enabled,
	})
	require.NoError(t, err)
	assert.False(t, result.Enabled)
	assert.Nil(t, result.NextRunAt) // disabled triggers have no next run
}

func TestService_GetByID_Success(t *testing.T) {
	repo := newMockRepo()
	cron := &mockCron{nextTime: time.Now().Add(time.Hour)}
	svc := trigger.NewService(repo, cron)

	agentID := uuid.New()
	created, _ := svc.Create(context.Background(), agentID, trigger.CreateTriggerRequest{
		Name: "test", CronExpression: "0 * * * *",
	})

	result, err := svc.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, result.ID)
}

func TestService_GetByID_NotFound(t *testing.T) {
	svc := trigger.NewService(newMockRepo(), &mockCron{})
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, trigger.ErrNotFound)
}

func TestService_Update_Success(t *testing.T) {
	repo := newMockRepo()
	cron := &mockCron{nextTime: time.Now().Add(time.Hour)}
	svc := trigger.NewService(repo, cron)

	agentID := uuid.New()
	created, _ := svc.Create(context.Background(), agentID, trigger.CreateTriggerRequest{
		Name: "original", CronExpression: "0 * * * *",
	})

	newName := "updated"
	result, err := svc.Update(context.Background(), created.ID, trigger.UpdateTriggerRequest{
		Name: &newName,
	})
	require.NoError(t, err)
	assert.Equal(t, "updated", result.Name)
}

func TestService_Update_NotFound(t *testing.T) {
	svc := trigger.NewService(newMockRepo(), &mockCron{})
	_, err := svc.Update(context.Background(), uuid.New(), trigger.UpdateTriggerRequest{})
	require.ErrorIs(t, err, trigger.ErrNotFound)
}

func TestService_Delete_Success(t *testing.T) {
	repo := newMockRepo()
	cron := &mockCron{nextTime: time.Now().Add(time.Hour)}
	svc := trigger.NewService(repo, cron)

	agentID := uuid.New()
	created, _ := svc.Create(context.Background(), agentID, trigger.CreateTriggerRequest{
		Name: "test", CronExpression: "0 * * * *",
	})

	require.NoError(t, svc.Delete(context.Background(), created.ID))
	_, err := svc.GetByID(context.Background(), created.ID)
	require.ErrorIs(t, err, trigger.ErrNotFound)
}

func TestService_Delete_NotFound(t *testing.T) {
	svc := trigger.NewService(newMockRepo(), &mockCron{})
	err := svc.Delete(context.Background(), uuid.New())
	require.ErrorIs(t, err, trigger.ErrNotFound)
}

func TestService_List(t *testing.T) {
	repo := newMockRepo()
	cron := &mockCron{nextTime: time.Now().Add(time.Hour)}
	svc := trigger.NewService(repo, cron)

	agentID := uuid.New()
	svc.Create(context.Background(), agentID, trigger.CreateTriggerRequest{
		Name: "t1", CronExpression: "0 * * * *",
	})
	svc.Create(context.Background(), agentID, trigger.CreateTriggerRequest{
		Name: "t2", CronExpression: "*/5 * * * *",
	})

	page, err := svc.List(context.Background(), agentID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(2), page.TotalElements)
}

func TestService_StartRun(t *testing.T) {
	repo := newMockRepo()
	svc := trigger.NewService(repo, &mockCron{nextTime: time.Now().Add(time.Hour)})

	triggerID := uuid.New()
	sessionID := uuid.New()
	run, err := svc.StartRun(context.Background(), triggerID, sessionID)
	require.NoError(t, err)
	assert.Equal(t, trigger.RunStatusRunning, run.Status)
	assert.Equal(t, sessionID, run.SessionID)
}

func TestService_CompleteRun(t *testing.T) {
	repo := newMockRepo()
	svc := trigger.NewService(repo, &mockCron{nextTime: time.Now().Add(time.Hour)})

	run, _ := svc.StartRun(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, svc.CompleteRun(context.Background(), run.ID, 3, 1500))

	stored := repo.runs[run.ID]
	assert.Equal(t, trigger.RunStatusCompleted, stored.Status)
	assert.Equal(t, 3, *stored.TotalTurns)
	assert.Equal(t, 1500, *stored.TotalTokens)
}

func TestService_FailRun(t *testing.T) {
	repo := newMockRepo()
	svc := trigger.NewService(repo, &mockCron{nextTime: time.Now().Add(time.Hour)})

	run, _ := svc.StartRun(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, svc.FailRun(context.Background(), run.ID, "timeout"))

	stored := repo.runs[run.ID]
	assert.Equal(t, trigger.RunStatusFailed, stored.Status)
	assert.Equal(t, "timeout", *stored.Error)
}

func TestService_Create_WithInputTemplate(t *testing.T) {
	repo := newMockRepo()
	cron := &mockCron{nextTime: time.Now().Add(time.Hour)}
	svc := trigger.NewService(repo, cron)

	tmpl := json.RawMessage(`{"message":"Generate daily report"}`)
	result, err := svc.Create(context.Background(), uuid.New(), trigger.CreateTriggerRequest{
		Name:           "daily",
		CronExpression: "0 9 * * *",
		InputTemplate:  tmpl,
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"message":"Generate daily report"}`, string(result.InputTemplate))
}
