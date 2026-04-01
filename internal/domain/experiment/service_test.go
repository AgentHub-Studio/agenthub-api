package experiment_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/experiment"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockExperimentRepo struct {
	experiments map[uuid.UUID]experiment.PromptExperiment
	results     []experiment.ExperimentResult
}

func newMockRepo() *mockExperimentRepo {
	return &mockExperimentRepo{
		experiments: make(map[uuid.UUID]experiment.PromptExperiment),
	}
}

func (m *mockExperimentRepo) ListAll(_ context.Context, _ string, _ pagination.PageRequest) ([]experiment.PromptExperiment, int, error) {
	out := make([]experiment.PromptExperiment, 0, len(m.experiments))
	for _, e := range m.experiments {
		out = append(out, e)
	}
	return out, len(out), nil
}

func (m *mockExperimentRepo) GetByID(_ context.Context, _ string, id uuid.UUID) (experiment.PromptExperiment, error) {
	e, ok := m.experiments[id]
	if !ok {
		return experiment.PromptExperiment{}, experiment.ErrNotFound
	}
	return e, nil
}

func (m *mockExperimentRepo) Create(_ context.Context, _ string, e experiment.PromptExperiment) (experiment.PromptExperiment, error) {
	e.ID = uuid.New()
	m.experiments[e.ID] = e
	return e, nil
}

func (m *mockExperimentRepo) Update(_ context.Context, _ string, id uuid.UUID, e experiment.PromptExperiment) (experiment.PromptExperiment, error) {
	if _, ok := m.experiments[id]; !ok {
		return experiment.PromptExperiment{}, experiment.ErrNotFound
	}
	e.ID = id
	m.experiments[id] = e
	return e, nil
}

func (m *mockExperimentRepo) UpdateStatus(_ context.Context, _ string, id uuid.UUID, status experiment.ExperimentStatus) (experiment.PromptExperiment, error) {
	e, ok := m.experiments[id]
	if !ok {
		return experiment.PromptExperiment{}, experiment.ErrNotFound
	}
	e.Status = status
	m.experiments[id] = e
	return e, nil
}

func (m *mockExperimentRepo) Delete(_ context.Context, _ string, id uuid.UUID) error {
	if _, ok := m.experiments[id]; !ok {
		return experiment.ErrNotFound
	}
	delete(m.experiments, id)
	return nil
}

func (m *mockExperimentRepo) RecordResult(_ context.Context, _ string, res experiment.ExperimentResult) (experiment.ExperimentResult, error) {
	res.ID = uuid.New()
	m.results = append(m.results, res)
	return res, nil
}

func (m *mockExperimentRepo) GetResults(_ context.Context, _ string, experimentID uuid.UUID, _ pagination.PageRequest) ([]experiment.ExperimentResult, int, error) {
	var out []experiment.ExperimentResult
	for _, r := range m.results {
		if r.ExperimentID == experimentID {
			out = append(out, r)
		}
	}
	return out, len(out), nil
}

const tenantID = "test-tenant"

func TestExperimentService_Create_Success(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	agentID := uuid.New()
	e, err := svc.Create(context.Background(), tenantID, experiment.CreateRequest{
		AgentID:      agentID,
		Name:         "My Experiment",
		TrafficSplit: `{"A":50,"B":50}`,
		Variants:     `["A","B"]`,
		StartDate:    time.Now(),
		EndDate:      time.Now().Add(24 * time.Hour),
	})
	require.NoError(t, err)
	assert.Equal(t, "My Experiment", e.Name)
	assert.NotEqual(t, uuid.Nil, e.ID)
	assert.Equal(t, experiment.ExperimentStatusDraft, e.Status)
}

func TestExperimentService_GetByID_NotFound(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), tenantID, uuid.New())
	require.ErrorIs(t, err, experiment.ErrNotFound)
}

func TestExperimentService_Activate_Success(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantID, experiment.CreateRequest{
		AgentID: uuid.New(), Name: "exp", StartDate: time.Now(), EndDate: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	activated, err := svc.Activate(context.Background(), tenantID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, experiment.ExperimentStatusActive, activated.Status)
}

func TestExperimentService_Delete_NotFound(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	err := svc.Delete(context.Background(), tenantID, uuid.New())
	require.ErrorIs(t, err, experiment.ErrNotFound)
}

func TestExperimentService_RecordResult(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	// seed a fake experiment so RecordResult has a valid reference
	created, err := svc.Create(context.Background(), tenantID, experiment.CreateRequest{
		AgentID: uuid.New(), Name: "exp", StartDate: time.Now(), EndDate: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	expID := created.ID

	res, err := svc.RecordResult(context.Background(), tenantID, expID, experiment.RecordResultRequest{
		VariantKey:   "A",
		SessionID:    "sess-1",
		UserFeedback: 1,
		LatencyMs:    120,
		TokenCount:   200,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, res.ID)
	assert.Equal(t, "A", res.VariantKey)
}

func TestExperimentService_GetResults(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantID, experiment.CreateRequest{
		AgentID: uuid.New(), Name: "exp", StartDate: time.Now(), EndDate: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err = svc.RecordResult(context.Background(), tenantID, created.ID, experiment.RecordResultRequest{
			VariantKey: "B", SessionID: "s", LatencyMs: 100,
		})
		require.NoError(t, err)
	}

	items, total, err := svc.GetResults(context.Background(), tenantID, created.ID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, items, 3)
}

func TestExperimentService_ListAll(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	for i := 0; i < 2; i++ {
		_, err := svc.Create(context.Background(), tenantID, experiment.CreateRequest{
			AgentID: uuid.New(), Name: "exp", StartDate: time.Now(), EndDate: time.Now().Add(time.Hour),
		})
		require.NoError(t, err)
	}
	items, total, err := svc.ListAll(context.Background(), tenantID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, items, 2)
}
