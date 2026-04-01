package knowledgebase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockRepository is a test double for knowledgebase.Repository.
type mockRepository struct {
	items  map[uuid.UUID]knowledgebase.KnowledgeBase
	createErr error
	getErr    error
	deleteErr error
	statusErr error
}

func newMockRepository() *mockRepository {
	return &mockRepository{items: make(map[uuid.UUID]knowledgebase.KnowledgeBase)}
}

func (m *mockRepository) List(_ context.Context, req pagination.PageRequest) ([]knowledgebase.KnowledgeBase, int64, error) {
	var result []knowledgebase.KnowledgeBase
	for _, v := range m.items {
		result = append(result, v)
	}
	return result, int64(len(result)), nil
}

func (m *mockRepository) GetByID(_ context.Context, id uuid.UUID) (knowledgebase.KnowledgeBase, error) {
	if m.getErr != nil {
		return knowledgebase.KnowledgeBase{}, m.getErr
	}
	kb, ok := m.items[id]
	if !ok {
		return knowledgebase.KnowledgeBase{}, knowledgebase.ErrNotFound
	}
	return kb, nil
}

func (m *mockRepository) Create(_ context.Context, k knowledgebase.KnowledgeBase) (knowledgebase.KnowledgeBase, error) {
	if m.createErr != nil {
		return knowledgebase.KnowledgeBase{}, m.createErr
	}
	k.ID = uuid.New()
	k.CreatedAt = time.Now()
	k.UpdatedAt = time.Now()
	m.items[k.ID] = k
	return k, nil
}

func (m *mockRepository) Update(_ context.Context, k knowledgebase.KnowledgeBase) (knowledgebase.KnowledgeBase, error) {
	if _, ok := m.items[k.ID]; !ok {
		return knowledgebase.KnowledgeBase{}, knowledgebase.ErrNotFound
	}
	k.UpdatedAt = time.Now()
	m.items[k.ID] = k
	return k, nil
}

func (m *mockRepository) Delete(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.items[id]; !ok {
		return knowledgebase.ErrNotFound
	}
	delete(m.items, id)
	return nil
}

func (m *mockRepository) UpdateStatus(_ context.Context, id uuid.UUID, status knowledgebase.KnowledgeBaseStatus) (knowledgebase.KnowledgeBase, error) {
	if m.statusErr != nil {
		return knowledgebase.KnowledgeBase{}, m.statusErr
	}
	kb, ok := m.items[id]
	if !ok {
		return knowledgebase.KnowledgeBase{}, knowledgebase.ErrNotFound
	}
	kb.Status = status
	kb.UpdatedAt = time.Now()
	m.items[id] = kb
	return kb, nil
}

// -------- tests --------

func TestService_Create_Success(t *testing.T) {
	repo := newMockRepository()
	svc := knowledgebase.NewService(repo)

	resp, err := svc.Create(context.Background(), knowledgebase.CreateRequest{
		Name:        "My KB",
		Description: "Test description",
	})

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, resp.ID)
	assert.Equal(t, "My KB", resp.Name)
	assert.Equal(t, "Test description", resp.Description)
	assert.Equal(t, knowledgebase.StatusActive, resp.Status)
}

func TestService_Create_MissingName(t *testing.T) {
	repo := newMockRepository()
	svc := knowledgebase.NewService(repo)

	_, err := svc.Create(context.Background(), knowledgebase.CreateRequest{Name: ""})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestService_Create_RepositoryError(t *testing.T) {
	repo := newMockRepository()
	repo.createErr = errors.New("db error")
	svc := knowledgebase.NewService(repo)

	_, err := svc.Create(context.Background(), knowledgebase.CreateRequest{Name: "KB"})

	require.Error(t, err)
}

func TestService_GetByID_NotFound(t *testing.T) {
	repo := newMockRepository()
	svc := knowledgebase.NewService(repo)

	_, err := svc.GetByID(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, knowledgebase.ErrNotFound))
}

func TestService_GetByID_Success(t *testing.T) {
	repo := newMockRepository()
	svc := knowledgebase.NewService(repo)

	created, err := svc.Create(context.Background(), knowledgebase.CreateRequest{Name: "My KB"})
	require.NoError(t, err)

	resp, err := svc.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, resp.ID)
	assert.Equal(t, "My KB", resp.Name)
}

func TestService_Delete_Success(t *testing.T) {
	repo := newMockRepository()
	svc := knowledgebase.NewService(repo)

	created, err := svc.Create(context.Background(), knowledgebase.CreateRequest{Name: "KB to delete"})
	require.NoError(t, err)

	err = svc.Delete(context.Background(), created.ID)
	require.NoError(t, err)

	_, err = svc.GetByID(context.Background(), created.ID)
	assert.True(t, errors.Is(err, knowledgebase.ErrNotFound))
}

func TestService_Delete_NotFound(t *testing.T) {
	repo := newMockRepository()
	svc := knowledgebase.NewService(repo)

	err := svc.Delete(context.Background(), uuid.New())

	require.Error(t, err)
}

func TestService_Pause_Success(t *testing.T) {
	repo := newMockRepository()
	svc := knowledgebase.NewService(repo)

	created, err := svc.Create(context.Background(), knowledgebase.CreateRequest{Name: "KB"})
	require.NoError(t, err)
	assert.Equal(t, knowledgebase.StatusActive, created.Status)

	paused, err := svc.Pause(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, knowledgebase.StatusPaused, paused.Status)
}

func TestService_Activate_Success(t *testing.T) {
	repo := newMockRepository()
	svc := knowledgebase.NewService(repo)

	created, err := svc.Create(context.Background(), knowledgebase.CreateRequest{Name: "KB"})
	require.NoError(t, err)

	_, err = svc.Pause(context.Background(), created.ID)
	require.NoError(t, err)

	activated, err := svc.Activate(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, knowledgebase.StatusActive, activated.Status)
}

func TestService_Pause_NotFound(t *testing.T) {
	repo := newMockRepository()
	svc := knowledgebase.NewService(repo)

	_, err := svc.Pause(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, knowledgebase.ErrNotFound))
}

func TestService_List_Success(t *testing.T) {
	repo := newMockRepository()
	svc := knowledgebase.NewService(repo)

	_, err := svc.Create(context.Background(), knowledgebase.CreateRequest{Name: "KB 1"})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), knowledgebase.CreateRequest{Name: "KB 2"})
	require.NoError(t, err)

	page, err := svc.List(context.Background(), pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(2), page.TotalElements)
	assert.Len(t, page.Content, 2)
}
