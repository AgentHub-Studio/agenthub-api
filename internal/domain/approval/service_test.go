package approval_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/approval"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockApprovalRepo implements approval.Repository for unit tests.
type mockApprovalRepo struct {
	data map[uuid.UUID]approval.PendingApproval
}

func newMockApprovalRepo() *mockApprovalRepo {
	return &mockApprovalRepo{data: make(map[uuid.UUID]approval.PendingApproval)}
}

func (m *mockApprovalRepo) Create(_ context.Context, req approval.CreateApprovalRequest) (approval.PendingApproval, error) {
	a := approval.PendingApproval{
		ID:             uuid.New(),
		ExecutionID:    req.ExecutionID,
		NodeID:         req.NodeID,
		Title:          req.Title,
		Description:    req.Description,
		Details:        req.Details,
		Status:         approval.StatusPending,
		TimeoutAt:      req.TimeoutAt,
		CallbackURL:    req.CallbackURL,
		NotifyChannels: req.NotifyChannels,
		CreatedAt:      time.Now(),
	}
	if a.NotifyChannels == nil {
		a.NotifyChannels = []string{}
	}
	m.data[a.ID] = a
	return a, nil
}

func (m *mockApprovalRepo) GetByID(_ context.Context, id uuid.UUID) (approval.PendingApproval, error) {
	a, ok := m.data[id]
	if !ok {
		return approval.PendingApproval{}, approval.ErrNotFound
	}
	return a, nil
}

func (m *mockApprovalRepo) List(_ context.Context, offset, limit int) ([]approval.PendingApproval, int64, error) {
	all := make([]approval.PendingApproval, 0, len(m.data))
	for _, a := range m.data {
		all = append(all, a)
	}
	total := int64(len(all))
	if offset >= len(all) {
		return []approval.PendingApproval{}, total, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

func (m *mockApprovalRepo) PendingCount(_ context.Context) (int64, error) {
	var count int64
	for _, a := range m.data {
		if a.Status == approval.StatusPending {
			count++
		}
	}
	return count, nil
}

func (m *mockApprovalRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return approval.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockApprovalRepo) Respond(_ context.Context, id uuid.UUID, respondedBy string, req approval.RespondRequest) (approval.PendingApproval, error) {
	a, ok := m.data[id]
	if !ok {
		return approval.PendingApproval{}, approval.ErrNotFound
	}
	if a.Status != approval.StatusPending {
		return approval.PendingApproval{}, approval.ErrAlreadyResolved
	}
	if req.Approved {
		a.Status = approval.StatusApproved
	} else {
		a.Status = approval.StatusRejected
	}
	now := time.Now()
	a.RespondedBy = &respondedBy
	a.RespondedAt = &now
	a.Comment = req.Comment
	m.data[id] = a
	return a, nil
}

// helpers

func createTestApproval(t *testing.T, svc approval.Service) approval.PendingApproval {
	t.Helper()
	a, err := svc.Create(context.Background(), approval.CreateApprovalRequest{
		ExecutionID: uuid.New(),
		NodeID:      "node-1",
		Title:       "Deploy to production?",
	})
	require.NoError(t, err)
	return a
}

// ------ service tests ------

func TestApprovalService_Create_Success(t *testing.T) {
	svc := approval.NewService(newMockApprovalRepo())
	a, err := svc.Create(context.Background(), approval.CreateApprovalRequest{
		ExecutionID: uuid.New(),
		NodeID:      "node-1",
		Title:       "Approve deployment",
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, a.ID)
	assert.Equal(t, approval.StatusPending, a.Status)
	assert.NotNil(t, a.NotifyChannels)
}

func TestApprovalService_Create_MissingExecutionID(t *testing.T) {
	svc := approval.NewService(newMockApprovalRepo())
	_, err := svc.Create(context.Background(), approval.CreateApprovalRequest{
		NodeID: "node-1",
		Title:  "Missing execution",
	})
	require.Error(t, err)
}

func TestApprovalService_Create_MissingNodeID(t *testing.T) {
	svc := approval.NewService(newMockApprovalRepo())
	_, err := svc.Create(context.Background(), approval.CreateApprovalRequest{
		ExecutionID: uuid.New(),
		Title:       "Missing node",
	})
	require.Error(t, err)
}

func TestApprovalService_Create_MissingTitle(t *testing.T) {
	svc := approval.NewService(newMockApprovalRepo())
	_, err := svc.Create(context.Background(), approval.CreateApprovalRequest{
		ExecutionID: uuid.New(),
		NodeID:      "node-1",
	})
	require.Error(t, err)
}

func TestApprovalService_GetByID_Success(t *testing.T) {
	svc := approval.NewService(newMockApprovalRepo())
	created := createTestApproval(t, svc)

	got, err := svc.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestApprovalService_GetByID_NotFound(t *testing.T) {
	svc := approval.NewService(newMockApprovalRepo())
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, approval.ErrNotFound)
}

func TestApprovalService_List_Paginated(t *testing.T) {
	svc := approval.NewService(newMockApprovalRepo())
	for i := 0; i < 5; i++ {
		createTestApproval(t, svc)
	}

	page, err := svc.List(context.Background(), pagination.PageRequest{Page: 0, Size: 3})
	require.NoError(t, err)
	assert.Equal(t, int64(5), page.TotalElements)
	assert.Len(t, page.Content, 3)
	assert.False(t, page.Last)
}

func TestApprovalService_PendingCount(t *testing.T) {
	svc := approval.NewService(newMockApprovalRepo())
	createTestApproval(t, svc)
	createTestApproval(t, svc)

	resp, err := svc.PendingCount(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Count)
}

func TestApprovalService_Respond_Approve(t *testing.T) {
	svc := approval.NewService(newMockApprovalRepo())
	a := createTestApproval(t, svc)

	responded, err := svc.Respond(context.Background(), a.ID, "user@example.com", approval.RespondRequest{
		Approved: true,
	})
	require.NoError(t, err)
	assert.Equal(t, approval.StatusApproved, responded.Status)
	require.NotNil(t, responded.RespondedBy)
	assert.Equal(t, "user@example.com", *responded.RespondedBy)
}

func TestApprovalService_Respond_Reject(t *testing.T) {
	svc := approval.NewService(newMockApprovalRepo())
	a := createTestApproval(t, svc)
	comment := "not ready"

	responded, err := svc.Respond(context.Background(), a.ID, "user@example.com", approval.RespondRequest{
		Approved: false,
		Comment:  &comment,
	})
	require.NoError(t, err)
	assert.Equal(t, approval.StatusRejected, responded.Status)
	require.NotNil(t, responded.Comment)
	assert.Equal(t, "not ready", *responded.Comment)
}

func TestApprovalService_Respond_NotFound(t *testing.T) {
	svc := approval.NewService(newMockApprovalRepo())
	_, err := svc.Respond(context.Background(), uuid.New(), "user@example.com", approval.RespondRequest{Approved: true})
	require.ErrorIs(t, err, approval.ErrNotFound)
}

func TestApprovalService_Respond_AlreadyResolved(t *testing.T) {
	svc := approval.NewService(newMockApprovalRepo())
	a := createTestApproval(t, svc)

	_, err := svc.Respond(context.Background(), a.ID, "user@example.com", approval.RespondRequest{Approved: true})
	require.NoError(t, err)

	// Second response must fail
	_, err = svc.Respond(context.Background(), a.ID, "user2@example.com", approval.RespondRequest{Approved: false})
	require.ErrorIs(t, err, approval.ErrAlreadyResolved)
}
