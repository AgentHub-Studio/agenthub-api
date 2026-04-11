package abtest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/abtest"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockRepo satisfies abtest.Repository for unit tests.
type mockRepo struct {
	tests       map[uuid.UUID]abtest.ABTest
	assignments []abtest.Assignment
}

func newMockRepo() *mockRepo {
	return &mockRepo{tests: make(map[uuid.UUID]abtest.ABTest)}
}

func (m *mockRepo) List(_ context.Context, agentID uuid.UUID, req pagination.PageRequest) (pagination.Page[abtest.ABTest], error) {
	var items []abtest.ABTest
	for _, t := range m.tests {
		if t.AgentID == agentID {
			items = append(items, t)
		}
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockRepo) GetByID(_ context.Context, id uuid.UUID) (abtest.ABTest, error) {
	t, ok := m.tests[id]
	if !ok {
		return abtest.ABTest{}, abtest.ErrNotFound
	}
	return t, nil
}

func (m *mockRepo) Create(_ context.Context, t abtest.ABTest) (abtest.ABTest, error) {
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	m.tests[t.ID] = t
	return t, nil
}

func (m *mockRepo) Update(_ context.Context, t abtest.ABTest) (abtest.ABTest, error) {
	if _, ok := m.tests[t.ID]; !ok {
		return abtest.ABTest{}, abtest.ErrNotFound
	}
	t.UpdatedAt = time.Now()
	m.tests[t.ID] = t
	return t, nil
}

func (m *mockRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.tests[id]; !ok {
		return abtest.ErrNotFound
	}
	delete(m.tests, id)
	return nil
}

func (m *mockRepo) GetActiveByAgent(_ context.Context, agentID uuid.UUID) (abtest.ABTest, error) {
	for _, t := range m.tests {
		if t.AgentID == agentID && t.Status == abtest.TestStatusActive {
			return t, nil
		}
	}
	return abtest.ABTest{}, abtest.ErrNotFound
}

func (m *mockRepo) RecordAssignment(_ context.Context, a abtest.Assignment) error {
	m.assignments = append(m.assignments, a)
	return nil
}

func setup() (abtest.Service, *mockRepo) {
	repo := newMockRepo()
	return abtest.NewService(repo), repo
}

func TestService_Create_Success(t *testing.T) {
	svc, _ := setup()
	variantID := uuid.New()
	req := abtest.CreateABTestRequest{
		AgentID:          uuid.New(),
		Name:             "My Test",
		VariantVersionID: variantID,
		TrafficPercent:   20,
	}
	resp, err := svc.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "My Test", resp.Name)
	assert.Equal(t, variantID, resp.VariantVersionID)
	assert.Equal(t, 20, resp.TrafficPercent)
	assert.Equal(t, "ACTIVE", resp.Status)
}

func TestService_Create_ValidationError(t *testing.T) {
	svc, _ := setup()
	_, err := svc.Create(context.Background(), abtest.CreateABTestRequest{
		AgentID:        uuid.New(),
		TrafficPercent: 50,
		// missing Name and VariantVersionID
	})
	require.Error(t, err)
}

func TestService_Create_TrafficPercentOutOfRange(t *testing.T) {
	svc, _ := setup()
	_, err := svc.Create(context.Background(), abtest.CreateABTestRequest{
		AgentID:          uuid.New(),
		Name:             "x",
		VariantVersionID: uuid.New(),
		TrafficPercent:   101,
	})
	require.Error(t, err)
}

func TestService_GetByID_NotFound(t *testing.T) {
	svc, _ := setup()
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, abtest.ErrNotFound))
}

func TestService_Update_Status_Concluded(t *testing.T) {
	svc, _ := setup()
	variantID := uuid.New()
	created, err := svc.Create(context.Background(), abtest.CreateABTestRequest{
		AgentID:          uuid.New(),
		Name:             "Conclude Me",
		VariantVersionID: variantID,
		TrafficPercent:   10,
	})
	require.NoError(t, err)

	status := "CONCLUDED"
	updated, err := svc.Update(context.Background(), created.ID, abtest.UpdateABTestRequest{
		Status: &status,
	})
	require.NoError(t, err)
	assert.Equal(t, "CONCLUDED", updated.Status)
	assert.NotNil(t, updated.EndedAt)
}

func TestService_Delete(t *testing.T) {
	svc, _ := setup()
	created, err := svc.Create(context.Background(), abtest.CreateABTestRequest{
		AgentID:          uuid.New(),
		Name:             "Delete Me",
		VariantVersionID: uuid.New(),
		TrafficPercent:   5,
	})
	require.NoError(t, err)
	require.NoError(t, svc.Delete(context.Background(), created.ID))
	_, err = svc.GetByID(context.Background(), created.ID)
	assert.True(t, errors.Is(err, abtest.ErrNotFound))
}

func TestService_AssignVariant_NoActiveTest(t *testing.T) {
	svc, _ := setup()
	result, err := svc.AssignVariant(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestService_AssignVariant_100Percent_AlwaysVariant(t *testing.T) {
	svc, repo := setup()
	variantID := uuid.New()
	agentID := uuid.New()
	created, err := svc.Create(context.Background(), abtest.CreateABTestRequest{
		AgentID:          agentID,
		Name:             "Full Traffic",
		VariantVersionID: variantID,
		TrafficPercent:   100,
	})
	require.NoError(t, err)
	assert.Equal(t, "ACTIVE", created.Status)

	for i := 0; i < 20; i++ {
		result, err := svc.AssignVariant(context.Background(), agentID, uuid.New())
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, variantID, *result)
	}
	// All assignments should be "variant"
	for _, a := range repo.assignments {
		assert.Equal(t, abtest.VariantVariant, a.Variant)
	}
}

func TestService_AssignVariant_0Percent_AlwaysControl(t *testing.T) {
	svc, repo := setup()
	agentID := uuid.New()
	_, err := svc.Create(context.Background(), abtest.CreateABTestRequest{
		AgentID:          agentID,
		Name:             "No Traffic",
		VariantVersionID: uuid.New(),
		TrafficPercent:   0,
	})
	require.NoError(t, err)

	for i := 0; i < 20; i++ {
		result, err := svc.AssignVariant(context.Background(), agentID, uuid.New())
		require.NoError(t, err)
		// nil = use control (agent's live config)
		assert.Nil(t, result)
	}
	for _, a := range repo.assignments {
		assert.Equal(t, abtest.VariantControl, a.Variant)
	}
}
