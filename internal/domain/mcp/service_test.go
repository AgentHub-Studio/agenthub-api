package mcp_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

const mcpTenantID = "test-tenant"

type mcpRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f mcpRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func withDefaultMCPHTTPTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = previous
	})
}

func newMockRepo() *mockMCPRepo {
	return &mockMCPRepo{data: make(map[uuid.UUID]mcp.McpServerConfig)}
}

func strPtr(s string) *string { return &s }

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

type callbackMCPRepo struct {
	*mockMCPRepo
	getByIDCalls int
}

func (m *callbackMCPRepo) GetByID(ctx context.Context, id uuid.UUID) (mcp.McpServerConfig, error) {
	m.getByIDCalls++
	return m.mockMCPRepo.GetByID(ctx, id)
}

type callbackOAuthSvc struct {
	*mockOAuthSvc
	exchangeErr   error
	exchangeCalls int
	tenantID      string
	credentialID  uuid.UUID
	code          string
}

func (m *callbackOAuthSvc) ExchangeCode(_ context.Context, tenantID string, credentialID uuid.UUID, code string) error {
	m.exchangeCalls++
	m.tenantID = tenantID
	m.credentialID = credentialID
	m.code = code
	return m.exchangeErr
}

func newCallbackOAuthSvc() *callbackOAuthSvc {
	return &callbackOAuthSvc{mockOAuthSvc: &mockOAuthSvc{credentials: make(map[uuid.UUID]oauth.OAuthCredential)}}
}

func TestMCPService_Create_Success(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	c, err := svc.Create(context.Background(), mcp.CreateRequest{
		Name:          "filesystem",
		TransportType: "http", HTTPBaseURL: strPtr("https://example.com/mcp"),
	})
	require.NoError(t, err)
	assert.Equal(t, "filesystem", c.Name)
	assert.NotEqual(t, uuid.Nil, c.ID)
}

func TestMCPService_Create_StdioPersistsExecutableConfig(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	c, err := svc.Create(context.Background(), mcp.CreateRequest{
		Name:          "filesystem",
		TransportType: "stdio",
		Command:       strPtr("  npx  "),
		Args:          []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
		Env:           map[string]string{"LOG_LEVEL": "info"},
		AutoStart:     true,
		Enabled:       true,
	})

	require.NoError(t, err)
	assert.Equal(t, "stdio", c.TransportType)
	require.NotNil(t, c.Command)
	assert.Equal(t, "npx", *c.Command)
	assert.Nil(t, c.HTTPBaseURL)
	assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"}, c.Args)
	assert.Equal(t, "info", c.Env["LOG_LEVEL"])
}

