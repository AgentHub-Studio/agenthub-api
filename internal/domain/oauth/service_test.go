package oauth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockOAuthRepo struct {
	data map[uuid.UUID]oauth.OAuthCredential
}

func newMockRepo() *mockOAuthRepo {
	return &mockOAuthRepo{data: make(map[uuid.UUID]oauth.OAuthCredential)}
}

func (m *mockOAuthRepo) ListAll(_ context.Context, _ string, pr pagination.PageRequest) ([]oauth.OAuthCredential, int, error) {
	out := make([]oauth.OAuthCredential, 0, len(m.data))
	for _, c := range m.data {
		out = append(out, c)
	}
	return out, len(out), nil
}

func (m *mockOAuthRepo) GetByID(_ context.Context, _ string, id uuid.UUID) (oauth.OAuthCredential, error) {
	c, ok := m.data[id]
	if !ok {
		return oauth.OAuthCredential{}, oauth.ErrNotFound
	}
	return c, nil
}

func (m *mockOAuthRepo) Create(_ context.Context, _ string, c oauth.OAuthCredential) (oauth.OAuthCredential, error) {
	c.ID = uuid.New()
	m.data[c.ID] = c
	return c, nil
}

func (m *mockOAuthRepo) Update(_ context.Context, _ string, id uuid.UUID, c oauth.OAuthCredential) (oauth.OAuthCredential, error) {
	if _, ok := m.data[id]; !ok {
		return oauth.OAuthCredential{}, oauth.ErrNotFound
	}
	c.ID = id
	m.data[id] = c
	return c, nil
}

func (m *mockOAuthRepo) Delete(_ context.Context, _ string, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return oauth.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

const tenantID = "test-tenant"

func TestOAuthService_Create_Success(t *testing.T) {
	svc := oauth.NewService(newMockRepo())
	c, err := svc.Create(context.Background(), tenantID, oauth.CreateRequest{
		Name:     "My API Key",
		AuthType: oauth.AuthTypeAPIKey,
	})
	require.NoError(t, err)
	assert.Equal(t, "My API Key", c.Name)
	assert.NotEqual(t, uuid.Nil, c.ID)
}

func TestOAuthService_GetByID_NotFound(t *testing.T) {
	svc := oauth.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), tenantID, uuid.New())
	require.ErrorIs(t, err, oauth.ErrNotFound)
}

func TestOAuthService_Update_NotFound(t *testing.T) {
	svc := oauth.NewService(newMockRepo())
	_, err := svc.Update(context.Background(), tenantID, uuid.New(), oauth.CreateRequest{Name: "x"})
	require.ErrorIs(t, err, oauth.ErrNotFound)
}

func TestOAuthService_Delete_Success(t *testing.T) {
	svc := oauth.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantID, oauth.CreateRequest{Name: "x", AuthType: oauth.AuthTypeBearerToken})
	require.NoError(t, err)
	err = svc.Delete(context.Background(), tenantID, created.ID)
	require.NoError(t, err)
}

func TestOAuthService_ListAll(t *testing.T) {
	svc := oauth.NewService(newMockRepo())
	for i := 0; i < 3; i++ {
		_, err := svc.Create(context.Background(), tenantID, oauth.CreateRequest{Name: "cred", AuthType: oauth.AuthTypeAPIKey})
		require.NoError(t, err)
	}
	items, total, err := svc.ListAll(context.Background(), tenantID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	assert.Len(t, items, 3)
}
