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
	for _, e := range m.experiments { out = append(out, e) }
	return out, len(out), nil
}

func (m *mockExperimentRepo) GetByID(_ context.Context, _ string, id uuid.UUID) (experiment.PromptExperiment, error) {
	e, ok := m.experiments[id]
	if !ok { return experiment.PromptExperiment{}, experiment.ErrNotFound }
	return e, nil
}

func (m *mockExperimentRepo) Create(_ context.Context, _ string, e experiment.PromptExperiment) (experiment.PromptExperiment, error) {
	e.ID = uuid.New()
	m.experiments[e.ID] = e
	return e, nil
}

func (m *mockExperimentRepo) Update(_ context.Context, _ string, id uuid.UUID, e experiment.PromptExperiment) (experiment.PromptExperiment, error) {
	if _, ok := m.experiments[id]; !ok { return experiment.PromptExperiment{}, experiment.ErrNotFound }
	e.ID = id
	m.experiments[id] = e
	return e, nil
}

func (m *mockExperimentRepo) UpdateStatus(_ context.Context, _ string, id uuid.UUID, status experiment.ExperimentStatus) (experiment.PromptExperiment, error) {
	e, ok := m.experiments[id]
	if !ok { return experiment.PromptExperiment{}, experiment.ErrNotFound }
	e.Status = status
	m.experiments[id] = e
	return e, nil
}

func (m *mockExperimentRepo) Delete(_ context.Context, _ string, id uuid.UUID) error {
	if _, ok := m.experiments[id]; !ok { return experiment.ErrNotFound }
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
		if r.ExperimentID == experimentID { out = append(out, r) }
	}
	return out, len(out), nil
}

const tenantID = "test-tenant"

func newExp(t *testing.T, svc *experiment.Service, split string) experiment.PromptExperiment {
	t.Helper()
	e, err := svc.Create(context.Background(), tenantID, experiment.CreateRequest{
		AgentID: uuid.New(), Name: "exp", TrafficSplit: split,
		Variants: `["A","B"]`, StartDate: time.Now(), EndDate: time.Now().Add(24 * time.Hour),
	})
	require.NoError(t, err)
	return e
}

func TestExperimentService_Create_Success(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, `{"A":50,"B":50}`)
	assert.NotEqual(t, uuid.Nil, e.ID)
	assert.Equal(t, experiment.ExperimentStatusDraft, e.Status)
}

func TestExperimentService_GetByID_NotFound(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), tenantID, uuid.New())
	require.ErrorIs(t, err, experiment.ErrNotFound)
}

func TestExperimentService_Delete_NotFound(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	require.ErrorIs(t, svc.Delete(context.Background(), tenantID, uuid.New()), experiment.ErrNotFound)
}

func TestExperimentService_ListAll(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	newExp(t, svc, ""); newExp(t, svc, "")
	items, total, err := svc.ListAll(context.Background(), tenantID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 2, total); assert.Len(t, items, 2)
}

// traffic split validation

func TestExperimentService_Create_InvalidTrafficSplit_NotSum100(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tenantID, experiment.CreateRequest{
		AgentID: uuid.New(), Name: "bad", TrafficSplit: `{"A":60,"B":30}`,
	})
	require.ErrorIs(t, err, experiment.ErrInvalidTrafficSplit)
}

func TestExperimentService_Create_ValidTrafficSplit_50_50(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, `{"A":50,"B":50}`)
	assert.NotEqual(t, uuid.Nil, e.ID)
}

func TestExperimentService_Create_ValidTrafficSplit_ThreeVariants(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, `{"A":33,"B":33,"C":34}`)
	assert.NotEqual(t, uuid.Nil, e.ID)
}

// state machine tests

func TestExperimentService_Activate_FromDraft_Success(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, `{"A":50,"B":50}`)
	activated, err := svc.Activate(context.Background(), tenantID, e.ID)
	require.NoError(t, err)
	assert.Equal(t, experiment.ExperimentStatusActive, activated.Status)
}

func TestExperimentService_Pause_FromActive_Success(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, `{"A":50,"B":50}`)
	_, _ = svc.Activate(context.Background(), tenantID, e.ID)
	paused, err := svc.Pause(context.Background(), tenantID, e.ID)
	require.NoError(t, err)
	assert.Equal(t, experiment.ExperimentStatusPaused, paused.Status)
}