func TestMCPService_Create_StdioRequiresCommandAndRejectsOAuth(t *testing.T) {
	svc := mcp.NewService(newMockRepo())

	_, err := svc.Create(context.Background(), mcp.CreateRequest{
		Name:          "missing-command",
		TransportType: "stdio",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command is required for stdio transport")

	credentialID := uuid.New()
	_, err = svc.Create(context.Background(), mcp.CreateRequest{
		Name:              "stdio-oauth",
		TransportType:     "stdio",
		Command:           strPtr("node"),
		OAuthCredentialID: &credentialID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oauthCredentialId is only supported for http transport")
}

func TestMCPService_Update_HTTPToStdioClearsHTTPOnlyState(t *testing.T) {
	repo := newMockRepo()
	serverID := uuid.New()
	httpURL := "https://example.com/mcp"
	credentialID := uuid.New()
	repo.data[serverID] = mcp.McpServerConfig{
		ID:                serverID,
		Name:              "switchable",
		TransportType:     "http",
		HTTPBaseURL:       &httpURL,
		OAuthCredentialID: &credentialID,
	}
	svc := mcp.NewService(repo)
	transport := "stdio"
	command := "node"

	updated, err := svc.Update(context.Background(), serverID, mcp.UpdateRequest{
		TransportType: &transport,
		Command:       &command,
	})

	require.NoError(t, err)
	assert.Equal(t, "stdio", updated.TransportType)
	assert.Nil(t, updated.HTTPBaseURL)
	assert.Nil(t, updated.OAuthCredentialID)
	require.NotNil(t, updated.Command)
	assert.Equal(t, "node", *updated.Command)
}

func TestMCPService_Update_StdioRejectsInvalidEnvironmentName(t *testing.T) {
	repo := newMockRepo()
	serverID := uuid.New()
	command := "node"
	repo.data[serverID] = mcp.McpServerConfig{
		ID:            serverID,
		Name:          "filesystem",
		TransportType: "stdio",
		Command:       &command,
	}
	svc := mcp.NewService(repo)
	env := map[string]string{"bad name": "value"}

	_, err := svc.Update(context.Background(), serverID, mcp.UpdateRequest{Env: &env})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "env name \"bad name\" invalid")
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
	_, err := svc.Create(context.Background(), mcp.CreateRequest{Name: "auto", TransportType: "http", HTTPBaseURL: strPtr("https://example.com/mcp"), AutoStart: autoStart})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), mcp.CreateRequest{Name: "manual", TransportType: "http", HTTPBaseURL: strPtr("https://example.com/mcp")})
	require.NoError(t, err)
	items, err := svc.ListAutoStart(context.Background())
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestMCPService_List(t *testing.T) {
	svc := mcp.NewService(newMockRepo())
	for _, name := range []string{"fs", "github", "slack"} {
		_, err := svc.Create(context.Background(), mcp.CreateRequest{Name: name, TransportType: "http", HTTPBaseURL: strPtr("https://example.com/mcp")})
		require.NoError(t, err)
	}
	items, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, items, 3)
}

func TestMCPResponse_RedactsSensitiveEnvValues(t *testing.T) {
	cfg := mcp.McpServerConfig{
		ID:            uuid.New(),
		Name:          "github",
		TransportType: "http",
		Env: map[string]string{
			"GITHUB_TOKEN": "ghp-mcp-env-token",
			"API_KEY":      "mcp-api-key-secret",
			"MCP_SECRET":   "mcp-secret-value",
			"SAFE_FLAG":    "enabled",
		},
	}

	resp := mcp.ResponseFrom(cfg)
	body, err := json.Marshal(resp)
	require.NoError(t, err)

	assert.NotContains(t, string(body), "ghp-mcp-env-token")
	assert.NotContains(t, string(body), "mcp-api-key-secret")
	assert.NotContains(t, string(body), "mcp-secret-value")
	assert.Contains(t, string(body), `"GITHUB_TOKEN":"***"`)
	assert.Contains(t, string(body), `"API_KEY":"***"`)
	assert.Contains(t, string(body), `"MCP_SECRET":"***"`)
	assert.Contains(t, string(body), `"SAFE_FLAG":"enabled"`)
	assert.Equal(t, "ghp-mcp-env-token", cfg.Env["GITHUB_TOKEN"])
}

func TestMCPResponse_RedactsEmbeddedCredentialsInSafeNamedEnvURLs(t *testing.T) {
	cfg := mcp.McpServerConfig{
		ID:            uuid.New(),
		Name:          "database-tool",
		TransportType: "stdio",
		Env: map[string]string{
			"DATABASE_URL": "postgres://audit_user:audit-database-url-secret@db.example.test:5432/agenthub?sslmode=require",
			"REDIS_URL":    "redis://:audit-redis-url-secret@redis.example.test:6379/0",
			"SERVICE_URL":  "https://service.example.test/v1?api_key=audit-query-url-secret&safe=true",
			"SAFE_FLAG":    "enabled",
		},
	}

	resp := mcp.ResponseFrom(cfg)
	body, err := json.Marshal(resp)
	require.NoError(t, err)

	for _, secret := range []string{
		"audit-database-url-secret",
		"audit-redis-url-secret",
		"audit-query-url-secret",
	} {
		assert.NotContains(t, string(body), secret)
	}
	assert.Contains(t, resp.Env["DATABASE_URL"], "audit_user:%2A%2A%2A@")
	assert.Contains(t, resp.Env["REDIS_URL"], ":%2A%2A%2A@")
	assert.Contains(t, resp.Env["SERVICE_URL"], "api_key=%2A%2A%2A")
	assert.Equal(t, "enabled", resp.Env["SAFE_FLAG"])
	assert.Equal(t, "postgres://audit_user:audit-database-url-secret@db.example.test:5432/agenthub?sslmode=require", cfg.Env["DATABASE_URL"])
}

func FuzzMCPResponseRedactsEmbeddedCredentialsInEnvURLs(f *testing.F) {
	f.Add("postgres", "audit_user", "database-password", "query-api-key")
	f.Add("redis", "", "redis-password", "query-token")

	f.Fuzz(func(t *testing.T, scheme, user, password, querySecret string) {
		validScheme := "postgres"
		if len(scheme)%2 == 1 {
			validScheme = "redis"
		}
		passwordMarker := fuzzSecretMarker("password", password)
		queryMarker := fuzzSecretMarker("query", querySecret)
		rawURL := (&url.URL{
			Scheme:   validScheme,
			Host:     "db.example.test:5432",
			User:     url.UserPassword(user, passwordMarker),
			Path:     "/agenthub",
			RawQuery: url.Values{"api_key": []string{queryMarker}, "safe": []string{"true"}}.Encode(),
		}).String()
		cfg := mcp.McpServerConfig{Env: map[string]string{"DATABASE_URL": rawURL}}

		resp := mcp.ResponseFrom(cfg)
		body, err := json.Marshal(resp)
		require.NoError(t, err)
		assert.NotContains(t, string(body), passwordMarker)
		assert.NotContains(t, string(body), queryMarker)
		assert.Equal(t, rawURL, cfg.Env["DATABASE_URL"])
	})
}

func fuzzSecretMarker(prefix, value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("audit-%s-%x", prefix, sum[:])
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

func TestMCPService_ListToolsUsesEscapedRuntimeEndpoint(t *testing.T) {
	repo := newMockRepo()
	serverID := uuid.New()
	repo.data[serverID] = mcp.McpServerConfig{
		ID:            serverID,
		Name:          "space server",
		TransportType: "http",
		Enabled:       true,
	}

	var paths []string
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.EscapedPath())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case "/runtime/servers/space%20server/status":
			_ = json.NewEncoder(w).Encode(map[string]string{"Status": "running"})
		case "/runtime/servers/space%20server/tools":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tools": []map[string]any{
					{
						"name":        "search",
						"description": "Search documents",
						"inputSchema": map[string]any{"type": "object"},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer runtime.Close()

	svc := mcp.NewService(repo).WithRuntimeURL(runtime.URL + "/runtime")
	tools, err := svc.ListTools(context.Background(), serverID)

	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "search", tools[0].Name)
	assert.Contains(t, paths, "/runtime/servers/space%20server/status")
	assert.Contains(t, paths, "/runtime/servers/space%20server/tools")
}

func TestMCPService_GetConnectURL_BlocksLegacySSRFBaseURLBeforeRequest(t *testing.T) {
	repo := newMockRepo()
	serverID := uuid.New()
	legacyURL := "http://localhost/mcp"
	repo.data[serverID] = mcp.McpServerConfig{
		ID:            serverID,
		Name:          "legacy-local",
		TransportType: "http",
		HTTPBaseURL:   &legacyURL,
	}

	var calls int
	withDefaultMCPHTTPTransport(t, mcpRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("unexpected outbound request")
	}))

	_, err := mcp.NewService(repo).WithAllowedRedirectOrigins([]string{"https://app.example.test"}).GetConnectURL(
		tenant.NewContext(context.Background(), mcpTenantID),
		serverID,
		"https://app.example.test/oauth/callback",
	)

	require.Error(t, err)
	assert.Zero(t, calls)
	assert.Contains(t, err.Error(), "URL is not allowed")
}

