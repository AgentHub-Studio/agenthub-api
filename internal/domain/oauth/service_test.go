package oauth_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockRepo implements CredentialRepository for unit tests.
type mockRepo struct {
	creds map[uuid.UUID]oauth.OAuthCredential
}

func newMockRepo() *mockRepo {
	return &mockRepo{creds: make(map[uuid.UUID]oauth.OAuthCredential)}
}

func (m *mockRepo) ListAll(_ context.Context, _ string, _ pagination.PageRequest) ([]oauth.OAuthCredential, int, error) {
	out := make([]oauth.OAuthCredential, 0, len(m.creds))
	for _, c := range m.creds { out = append(out, c) }
	return out, len(out), nil
}

func (m *mockRepo) GetByID(_ context.Context, _ string, id uuid.UUID) (oauth.OAuthCredential, error) {
	c, ok := m.creds[id]
	if !ok { return oauth.OAuthCredential{}, oauth.ErrNotFound }
	return c, nil
}

func (m *mockRepo) Create(_ context.Context, _ string, c oauth.OAuthCredential) (oauth.OAuthCredential, error) {
	c.ID = uuid.New()
	m.creds[c.ID] = c
	return c, nil
}

func (m *mockRepo) Update(_ context.Context, _ string, id uuid.UUID, c oauth.OAuthCredential) (oauth.OAuthCredential, error) {
	if _, ok := m.creds[id]; !ok { return oauth.OAuthCredential{}, oauth.ErrNotFound }
	c.ID = id
	m.creds[id] = c
	return c, nil
}

func (m *mockRepo) Delete(_ context.Context, _ string, id uuid.UUID) error {
	if _, ok := m.creds[id]; !ok { return oauth.ErrNotFound }
	delete(m.creds, id)
	return nil
}

// mockHTTPClient captures requests and returns a configurable response.
type mockHTTPClient struct {
	calls    int
	body     string
	status   int
}

func (c *mockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	c.calls++
	statusCode := c.status
	if statusCode == 0 { statusCode = http.StatusOK }
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(c.body)),
	}, nil
}

const tenantID = "test-tenant"

// helpers

func createCred(t *testing.T, svc *oauth.Service, req oauth.CreateRequest) oauth.OAuthCredential {
	t.Helper()
	c, err := svc.Create(context.Background(), tenantID, req)
	require.NoError(t, err)
	return c
}

// ---- basic CRUD tests ----

func TestOAuthService_Create_GetByID(t *testing.T) {
	svc := oauth.NewService(newMockRepo())
	c := createCred(t, svc, oauth.CreateRequest{
		Name: "My API Key", AuthType: oauth.AuthTypeAPIKey, APIKeyValue: "s3cr3t", APIKeyHeader: "X-Key",
	})
	assert.NotEqual(t, uuid.Nil, c.ID)
	fetched, err := svc.GetByID(context.Background(), tenantID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, c.ID, fetched.ID)
}

func TestOAuthService_Delete_NotFound(t *testing.T) {
	svc := oauth.NewService(newMockRepo())
	err := svc.Delete(context.Background(), tenantID, uuid.New())
	require.ErrorIs(t, err, oauth.ErrNotFound)
}

func TestOAuthService_ListAll(t *testing.T) {
	svc := oauth.NewService(newMockRepo())
	for i := 0; i < 3; i++ {
		createCred(t, svc, oauth.CreateRequest{Name: "c", AuthType: oauth.AuthTypeAPIKey})
	}
	items, total, err := svc.ListAll(context.Background(), tenantID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, 3, total); assert.Len(t, items, 3)
}

// ---- ResolveAuthHeader tests (non-OAuth2) ----

func TestOAuthService_ResolveAuthHeader_APIKey(t *testing.T) {
	svc := oauth.NewService(newMockRepo())
	c := createCred(t, svc, oauth.CreateRequest{
		Name: "k", AuthType: oauth.AuthTypeAPIKey, APIKeyHeader: "X-My-Key", APIKeyValue: "token123",
	})
	res, err := svc.ResolveAuthHeader(context.Background(), tenantID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "X-My-Key", res.Header)
	assert.Equal(t, "token123", res.Value)
}

func TestOAuthService_ResolveAuthHeader_APIKey_DefaultHeader(t *testing.T) {
	svc := oauth.NewService(newMockRepo())
	c := createCred(t, svc, oauth.CreateRequest{
		Name: "k", AuthType: oauth.AuthTypeAPIKey, APIKeyValue: "mykey",
	})
	res, err := svc.ResolveAuthHeader(context.Background(), tenantID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "X-API-Key", res.Header)
}

