package mcp_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockMCPSvc satisfies the private mcpService interface in mcp.Handler.
type mockMCPSvc struct {
	configs       map[uuid.UUID]mcp.McpServerConfigResponse
	connectErr    error
	callbackErr   error
	callbackCalls int
	callbackID    uuid.UUID
	callbackCode  string
}

func newMockMCPSvc() *mockMCPSvc {
	return &mockMCPSvc{configs: make(map[uuid.UUID]mcp.McpServerConfigResponse)}
}

func (m *mockMCPSvc) List(_ context.Context) ([]mcp.McpServerConfigResponse, error) {
	items := make([]mcp.McpServerConfigResponse, 0, len(m.configs))
	for _, c := range m.configs {
		items = append(items, c)
	}
	return items, nil
}

func (m *mockMCPSvc) ListAutoStart(_ context.Context) ([]mcp.McpServerConfigResponse, error) {
	var items []mcp.McpServerConfigResponse
	for _, c := range m.configs {
		if c.AutoStart {
			items = append(items, c)
		}
	}
	return items, nil
}

func (m *mockMCPSvc) ListAllEnabled(_ context.Context) ([]mcp.McpServerConfigResponse, error) {
	var items []mcp.McpServerConfigResponse
	for _, c := range m.configs {
		if c.Enabled {
			items = append(items, c)
		}
	}
	return items, nil
}

func (m *mockMCPSvc) ListBootstrap(_ context.Context) ([]mcp.McpServerConfigBootstrapResponse, error) {
	var items []mcp.McpServerConfigBootstrapResponse
	for _, c := range m.configs {
		if !c.Enabled {
			continue
		}
		items = append(items, mcp.McpServerConfigBootstrapResponse{
			ID:            c.ID,
			Name:          c.Name,
			TransportType: c.TransportType,
			AutoStart:     c.AutoStart,
			Enabled:       c.Enabled,
		})
	}
	return items, nil
}

func (m *mockMCPSvc) Create(_ context.Context, req mcp.CreateRequest) (mcp.McpServerConfigResponse, error) {
	id := uuid.New()
	resp := mcp.McpServerConfigResponse{
		ID:            id,
		Name:          req.Name,
		TransportType: req.TransportType,
	}
	m.configs[id] = resp
	return resp, nil
}

func (m *mockMCPSvc) GetByID(_ context.Context, id uuid.UUID) (mcp.McpServerConfigResponse, error) {
	c, ok := m.configs[id]
	if !ok {
		return mcp.McpServerConfigResponse{}, mcp.ErrNotFound
	}
	return c, nil
}

func (m *mockMCPSvc) Update(_ context.Context, id uuid.UUID, req mcp.UpdateRequest) (mcp.McpServerConfigResponse, error) {
	c, ok := m.configs[id]
	if !ok {
		return mcp.McpServerConfigResponse{}, mcp.ErrNotFound
	}
	if req.Name != nil {
		c.Name = *req.Name
	}
	m.configs[id] = c
	return c, nil
}

func (m *mockMCPSvc) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.configs[id]; !ok {
		return mcp.ErrNotFound
	}
	delete(m.configs, id)
	return nil
}

func (m *mockMCPSvc) GetAuthStatus(_ context.Context, _ uuid.UUID) (mcp.AuthStatusResponse, error) {
	return mcp.AuthStatusResponse{}, nil
}

func (m *mockMCPSvc) GetConnectURL(_ context.Context, _ uuid.UUID, _ string) (mcp.ConnectURLResponse, error) {
	return mcp.ConnectURLResponse{}, m.connectErr
}

func (m *mockMCPSvc) HandleOAuthCallback(_ context.Context, id uuid.UUID, code string) error {
	m.callbackCalls++
	m.callbackID = id
	m.callbackCode = code
	return m.callbackErr
}

func (m *mockMCPSvc) ListTools(_ context.Context, _ uuid.UUID) ([]mcp.ToolResponse, error) {
	return nil, nil
}

func setupMCP() (*chi.Mux, *mockMCPSvc) {
	return setupMCPWithRoles("admin")
}

func setupMCPWithRoles(roles ...string) (*chi.Mux, *mockMCPSvc) {
	svc := newMockMCPSvc()
	h := mcp.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(middleware.ContextWithRoles(r.Context(), roles...)))
		})
	})
	h.RegisterRoutes(r)
	return r, svc
}

func setupMCPWithAuth(t *testing.T, roles ...string) (*chi.Mux, *mockMCPSvc, string) {
	t.Helper()

	realm := "mcp-test-" + uuid.NewString()
	key, keycloakURL := setupFakeKeycloak(t, realm)
	token := signMCPJWT(t, key, realm, keycloakURL, roles)

	svc := newMockMCPSvc()
	h := mcp.NewHandler(svc)
	r := chi.NewRouter()
	chain := middleware.New(keycloakURL, []string{"https://app.cezar.dev"})
	for _, mw := range chain.Protected() {
		r.Use(mw)
	}
	h.RegisterRoutes(r)
	return r, svc, token
}