func TestMCPService_GetConnectURL_BlocksRedirectToSSRFURL(t *testing.T) {
	repo := newMockRepo()
	serverID := uuid.New()
	baseURL := "https://1.1.1.1/mcp"
	repo.data[serverID] = mcp.McpServerConfig{
		ID:            serverID,
		Name:          "redirecting-mcp",
		TransportType: "http",
		HTTPBaseURL:   &baseURL,
	}

	var blockedTargetReached bool
	withDefaultMCPHTTPTransport(t, mcpRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "1.1.1.1":
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"http://localhost/mcp"}},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		case "localhost":
			blockedTargetReached = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		default:
			return nil, errors.New("unexpected host")
		}
	}))

	_, err := mcp.NewService(repo).WithAllowedRedirectOrigins([]string{"https://app.example.test"}).GetConnectURL(
		tenant.NewContext(context.Background(), mcpTenantID),
		serverID,
		"https://app.example.test/oauth/callback",
	)

	require.Error(t, err)
	assert.False(t, blockedTargetReached)
}

func TestMCPService_GetConnectURL_BlocksInternalAuthorizationServerMetadata(t *testing.T) {
	repo := newMockRepo()
	serverID := uuid.New()
	credentialID := uuid.New()
	baseURL := "https://1.1.1.1/mcp"
	clientID := "known-client"
	repo.data[serverID] = mcp.McpServerConfig{
		ID:                serverID,
		Name:              "metadata-mcp",
		TransportType:     "http",
		HTTPBaseURL:       &baseURL,
		OAuthCredentialID: &credentialID,
	}

	var blockedTargetReached bool
	withDefaultMCPHTTPTransport(t, mcpRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "1.1.1.1":
			if req.URL.EscapedPath() == "/.well-known/oauth-protected-resource/mcp" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(
						`{"authorization_servers":["http://localhost/auth"]}`,
					)),
				}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
		case "localhost":
			blockedTargetReached = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					`{"authorization_endpoint":"https://1.1.1.1/authorize","token_endpoint":"https://1.1.1.1/token"}`,
				)),
			}, nil
		default:
			return nil, errors.New("unexpected host")
		}
	}))

	svc := mcp.NewService(repo).WithAllowedRedirectOrigins([]string{"https://app.example.test"}).WithOAuthService(&mockOAuthSvc{credentials: map[uuid.UUID]oauth.OAuthCredential{
		credentialID: {ID: credentialID, ClientID: &clientID},
	}})
	_, err := svc.GetConnectURL(
		tenant.NewContext(context.Background(), mcpTenantID),
		serverID,
		"https://app.example.test/oauth/callback",
	)

	require.Error(t, err)
	assert.False(t, blockedTargetReached)
	assert.Contains(t, err.Error(), "URL is not allowed")
}

