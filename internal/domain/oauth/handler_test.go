package oauth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockOAuthSvc satisfies the private service interface in oauth.Handler.
type mockOAuthSvc struct {
	credentials map[uuid.UUID]oauth.OAuthCredential
	resolve     map[uuid.UUID]oauth.ResolveResponse
}

func newMockOAuthSvc() *mockOAuthSvc {
	return &mockOAuthSvc{
		credentials: make(map[uuid.UUID]oauth.OAuthCredential),
		resolve:     make(map[uuid.UUID]oauth.ResolveResponse),
	}
}

func (m *mockOAuthSvc) ListAll(_ context.Context, _ string, req pagination.PageRequest) ([]oauth.OAuthCredential, int, error) {
	items := make([]oauth.OAuthCredential, 0, len(m.credentials))
	for _, c := range m.credentials {
		items = append(items, c)
	}
	return items, len(items), nil
}

func (m *mockOAuthSvc) Create(_ context.Context, _ string, req oauth.CreateRequest) (oauth.OAuthCredential, error) {
	id := uuid.New()
	c := oauth.OAuthCredential{
		ID:       id,
		Name:     req.Name,
		AuthType: req.AuthType,
	}
	m.credentials[id] = c
	return c, nil
}

func (m *mockOAuthSvc) GetByID(_ context.Context, _ string, id uuid.UUID) (oauth.OAuthCredential, error) {
	c, ok := m.credentials[id]
	if !ok {
		return oauth.OAuthCredential{}, oauth.ErrNotFound
	}
	return c, nil
}

func (m *mockOAuthSvc) Update(_ context.Context, _ string, id uuid.UUID, req oauth.CreateRequest) (oauth.OAuthCredential, error) {
	c, ok := m.credentials[id]
	if !ok {
		return oauth.OAuthCredential{}, oauth.ErrNotFound
	}
	if req.Name != "" {
		c.Name = req.Name
	}
	m.credentials[id] = c
	return c, nil
}

func (m *mockOAuthSvc) Delete(_ context.Context, _ string, id uuid.UUID) error {
	if _, ok := m.credentials[id]; !ok {
		return oauth.ErrNotFound
	}
	delete(m.credentials, id)
	return nil
}

func (m *mockOAuthSvc) ResolveAuthHeader(_ context.Context, _ string, id uuid.UUID) (oauth.ResolveResponse, error) {
	if _, ok := m.credentials[id]; !ok {
		return oauth.ResolveResponse{}, oauth.ErrNotFound
	}
	if res, ok := m.resolve[id]; ok {
		return res, nil
	}
	return oauth.ResolveResponse{Header: "Authorization", Value: "Bearer token123"}, nil
}

func (m *mockOAuthSvc) ExchangeCode(_ context.Context, _ string, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockOAuthSvc) RefreshToken(_ context.Context, _ string, _ uuid.UUID) (oauth.OAuthCredential, error) {
	return oauth.OAuthCredential{}, nil
}

func setupOAuth() (*chi.Mux, *mockOAuthSvc) {
	return setupOAuthWithRoles("admin")
}

func setupOAuthWithRoles(roles ...string) (*chi.Mux, *mockOAuthSvc) {
	svc := newMockOAuthSvc()
	h := oauth.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.NewContext(req.Context(), "test-tenant")
			ctx = middleware.ContextWithRoles(ctx, roles...)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Mount("/api/oauth", h.Routes())
	return r, svc
}

func TestOAuthHandler_List_Success(t *testing.T) {
	r, svc := setupOAuth()
	id := uuid.New()
	svc.credentials[id] = oauth.OAuthCredential{ID: id, Name: "my-cred", AuthType: oauth.AuthTypeAPIKey}

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[oauth.OAuthCredentialResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestOAuthHandler_List_Empty(t *testing.T) {
	r, _ := setupOAuth()

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[oauth.OAuthCredentialResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(0), page.TotalElements)
}

func TestOAuthHandler_Create_Success(t *testing.T) {
	r, _ := setupOAuth()
	body, _ := json.Marshal(oauth.CreateRequest{
		Name:     "API Cred",
		AuthType: oauth.AuthTypeAPIKey,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp oauth.OAuthCredentialResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "API Cred", resp.Name)
}

func TestOAuthHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupOAuth()
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOAuthHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, svc := setupOAuth()
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/oauth/",
			bytes.NewBufferString(`{"name":"first","authType":"API_KEY"}{"name":"ignored"}`),
		)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.credentials)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupOAuth()
		id := uuid.New()
		svc.credentials[id] = oauth.OAuthCredential{ID: id, Name: "original", AuthType: oauth.AuthTypeAPIKey}
		req := httptest.NewRequest(
			http.MethodPut,
			"/api/oauth/"+id.String(),
			bytes.NewBufferString(`{"name":"changed","authType":"API_KEY"}{"name":"ignored"}`),
		)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, "original", svc.credentials[id].Name)
	})
}

func TestOAuthHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupOAuth()
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOAuthHandler_GetByID_InvalidID(t *testing.T) {
	r, _ := setupOAuth()
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOAuthHandler_Update_Success(t *testing.T) {
	r, svc := setupOAuth()
	id := uuid.New()
	svc.credentials[id] = oauth.OAuthCredential{ID: id, Name: "old-name", AuthType: oauth.AuthTypeAPIKey}

	body, _ := json.Marshal(oauth.CreateRequest{Name: "new-name", AuthType: oauth.AuthTypeAPIKey})
	req := httptest.NewRequest(http.MethodPut, "/api/oauth/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp oauth.OAuthCredentialResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "new-name", resp.Name)
}

func TestOAuthHandler_Update_NotFound(t *testing.T) {
	r, _ := setupOAuth()
	body, _ := json.Marshal(oauth.CreateRequest{Name: "name", AuthType: oauth.AuthTypeAPIKey})
	req := httptest.NewRequest(http.MethodPut, "/api/oauth/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOAuthHandler_Delete_Success(t *testing.T) {
	r, svc := setupOAuth()
	id := uuid.New()
	svc.credentials[id] = oauth.OAuthCredential{ID: id, Name: "to-delete"}

	req := httptest.NewRequest(http.MethodDelete, "/api/oauth/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestOAuthHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupOAuth()
	req := httptest.NewRequest(http.MethodDelete, "/api/oauth/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOAuthHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupOAuthWithRoles("user")
	id := uuid.NewString()
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/oauth/"},
		{name: "create", method: http.MethodPost, path: "/api/oauth/", body: `{"name":"credential","authType":"API_KEY"}`},
		{name: "get", method: http.MethodGet, path: "/api/oauth/" + id},
		{name: "put", method: http.MethodPut, path: "/api/oauth/" + id, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/oauth/" + id, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/oauth/" + id},
		{name: "resolve", method: http.MethodGet, path: "/api/oauth/" + id + "/resolve"},
		{name: "callback", method: http.MethodGet, path: oauthCallbackPath(uuid.New(), "")},
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

func TestOAuthHandler_CallbackAllowsAdminRole(t *testing.T) {
	r, _ := setupOAuth()
	req := httptest.NewRequest(http.MethodGet, oauthCallbackPath(uuid.New(), "/internal/(right:oauth-credentials)"), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Equal(t, "/internal/(right:oauth-credentials)", w.Header().Get("Location"))
}

func TestOAuthHandler_Resolve_Success(t *testing.T) {
	r, svc := setupOAuth()
	id := uuid.New()
	svc.credentials[id] = oauth.OAuthCredential{ID: id, Name: "bearer-cred", AuthType: oauth.AuthTypeBearerToken}

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/"+id.String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp oauth.ResolveResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Authorization", resp.Header)
}

func TestOAuthHandler_Resolve_DoesNotExposeSecretValue(t *testing.T) {
	r, svc := setupOAuth()
	id := uuid.New()
	svc.credentials[id] = oauth.OAuthCredential{ID: id, Name: "bearer-cred", AuthType: oauth.AuthTypeBearerToken}
	svc.resolve[id] = oauth.ResolveResponse{Header: "Authorization", Value: "Bearer oauth-resolve-secret"}

	req := httptest.NewRequest(http.MethodGet, "/api/oauth/"+id.String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, "oauth-resolve-secret")
	var resp oauth.ResolveResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Authorization", resp.Header)
	assert.Equal(t, "***", resp.Value)
}

func TestOAuthHandler_Callback_RedirectsAllowedRelativeTarget(t *testing.T) {
	r, _ := setupOAuth()
	id := uuid.New()
	target := "/internal/agents?status=DRAFT"

	req := httptest.NewRequest(http.MethodGet, oauthCallbackPath(id, target), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Equal(t, target, w.Header().Get("Location"))
}

func TestOAuthHandler_Callback_RejectsUnsafeRedirectTargets(t *testing.T) {
	for _, target := range []string{
		"https://evil.example/phish",
		"/internal\nLocation: https://evil.example",
		"https://evil.example@app.cezar.dev/internal",
		"//evil.example/phish",
	} {
		t.Run(target, func(t *testing.T) {
			r, _ := setupOAuth()
			id := uuid.New()

			req := httptest.NewRequest(http.MethodGet, oauthCallbackPath(id, target), nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Empty(t, w.Header().Get("Location"))
			assert.Contains(t, w.Body.String(), "Authentication Successful")
		})
	}
}

func TestOAuthHandler_Resolve_NotFound(t *testing.T) {
	r, _ := setupOAuth()
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/"+uuid.New().String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func oauthCallbackPath(id uuid.UUID, redirectTarget string) string {
	values := url.Values{}
	values.Set("tenantId", "test-tenant")
	values.Set("state", id.String())
	values.Set("code", "oauth-code")
	values.Set("redirect_ui", redirectTarget)
	return "/api/oauth/callback?" + values.Encode()
}