func TestOAuthService_ResolveAuthHeader_BearerToken(t *testing.T) {
	svc := oauth.NewService(newMockRepo())
	c := createCred(t, svc, oauth.CreateRequest{
		Name: "b", AuthType: oauth.AuthTypeBearerToken, BearerToken: "jwt.token.here",
	})
	res, err := svc.ResolveAuthHeader(context.Background(), tenantID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "Authorization", res.Header)
	assert.Equal(t, "Bearer jwt.token.here", res.Value)
}

func TestOAuthService_ResolveAuthHeader_BasicAuth(t *testing.T) {
	svc := oauth.NewService(newMockRepo())
	c := createCred(t, svc, oauth.CreateRequest{
		Name: "basic", AuthType: oauth.AuthTypeBasicAuth, Username: "user", Password: "pass",
	})
	res, err := svc.ResolveAuthHeader(context.Background(), tenantID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "Authorization", res.Header)
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	assert.Equal(t, expected, res.Value)
}

// ---- OAuth2 Client Credentials token exchange tests ----

func TestOAuthService_ResolveAuthHeader_OAuth2_FetchesToken(t *testing.T) {
	httpClient := &mockHTTPClient{
		body:   `{"access_token":"my-token","expires_in":3600}`,
		status: http.StatusOK,
	}
	repo := newMockRepo()
	svc := oauth.NewServiceWithClient(repo, httpClient)

	c := createCred(t, svc, oauth.CreateRequest{
		Name:         "cc",
		AuthType:     oauth.AuthTypeOAuth2ClientCredentials,
		TokenURL:     "https://auth.example.com/token",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	})

	res, err := svc.ResolveAuthHeader(context.Background(), tenantID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "Authorization", res.Header)
	assert.Equal(t, "Bearer my-token", res.Value)
	assert.Equal(t, 1, httpClient.calls)
}

func TestOAuthService_ResolveAuthHeader_OAuth2_UsesCache(t *testing.T) {
	httpClient := &mockHTTPClient{
		body:   `{"access_token":"cached-token","expires_in":3600}`,
		status: http.StatusOK,
	}
	repo := newMockRepo()
	svc := oauth.NewServiceWithClient(repo, httpClient)

	c := createCred(t, svc, oauth.CreateRequest{
		Name: "cc", AuthType: oauth.AuthTypeOAuth2ClientCredentials,
		TokenURL: "https://auth.example.com/token",
		ClientID: "cid", ClientSecret: "cs",
	})

	// First call — fetches from token endpoint
	_, err := svc.ResolveAuthHeader(context.Background(), tenantID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, httpClient.calls)

	// Second call — must use cache, no additional HTTP call
	res, err := svc.ResolveAuthHeader(context.Background(), tenantID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "Bearer cached-token", res.Value)
	assert.Equal(t, 1, httpClient.calls, "token endpoint must not be called again")
}

func TestOAuthService_ResolveAuthHeader_OAuth2_TokenEndpointError(t *testing.T) {
	httpClient := &mockHTTPClient{body: `{"error":"invalid_client"}`, status: http.StatusUnauthorized}
	repo := newMockRepo()
	svc := oauth.NewServiceWithClient(repo, httpClient)

	c := createCred(t, svc, oauth.CreateRequest{
		Name: "cc", AuthType: oauth.AuthTypeOAuth2ClientCredentials,
		TokenURL: "https://auth.example.com/token",
		ClientID: "bad", ClientSecret: "bad",
	})

	_, err := svc.ResolveAuthHeader(context.Background(), tenantID, c.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestOAuthService_Update_ClearsTokenCache(t *testing.T) {
	calls := 0
	httpClient := &mockHTTPClient{}
	// First response is old token, second is new token
	httpClient.body = `{"access_token":"old-token","expires_in":3600}`

	repo := newMockRepo()
	svc := oauth.NewServiceWithClient(repo, httpClient)

	c := createCred(t, svc, oauth.CreateRequest{
		Name: "cc", AuthType: oauth.AuthTypeOAuth2ClientCredentials,
		TokenURL: "https://auth.example.com/token",
		ClientID: "cid", ClientSecret: "cs",
	})

	// Populate cache
	_, err := svc.ResolveAuthHeader(context.Background(), tenantID, c.ID)
	require.NoError(t, err)
	calls = httpClient.calls

	// Update credential — must clear cache
	httpClient.body = `{"access_token":"new-token","expires_in":3600}`
	_, err = svc.Update(context.Background(), tenantID, c.ID, oauth.CreateRequest{
		Name: "cc", AuthType: oauth.AuthTypeOAuth2ClientCredentials,
		TokenURL: "https://auth.example.com/token",
		ClientID: "cid2", ClientSecret: "cs2",
	})
	require.NoError(t, err)

	// Next resolve must fetch a new token
	res, err := svc.ResolveAuthHeader(context.Background(), tenantID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "Bearer new-token", res.Value)
	assert.Equal(t, calls+1, httpClient.calls, "should have fetched a new token after update")
}