func TestMCPService_GetConnectURL_RejectsUntrustedRedirectBeforeDiscovery(t *testing.T) {
	repo := newMockRepo()
	serverID := uuid.New()
	baseURL := "https://1.1.1.1/mcp"
	repo.data[serverID] = mcp.McpServerConfig{
		ID:            serverID,
		Name:          "redirect-guard",
		TransportType: "http",
		HTTPBaseURL:   &baseURL,
	}

	var calls int
	withDefaultMCPHTTPTransport(t, mcpRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("discovery must not be called for an untrusted redirect")
	}))

	for _, tc := range []struct {
		name        string
		redirectURL string
	}{
		{name: "untrusted origin", redirectURL: "https://attacker.example/callback"},
		{name: "oversized URL", redirectURL: "https://app.example.test/" + strings.Repeat("a", 2048)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mcp.NewService(repo).
				WithAllowedRedirectOrigins([]string{"https://app.example.test", "https://*.cezar.dev"}).
				GetConnectURL(
					tenant.NewContext(context.Background(), mcpTenantID),
					serverID,
					tc.redirectURL,
				)

			require.ErrorIs(t, err, mcp.ErrRedirectURLNotAllowed)
		})
	}
	assert.Zero(t, calls)
}

func TestMCPService_ListToolsRejectsInvalidRuntimeURL(t *testing.T) {
	repo := newMockRepo()
	serverID := uuid.New()
	repo.data[serverID] = mcp.McpServerConfig{
		ID:            serverID,
		Name:          "server",
		TransportType: "http",
		Enabled:       true,
	}

	svc := mcp.NewService(repo).WithRuntimeURL("https://user:pass@runtime.example")
	_, err := svc.ListTools(context.Background(), serverID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid MCP runtime URL")
}

func TestMCPService_HandleOAuthCallback_RejectsBeforeLookupOrExchange(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		code string
		want string
	}{
		{
			name: "missing tenant",
			ctx:  context.Background(),
			code: "authorization-code",
			want: "tenant context is required",
		},
		{
			name: "blank authorization code",
			ctx:  tenant.NewContext(context.Background(), mcpTenantID),
			code: " \t\n ",
			want: "authorization code is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &callbackMCPRepo{mockMCPRepo: newMockRepo()}
			oauthSvc := newCallbackOAuthSvc()
			err := mcp.NewService(repo).WithOAuthService(oauthSvc).HandleOAuthCallback(tt.ctx, uuid.New(), tt.code)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.Zero(t, repo.getByIDCalls)
			assert.Zero(t, oauthSvc.exchangeCalls)
		})
	}
}

