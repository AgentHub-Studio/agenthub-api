package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/user"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
)

// mockUserSvc implements user.Service for handler tests.
type mockUserSvc struct {
	users            map[string]user.UserResponse
	upstreamErr      error
	resetPasswordErr error
}

func newMockUserSvc() *mockUserSvc {
	return &mockUserSvc{users: make(map[string]user.UserResponse)}
}

func (m *mockUserSvc) List(_ context.Context) ([]user.UserResponse, error) {
	if m.upstreamErr != nil {
		return nil, m.upstreamErr
	}
	items := make([]user.UserResponse, 0, len(m.users))
	for _, u := range m.users {
		items = append(items, u)
	}
	return items, nil
}

func (m *mockUserSvc) Get(_ context.Context, userID string) (user.UserResponse, error) {
	if m.upstreamErr != nil {
		return user.UserResponse{}, m.upstreamErr
	}
	u, ok := m.users[userID]
	if !ok {
		return user.UserResponse{}, user.ErrNotFound
	}
	return u, nil
}

func (m *mockUserSvc) Create(_ context.Context, req user.CreateUserRequest) (user.UserResponse, error) {
	if m.upstreamErr != nil {
		return user.UserResponse{}, m.upstreamErr
	}
	for _, u := range m.users {
		if u.Username == req.Username {
			return user.UserResponse{}, user.ErrAlreadyExists
		}
	}
	id := "user-" + req.Username
	resp := user.UserResponse{
		ID:       id,
		Username: req.Username,
		Email:    req.Email,
		Roles:    []string{},
	}
	m.users[id] = resp
	return resp, nil
}

func (m *mockUserSvc) Update(_ context.Context, userID string, req user.UpdateUserRequest) (user.UserResponse, error) {
	if m.upstreamErr != nil {
		return user.UserResponse{}, m.upstreamErr
	}
	u, ok := m.users[userID]
	if !ok {
		return user.UserResponse{}, user.ErrNotFound
	}
	if req.Email != nil {
		u.Email = *req.Email
	}
	m.users[userID] = u
	return u, nil
}

func (m *mockUserSvc) Delete(_ context.Context, userID string) error {
	if m.upstreamErr != nil {
		return m.upstreamErr
	}
	if _, ok := m.users[userID]; !ok {
		return user.ErrNotFound
	}
	delete(m.users, userID)
	return nil
}

func (m *mockUserSvc) AssignRole(_ context.Context, userID string, _ string) error {
	if m.upstreamErr != nil {
		return m.upstreamErr
	}
	if _, ok := m.users[userID]; !ok {
		return user.ErrNotFound
	}
	return nil
}

func (m *mockUserSvc) RemoveRole(_ context.Context, userID string, _ string) error {
	if m.upstreamErr != nil {
		return m.upstreamErr
	}
	if _, ok := m.users[userID]; !ok {
		return user.ErrNotFound
	}
	return nil
}

func (m *mockUserSvc) ResetPassword(_ context.Context, userID string) error {
	if m.upstreamErr != nil {
		return m.upstreamErr
	}
	if m.resetPasswordErr != nil {
		return m.resetPasswordErr
	}
	if _, ok := m.users[userID]; !ok {
		return user.ErrNotFound
	}
	return nil
}

func (m *mockUserSvc) ListRoles(_ context.Context) ([]string, error) {
	if m.upstreamErr != nil {
		return nil, m.upstreamErr
	}
	return []string{"admin", "user"}, nil
}

func setupUser() (*chi.Mux, *mockUserSvc) {
	return setupUserWithRoles("admin")
}

func setupUserWithRoles(roles ...string) (*chi.Mux, *mockUserSvc) {
	svc := newMockUserSvc()
	h := user.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterProtectedRoutes(r)
	return r, svc
}

func TestUserHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupUserWithRoles("user")
	id := uuid.NewString()
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/users"},
		{name: "create", method: http.MethodPost, path: "/api/users", body: `{}`},
		{name: "list roles", method: http.MethodGet, path: "/api/users/roles"},
		{name: "get", method: http.MethodGet, path: "/api/users/" + id},
		{name: "update", method: http.MethodPatch, path: "/api/users/" + id, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/users/" + id},
		{name: "assign role", method: http.MethodPost, path: "/api/users/" + id + "/roles/admin"},
		{name: "remove role", method: http.MethodDelete, path: "/api/users/" + id + "/roles/admin"},
		{name: "reset password", method: http.MethodPost, path: "/api/users/" + id + "/reset-password", body: `{}`},
	} {
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

func TestUserHandler_List_Success(t *testing.T) {
	r, svc := setupUser()
	svc.users["u1"] = user.UserResponse{ID: "u1", Username: "alice", Roles: []string{}}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var items []user.UserResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	assert.Len(t, items, 1)
}

func TestUserHandler_List_Empty(t *testing.T) {
	r, _ := setupUser()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserHandler_Create_Success(t *testing.T) {
	r, _ := setupUser()
	body, _ := json.Marshal(user.CreateUserRequest{Username: "bob", Email: "bob@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp user.UserResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "bob", resp.Username)
}

func TestUserHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupUser()
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, svc := setupUser()
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/users",
			bytes.NewBufferString(`{"username":"first","email":"first@example.com"}{"username":"ignored"}`),
		)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, svc.users)
	})

	t.Run("update", func(t *testing.T) {
		r, svc := setupUser()
		id := uuid.NewString()
		svc.users[id] = user.UserResponse{ID: id, Username: "alice", Email: "alice@example.com", Roles: []string{}}
		req := httptest.NewRequest(
			http.MethodPatch,
			"/api/users/"+id,
			bytes.NewBufferString(`{"email":"changed@example.com"}{"email":"ignored@example.com"}`),
		)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, "alice@example.com", svc.users[id].Email)
	})
}

func TestUserHandler_Create_AlreadyExists(t *testing.T) {
	r, svc := setupUser()
	svc.users["user-alice"] = user.UserResponse{ID: "user-alice", Username: "alice", Roles: []string{}}

	body, _ := json.Marshal(user.CreateUserRequest{Username: "alice", Email: "alice@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUserHandler_Get_NotFound(t *testing.T) {
	r, _ := setupUser()
	req := httptest.NewRequest(http.MethodGet, "/api/users/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUserHandler_Delete_Success(t *testing.T) {
	r, svc := setupUser()
	id := uuid.New().String()
	svc.users[id] = user.UserResponse{ID: id, Username: "todelete", Roles: []string{}}

	req := httptest.NewRequest(http.MethodDelete, "/api/users/"+id, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestUserHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupUser()
	req := httptest.NewRequest(http.MethodDelete, "/api/users/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUserHandler_ResetPassword_Success(t *testing.T) {
	r, svc := setupUser()
	id := uuid.New().String()
	svc.users[id] = user.UserResponse{ID: id, Username: "resettable", Roles: []string{}}

	req := httptest.NewRequest(http.MethodPost, "/api/users/"+id+"/reset-password", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestUserHandler_ResetPassword_KeycloakFailureReturnsSanitizedBadGateway(t *testing.T) {
	r, svc := setupUser()
	id := uuid.New().String()
	svc.users[id] = user.UserResponse{ID: id, Username: "resettable", Roles: []string{}}
	svc.resetPasswordErr = errors.New("keycloak: reset password 500: internal mail detail")

	req := httptest.NewRequest(http.MethodPost, "/api/users/"+id+"/reset-password", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.JSONEq(t, `{"error":"user provisioning service unavailable"}`, w.Body.String())
	assert.NotContains(t, w.Body.String(), "internal mail detail")
}

func TestUserHandler_KeycloakUpstreamFailuresReturnSanitizedBadGateway(t *testing.T) {
	id := uuid.New().String()
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/users"},
		{name: "create", method: http.MethodPost, path: "/api/users", body: `{"username":"alice"}`},
		{name: "list roles", method: http.MethodGet, path: "/api/users/roles"},
		{name: "get", method: http.MethodGet, path: "/api/users/" + id},
		{name: "update", method: http.MethodPatch, path: "/api/users/" + id, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/users/" + id},
		{name: "assign role", method: http.MethodPost, path: "/api/users/" + id + "/roles/admin"},
		{name: "remove role", method: http.MethodDelete, path: "/api/users/" + id + "/roles/admin"},
		{name: "reset password", method: http.MethodPost, path: "/api/users/" + id + "/reset-password"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, svc := setupUser()
			svc.users[id] = user.UserResponse{ID: id, Username: "upstream", Roles: []string{}}
			svc.upstreamErr = errors.New("keycloak: synthetic internal detail")

			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadGateway, w.Code)
			assert.JSONEq(t, `{"error":"user provisioning service unavailable"}`, w.Body.String())
			assert.NotContains(t, w.Body.String(), "synthetic internal detail")
		})
	}
}

func TestUserHandler_AssignRole_Success(t *testing.T) {
	r, svc := setupUser()
	id := uuid.New().String()
	svc.users[id] = user.UserResponse{ID: id, Username: "alice", Roles: []string{}}

	req := httptest.NewRequest(http.MethodPost, "/api/users/"+id+"/roles/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestUserHandler_RemoveRole_NotFound(t *testing.T) {
	r, _ := setupUser()
	req := httptest.NewRequest(http.MethodDelete, "/api/users/"+uuid.New().String()+"/roles/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