func setupFakeKeycloak(t *testing.T, realm string) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwksDoc := map[string]any{
		"keys": []map[string]any{
			{
				"kid": "mcp-test-kid",
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/realms/%s/protocol/openid-connect/certs", realm), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwksDoc)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return key, srv.URL
}

type mcpTestAccessRoles struct {
	Roles []string `json:"roles"`
}

type mcpTestClaims struct {
	jwt.RegisteredClaims
	RealmAccess mcpTestAccessRoles `json:"realm_access"`
}

func signMCPJWT(t *testing.T, key *rsa.PrivateKey, realm, keycloakURL string, roles []string) string {
	t.Helper()
	claims := mcpTestClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    fmt.Sprintf("%s/realms/%s", keycloakURL, realm),
			Subject:   "mcp-runtime",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		RealmAccess: mcpTestAccessRoles{Roles: roles},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "mcp-test-kid"
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func TestMCPHandler_List_Success(t *testing.T) {
	r, svc := setupMCP()
	id := uuid.New()
	svc.configs[id] = mcp.McpServerConfigResponse{ID: id, Name: "filesystem", TransportType: "stdio"}

	req := httptest.NewRequest(http.MethodGet, "/api/mcp-server-configs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[mcp.McpServerConfigResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestMCPHandler_Create_Success(t *testing.T) {
	r, _ := setupMCP()
	body, _ := json.Marshal(mcp.CreateRequest{Name: "my-server", TransportType: "stdio"})
	req := httptest.NewRequest(http.MethodPost, "/api/mcp-server-configs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp mcp.McpServerConfigResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "my-server", resp.Name)
}

func TestMCPHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupMCP()
	req := httptest.NewRequest(http.MethodPost, "/api/mcp-server-configs", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMCPHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, svc := setupMCP()
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/mcp-server-configs",
			bytes.NewBufferString(`{"name":"first","transportType":"stdio"}{"name":"ignored"}`),
		)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.configs)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupMCP()
		id := uuid.New()
		svc.configs[id] = mcp.McpServerConfigResponse{ID: id, Name: "original", TransportType: "stdio"}
		req := httptest.NewRequest(
			http.MethodPut,
			"/api/mcp-server-configs/"+id.String(),
			bytes.NewBufferString(`{"name":"changed"}{"name":"ignored"}`),
		)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, "original", svc.configs[id].Name)
	})

	t.Run("callback", func(t *testing.T) {
		r, svc := setupMCP()
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/mcp-server-configs/"+uuid.NewString()+"/callback",
			bytes.NewBufferString(`{"code":"authorization-code"}{"code":"ignored"}`),
		)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Zero(t, svc.callbackCalls)
	})
}

func TestMCPHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupMCP()
	req := httptest.NewRequest(http.MethodGet, "/api/mcp-server-configs/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMCPHandler_Delete_Success(t *testing.T) {
	r, svc := setupMCP()
	id := uuid.New()
	svc.configs[id] = mcp.McpServerConfigResponse{ID: id, Name: "to-delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/mcp-server-configs/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestMCPHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupMCP()
	req := httptest.NewRequest(http.MethodDelete, "/api/mcp-server-configs/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMCPHandler_Connect_InvalidRedirectURL_ReturnsBadRequest(t *testing.T) {
	r, svc := setupMCP()
	svc.connectErr = mcp.ErrRedirectURLNotAllowed

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/mcp-server-configs/"+uuid.NewString()+"/connect?redirectUrl=https%3A%2F%2Fattacker.example%2Fcallback",
		nil,
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid redirectUrl")
	assert.NotContains(t, w.Body.String(), "attacker.example")
}

func TestMCPHandler_Callback_ValidatesCodeBeforeService(t *testing.T) {
	r, svc := setupMCP()
	id := uuid.New()

	for _, body := range []string{`{}`, `{"code":""}`, `{"code":"   \t\n"}`, `not-json`} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/mcp-server-configs/"+id.String()+"/callback", bytes.NewBufferString(body))
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		assert.Contains(t, w.Body.String(), "code is required", body)
	}
	assert.Zero(t, svc.callbackCalls)
}

func TestMCPHandler_Callback_UsesTrimmedCodeAndMapsErrors(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		r, svc := setupMCP()
		id := uuid.New()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/mcp-server-configs/"+id.String()+"/callback", bytes.NewBufferString(`{"code":"  authorization-code  "}`))
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 1, svc.callbackCalls)
		assert.Equal(t, id, svc.callbackID)
		assert.Equal(t, "authorization-code", svc.callbackCode)
		assert.JSONEq(t, `{"status":"connected"}`, w.Body.String())
	})

	t.Run("not found", func(t *testing.T) {
		r, svc := setupMCP()
		svc.callbackErr = mcp.ErrNotFound
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/mcp-server-configs/"+uuid.NewString()+"/callback", bytes.NewBufferString(`{"code":"authorization-code"}`))
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, 1, svc.callbackCalls)
	})

	t.Run("internal error is not reflected", func(t *testing.T) {
		r, svc := setupMCP()
		svc.callbackErr = errors.New("token endpoint http://internal.example failed")
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/mcp-server-configs/"+uuid.NewString()+"/callback", bytes.NewBufferString(`{"code":"authorization-code"}`))
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "internal error")
		assert.NotContains(t, w.Body.String(), "internal.example")
	})
}

func TestMCPHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupMCPWithRoles("user")
	id := uuid.NewString()
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/mcp-server-configs"},
		{name: "create", method: http.MethodPost, path: "/api/mcp-server-configs", body: `{"name":"server","transportType":"http"}`},
		{name: "get", method: http.MethodGet, path: "/api/mcp-server-configs/" + id},
		{name: "put", method: http.MethodPut, path: "/api/mcp-server-configs/" + id, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/mcp-server-configs/" + id, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/mcp-server-configs/" + id},
		{name: "auth status", method: http.MethodGet, path: "/api/mcp-server-configs/" + id + "/auth-status"},
		{name: "connect", method: http.MethodGet, path: "/api/mcp-server-configs/" + id + "/connect"},
		{name: "callback", method: http.MethodPost, path: "/api/mcp-server-configs/" + id + "/callback", body: `{"code":"code"}`},
		{name: "tools", method: http.MethodGet, path: "/api/mcp-server-configs/" + id + "/tools"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "missing required role")
		})
	}
}

func TestMCPHandler_DoesNotExposePublicRuntimeLifecycle(t *testing.T) {
	r, svc := setupMCP()
	id := uuid.New()
	svc.configs[id] = mcp.McpServerConfigResponse{ID: id, Name: "lifecycle-private", Enabled: true}

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "status", method: http.MethodGet, path: "/api/mcp-server-configs/" + id.String() + "/status"},
		{name: "start", method: http.MethodPost, path: "/api/mcp-server-configs/" + id.String() + "/start"},
		{name: "stop", method: http.MethodPost, path: "/api/mcp-server-configs/" + id.String() + "/stop"},
		{name: "restart", method: http.MethodPost, path: "/api/mcp-server-configs/" + id.String() + "/restart"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

// Bootstrap endpoint tests (ACT-F3-12 / P-C351-1).

// BUG-MCP-RUNTIME-STALE fix: bootstrap now returns all enabled configs (not just auto_start=true).
// Servers with auto_start=false but enabled=true are also registered so agents can reach them.

func TestMCPHandler_Bootstrap_ReturnsAllEnabledConfigs(t *testing.T) {
	r, svc, token := setupMCPWithAuth(t, "mcp-client-runtime")
	// enabled=true, auto_start=true — must appear.
	id1 := uuid.New()
	svc.configs[id1] = mcp.McpServerConfigResponse{ID: id1, Name: "auto-enabled", AutoStart: true, Enabled: true}
	// enabled=true, auto_start=false — must also appear (BUG-MCP-RUNTIME-STALE fix).
	id2 := uuid.New()
	svc.configs[id2] = mcp.McpServerConfigResponse{ID: id2, Name: "manual-enabled", AutoStart: false, Enabled: true}
	// enabled=false — must NOT appear.
	id3 := uuid.New()
	svc.configs[id3] = mcp.McpServerConfigResponse{ID: id3, Name: "disabled", AutoStart: true, Enabled: false}

	req := httptest.NewRequest(http.MethodGet, "/api/mcp-server-configs/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var items []mcp.McpServerConfigBootstrapResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&items))
	require.Len(t, items, 2)
	names := make(map[string]bool)
	for _, it := range items {
		names[it.Name] = true
	}
	assert.True(t, names["auto-enabled"])
	assert.True(t, names["manual-enabled"])
	assert.False(t, names["disabled"])
}

func TestMCPHandler_Bootstrap_EmptyWhenAllDisabled(t *testing.T) {
	r, svc, token := setupMCPWithAuth(t, "mcp-client-runtime")
	id := uuid.New()
	svc.configs[id] = mcp.McpServerConfigResponse{ID: id, Name: "disabled", AutoStart: true, Enabled: false}

	req := httptest.NewRequest(http.MethodGet, "/api/mcp-server-configs/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var items []mcp.McpServerConfigBootstrapResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&items))
	assert.Len(t, items, 0)
}
