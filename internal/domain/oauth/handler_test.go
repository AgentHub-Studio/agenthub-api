package oauth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// mockOAuthSvc satisfies the private service interface in oauth.Handler.
type mockOAuthSvc struct {
	credentials map[uuid.UUID]oauth.OAuthCredential
}

func newMockOAuthSvc() *mockOAuthSvc {
	return &mockOAuthSvc{credentials: make(map[uuid.UUID]oauth.OAuthCredential)}
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
	return oauth.ResolveResponse{Header: "Authorization", Value: "Bearer token123"}, nil
}

func (m *mockOAuthSvc) ExchangeCode(_ context.Context, _ string, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockOAuthSvc) RefreshToken(_ context.Context, _ string, _ uuid.UUID) (oauth.OAuthCredential, error) {
	return oauth.OAuthCredential{}, nil
}

func setupOAuth() (*chi.Mux, *mockOAuthSvc) {
	svc := newMockOAuthSvc()
	h := oauth.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.NewContext(req.Context(), "test-tenant")
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

func TestOAuthHandler_Resolve_NotFound(t *testing.T) {
	r, _ := setupOAuth()
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/"+uuid.New().String()+"/resolve", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
