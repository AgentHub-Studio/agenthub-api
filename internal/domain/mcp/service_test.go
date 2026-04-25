package mcp_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

type mockMCPRepo struct {
	data map[uuid.UUID]mcp.McpServerConfig
}

func newMockRepo() *mockMCPRepo {
	return &mockMCPRepo{data: make(map[uuid.UUID]mcp.McpServerConfig)}
}

func (m *mockMCPRepo) List(_ context.Context) ([]mcp.McpServerConfig, error) {
	out := make([]mcp.McpServerConfig, 0, len(m.data))
	for _, c := range m.data {
		out = append(out, c)
	}
	return out, nil
}

func (m *mockMCPRepo) GetByID(_ context.Context, id uuid.UUID) (mcp.McpServerConfig, error) {
	c, ok := m.data[id]
	if !ok {
		return mcp.McpServerConfig{}, mcp.ErrNotFound
	}
	return c, nil
}

func (m *mockMCPRepo) GetByName(_ context.Context, name string) (mcp.McpServerConfig, error) {
	for _, c := range m.data {
		if c.Name == name {
			return c, nil
		}
	}
	return mcp.McpServerConfig{}, mcp.ErrNotFound
}

func (m *mockMCPRepo) Create(_ context.Context, c mcp.McpServerConfig) (mcp.McpServerConfig, error) {
	c.ID = uuid.New()
	m.data[c.ID] = c
	return c, nil
}

func (m *mockMCPRepo) Update(_ context.Context, c mcp.McpServerConfig) (mcp.McpServerConfig, error) {
	if _, ok := m.data[c.ID]; !ok {
		return mcp.McpServerConfig{}, mcp.ErrNotFound
	}
	m.data[c.ID] = c
	return c, nil
}

func (m *mockMCPRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return mcp.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockMCPRepo) ListAutoStart(_ context.Context) ([]mcp.McpServerConfig, error) {
	var out []mcp.McpServerConfig
	for _, c := range m.data {
		if c.AutoStart {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *mockMCPRepo) ListAllEnabled(_ context.Context) ([]mcp.McpServerConfig, error) {
	var out []mcp.McpServerConfig
	for _, c := range m.data {
		if c.Enabled {
			out = append(out, c)
		}
	}
	return out, nil
}

type mockOAuthSvc struct {
	credentials map[uuid.UUID]oauth.OAuthCredential
}

func (m *mockOAuthSvc) GetByID(_ context.Context, _ string, id uuid.UUID) (oauth.OAuthCredential, error) {
	c, ok := m.credentials[id]
	if !ok {
		return oauth.OAuthCredential{}, oauth.ErrNotFound
	}
	return c, nil
}

func (m *mockOAuthSvc) Create(context.Context, string, oauth.CreateRequest) (oauth.OAuthCredential, error) {
	return oauth.OAuthCredential{}, nil
}

func (m *mockOAuthSvc) Update(context.Context, string, uuid.UUID, oauth.CreateRequest) (oauth.OAuthCredential, error) {
	return oauth.OAuthCredential{}, nil
}

func (m *mockOAuthSvc) ExchangeCode(context.Context, string, uuid.UUID, string) error {
	return nil
}

func (m *mockOAuthSvc) DecryptSecret(ciphertext *string) (*string, error) {
	return ciphertext, nil
}

func (m *mockOAuthSvc) GeneratePKCE() (string, string) {
	return "challenge", "verifier"
}

func TestMCPService_Create_Success(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	c, err := svc.Create(context.Background(), mcp.CreateRequest{
		Name:          "filesystem",
		TransportType: "http",
	})
	require.NoError(t, err)
	assert.Equal(t, "filesystem", c.Name)
	assert.NotEqual(t, uuid.Nil, c.ID)
}

func TestMCPService_GetByID_NotFound(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, mcp.ErrNotFound)
}

func TestMCPService_Delete_NotFound(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	err := svc.Delete(context.Background(), uuid.New())
	require.ErrorIs(t, err, mcp.ErrNotFound)
}

func TestMCPService_ListAutoStart(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	autoStart := true
	_, err := svc.Create(context.Background(), mcp.CreateRequest{Name: "auto", TransportType: "http", AutoStart: autoStart})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), mcp.CreateRequest{Name: "manual", TransportType: "http"})
	require.NoError(t, err)
	items, err := svc.ListAutoStart(context.Background())
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestMCPService_List(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	for _, name := range []string{"fs", "github", "slack"} {
		_, err := svc.Create(context.Background(), mcp.CreateRequest{Name: name, TransportType: "http"})
		require.NoError(t, err)
	}
	items, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, items, 3)
}

func TestMCPService_ListBootstrapIncludesOAuthRuntimeFields(t *testing.T) {
	repo := newMockRepo()
	serverID := uuid.New()
	credentialID := uuid.New()
	httpURL := "https://mcp.example.com"
	tokenURL := "https://auth.example.com/token"
	clientID := "client-id"
	clientSecret := "client-secret"
	bearerToken := "access-token"
	refreshToken := "refresh-token"
	scopes := "mcp:tools mcp:resources"

	repo.data[serverID] = mcp.McpServerConfig{
		ID:                serverID,
		Name:              "external-mcp",
		TransportType:     "http",
		HTTPBaseURL:       &httpURL,
		OAuthCredentialID: &credentialID,
		AutoStart:         true,
		Enabled:           true,
	}

	oauthSvc := &mockOAuthSvc{credentials: map[uuid.UUID]oauth.OAuthCredential{
		credentialID: {
			ID:           credentialID,
			TokenURL:     &tokenURL,
			ClientID:     &clientID,
			ClientSecret: &clientSecret,
			BearerToken:  &bearerToken,
			RefreshToken: &refreshToken,
			Scopes:       &scopes,
		},
	}}
	svc := mcp.NewService(repo).WithOAuthService(oauthSvc)

	items, err := svc.ListBootstrap(tenant.NewContext(context.Background(), "test"))

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "https://mcp.example.com", items[0].HTTPBaseURL)
	assert.Equal(t, tokenURL, items[0].OAuthTokenURL)
	assert.Equal(t, clientID, items[0].OAuthClientID)
	assert.Equal(t, clientSecret, items[0].OAuthClientSecret)
	assert.Equal(t, bearerToken, items[0].OAuthBearerToken)
	assert.Equal(t, refreshToken, items[0].OAuthRefreshToken)
	assert.Equal(t, []string{"mcp:tools", "mcp:resources"}, items[0].OAuthScopes)
}