func TestMCPService_HandleOAuthCallback_StopsBeforeExchangeForInvalidConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     *mcp.McpServerConfig
		withOAuth  bool
		wantError  string
		wantLookup int
	}{
		{
			name:       "MCP server is missing",
			wantError:  "mcp: not found",
			wantLookup: 1,
		},
		{
			name: "no linked OAuth credential",
			config: &mcp.McpServerConfig{
				Name: "calendar",
			},
			withOAuth:  true,
			wantError:  "has no linked OAuth credential",
			wantLookup: 1,
		},
		{
			name: "OAuth service unavailable",
			config: &mcp.McpServerConfig{
				Name:              "calendar",
				OAuthCredentialID: ptrUUID(uuid.New()),
			},
			wantError:  "oauth service not available",
			wantLookup: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &callbackMCPRepo{mockMCPRepo: newMockRepo()}
			serverID := uuid.New()
			if tt.config != nil {
				config := *tt.config
				config.ID = serverID
				repo.data[serverID] = config
			}

			svc := mcp.NewService(repo)
			var oauthSvc *callbackOAuthSvc
			if tt.withOAuth {
				oauthSvc = newCallbackOAuthSvc()
				svc.WithOAuthService(oauthSvc)
			}

			err := svc.HandleOAuthCallback(tenant.NewContext(context.Background(), mcpTenantID), serverID, "authorization-code")

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
			assert.Equal(t, tt.wantLookup, repo.getByIDCalls)
			if oauthSvc != nil {
				assert.Zero(t, oauthSvc.exchangeCalls)
			}
		})
	}
}

func TestMCPService_HandleOAuthCallback_ExchangeFailureDoesNotReloadRuntime(t *testing.T) {
	repo := &callbackMCPRepo{mockMCPRepo: newMockRepo()}
	serverID := uuid.New()
	credentialID := uuid.New()
	repo.data[serverID] = mcp.McpServerConfig{
		ID:                serverID,
		Name:              "calendar",
		OAuthCredentialID: &credentialID,
	}
	oauthSvc := newCallbackOAuthSvc()
	oauthSvc.exchangeErr = errors.New("token endpoint unavailable")

	var runtimeCalls int
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runtimeCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(runtime.Close)

	err := mcp.NewService(repo).
		WithOAuthService(oauthSvc).
		WithRuntimeURL(runtime.URL).
		HandleOAuthCallback(tenant.NewContext(context.Background(), mcpTenantID), serverID, " authorization-code ")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "token exchange failed")
	assert.Equal(t, 1, oauthSvc.exchangeCalls)
	assert.Equal(t, mcpTenantID, oauthSvc.tenantID)
	assert.Equal(t, credentialID, oauthSvc.credentialID)
	assert.Equal(t, "authorization-code", oauthSvc.code)
	assert.Zero(t, runtimeCalls)
}

func TestMCPService_HandleOAuthCallback_ExchangesAndReloadsRuntime(t *testing.T) {
	repo := &callbackMCPRepo{mockMCPRepo: newMockRepo()}
	serverID := uuid.New()
	credentialID := uuid.New()
	repo.data[serverID] = mcp.McpServerConfig{
		ID:                serverID,
		Name:              "calendar",
		OAuthCredentialID: &credentialID,
	}
	oauthSvc := newCallbackOAuthSvc()

	var runtimeCalls int
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runtimeCalls++
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/servers/calendar", r.URL.EscapedPath())
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(runtime.Close)

	err := mcp.NewService(repo).
		WithOAuthService(oauthSvc).
		WithRuntimeURL(runtime.URL).
		HandleOAuthCallback(tenant.NewContext(context.Background(), mcpTenantID), serverID, "authorization-code")

	require.NoError(t, err)
	assert.Equal(t, 1, repo.getByIDCalls)
	assert.Equal(t, 1, oauthSvc.exchangeCalls)
	assert.Equal(t, mcpTenantID, oauthSvc.tenantID)
	assert.Equal(t, credentialID, oauthSvc.credentialID)
	assert.Equal(t, "authorization-code", oauthSvc.code)
	assert.Equal(t, 1, runtimeCalls)
}

func ptrUUID(value uuid.UUID) *uuid.UUID {
	return &value
}