func TestExperimentService_Complete_FromPaused_Success(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, `{"A":50,"B":50}`)
	_, _ = svc.Activate(context.Background(), tenantID, e.ID)
	_, _ = svc.Pause(context.Background(), tenantID, e.ID)
	completed, err := svc.Complete(context.Background(), tenantID, e.ID)
	require.NoError(t, err)
	assert.Equal(t, experiment.ExperimentStatusCompleted, completed.Status)
}

func TestExperimentService_Pause_FromDraft_InvalidTransition(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, "")
	_, err := svc.Pause(context.Background(), tenantID, e.ID)
	require.ErrorIs(t, err, experiment.ErrInvalidTransition)
}

func TestExperimentService_Activate_FromCompleted_InvalidTransition(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, `{"A":50,"B":50}`)
	_, _ = svc.Activate(context.Background(), tenantID, e.ID)
	_, _ = svc.Complete(context.Background(), tenantID, e.ID)
	_, err := svc.Activate(context.Background(), tenantID, e.ID)
	require.ErrorIs(t, err, experiment.ErrInvalidTransition)
}

// variant selection tests

func TestExperimentService_SelectVariant_Deterministic(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, `{"A":50,"B":50}`)
	v1, err := svc.SelectVariant(context.Background(), tenantID, e.ID, "session-abc")
	require.NoError(t, err)
	v2, err := svc.SelectVariant(context.Background(), tenantID, e.ID, "session-abc")
	require.NoError(t, err)
	assert.Equal(t, v1, v2)
}

func TestExperimentService_SelectVariant_DistributesBothVariants(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, `{"A":50,"B":50}`)
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		v, err := svc.SelectVariant(context.Background(), tenantID, e.ID, uuid.New().String())
		require.NoError(t, err)
		seen[v] = true
	}
	assert.True(t, seen["A"], "variant A should appear")
	assert.True(t, seen["B"], "variant B should appear")
}

// GetSummary tests

func TestExperimentService_GetSummary_AggregatesPerVariant(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, `{"A":50,"B":50}`)
	for i := 0; i < 3; i++ {
		_, err := svc.RecordResult(context.Background(), tenantID, e.ID, experiment.RecordResultRequest{
			VariantKey: "A", SessionID: uuid.New().String(), UserFeedback: 1, LatencyMs: 100,
		})
		require.NoError(t, err)
	}
	for i := 0; i < 2; i++ {
		_, err := svc.RecordResult(context.Background(), tenantID, e.ID, experiment.RecordResultRequest{
			VariantKey: "B", SessionID: uuid.New().String(), UserFeedback: 0, LatencyMs: 200,
		})
		require.NoError(t, err)
	}
	summary, err := svc.GetSummary(context.Background(), tenantID, e.ID)
	require.NoError(t, err)
	assert.Equal(t, e.ID, summary.ExperimentID)
	assert.Len(t, summary.Variants, 2)
	byKey := map[string]experiment.VariantSummary{}
	for _, v := range summary.Variants { byKey[v.VariantKey] = v }
	assert.Equal(t, 3, byKey["A"].Count)
	assert.Equal(t, 2, byKey["B"].Count)
	assert.InDelta(t, 1.0, byKey["A"].AvgFeedback, 0.01)
	assert.InDelta(t, 100.0, byKey["A"].AvgLatencyMs, 0.01)
}

// results tests

func TestExperimentService_RecordResult(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, "")
	res, err := svc.RecordResult(context.Background(), tenantID, e.ID, experiment.RecordResultRequest{
		VariantKey: "A", SessionID: "sess-1", UserFeedback: 1, LatencyMs: 120, TokenCount: 200,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, res.ID)
	assert.Equal(t, "A", res.VariantKey)
}

func TestExperimentService_GetResults(t *testing.T) {
	svc := experiment.NewService(newMockRepo())
	e := newExp(t, svc, "")
	for i := 0; i < 3; i++ {
		_, err := svc.RecordResult(context.Background(), tenantID, e.ID, experiment.RecordResultRequest{
			VariantKey: "B", SessionID: "s", LatencyMs: 100,
		})
		require.NoError(t, err)
	}
	items, total, err := svc.GetResults(context.Background(), tenantID, e.ID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 3, total); assert.Len(t, items, 3)
}
