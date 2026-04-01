package user_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/user"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockKeycloakClient is a simple in-memory stub for KeycloakUserClient.
type mockKeycloakClient struct {
	users map[string]user.User
	roles map[string][]string // userID -> roles
}

func newMockClient() *mockKeycloakClient {
	return &mockKeycloakClient{
		users: make(map[string]user.User),
		roles: make(map[string][]string),
	}
}

func (m *mockKeycloakClient) ListUsers(_ context.Context, _ string) ([]user.User, error) {
	out := make([]user.User, 0, len(m.users))
	for _, u := range m.users {
		out = append(out, u)
	}
	return out, nil
}

func (m *mockKeycloakClient) GetUser(_ context.Context, _ string, userID string) (user.User, error) {
	u, ok := m.users[userID]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}

func (m *mockKeycloakClient) CreateUser(_ context.Context, _ string, req user.CreateUserRequest) (user.User, error) {
	for _, u := range m.users {
		if u.Username == req.Username {
			return user.User{}, user.ErrAlreadyExists
		}
	}
	id := req.Username + "-id"
	u := user.User{
		ID:       id,
		Username: req.Username,
		Email:    req.Email,
		Enabled:  true,
	}
	m.users[id] = u
	return u, nil
}

func (m *mockKeycloakClient) UpdateUser(_ context.Context, _ string, userID string, req user.UpdateUserRequest) (user.User, error) {
	u, ok := m.users[userID]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	if req.Email != nil {
		u.Email = *req.Email
	}
	if req.FirstName != nil {
		u.FirstName = *req.FirstName
	}
	m.users[userID] = u
	return u, nil
}

func (m *mockKeycloakClient) DeleteUser(_ context.Context, _ string, userID string) error {
	if _, ok := m.users[userID]; !ok {
		return user.ErrNotFound
	}
	delete(m.users, userID)
	return nil
}

func (m *mockKeycloakClient) AssignRole(_ context.Context, _ string, userID string, role string) error {
	m.roles[userID] = append(m.roles[userID], role)
	return nil
}

func (m *mockKeycloakClient) RemoveRole(_ context.Context, _ string, userID string, role string) error {
	roles := m.roles[userID]
	for i, r := range roles {
		if r == role {
			m.roles[userID] = append(roles[:i], roles[i+1:]...)
			return nil
		}
	}
	return user.ErrNotFound
}

// ctxWithTenant returns a context with the given tenant ID set.
func ctxWithTenant(tenantID string) context.Context {
	return tenant.NewContext(context.Background(), tenantID)
}

func TestUserService_List_Success(t *testing.T) {
	client := newMockClient()
	svc := user.NewService(client)

	ctx := ctxWithTenant("tenant-1")
	_, err := svc.Create(ctx, user.CreateUserRequest{Username: "alice", Email: "alice@test.com"})
	require.NoError(t, err)
	_, err = svc.Create(ctx, user.CreateUserRequest{Username: "bob", Email: "bob@test.com"})
	require.NoError(t, err)

	users, err := svc.List(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUserService_Create_MissingUsername(t *testing.T) {
	svc := user.NewService(newMockClient())
	_, err := svc.Create(ctxWithTenant("t1"), user.CreateUserRequest{Email: "no-username@x.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username is required")
}

func TestUserService_Get_NotFound(t *testing.T) {
	svc := user.NewService(newMockClient())
	_, err := svc.Get(ctxWithTenant("t1"), "ghost")
	require.ErrorIs(t, err, user.ErrNotFound)
}

func TestUserService_Create_AlreadyExists(t *testing.T) {
	svc := user.NewService(newMockClient())
	ctx := ctxWithTenant("t1")
	_, err := svc.Create(ctx, user.CreateUserRequest{Username: "alice"})
	require.NoError(t, err)
	_, err = svc.Create(ctx, user.CreateUserRequest{Username: "alice"})
	require.ErrorIs(t, err, user.ErrAlreadyExists)
}

func TestUserService_AssignAndRemoveRole(t *testing.T) {
	client := newMockClient()
	svc := user.NewService(client)
	ctx := ctxWithTenant("t1")
	u, err := svc.Create(ctx, user.CreateUserRequest{Username: "carol"})
	require.NoError(t, err)

	err = svc.AssignRole(ctx, u.ID, "admin")
	require.NoError(t, err)
	assert.Contains(t, client.roles[u.ID], "admin")

	err = svc.RemoveRole(ctx, u.ID, "admin")
	require.NoError(t, err)
	assert.NotContains(t, client.roles[u.ID], "admin")
}

func TestUserService_Delete_NotFound(t *testing.T) {
	svc := user.NewService(newMockClient())
	err := svc.Delete(ctxWithTenant("t1"), "ghost-id")
	require.ErrorIs(t, err, user.ErrNotFound)
}

func TestUserService_NoTenantContext(t *testing.T) {
	svc := user.NewService(newMockClient())
	// Context without tenant should return an error.
	_, err := svc.List(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenantID not found")
}
