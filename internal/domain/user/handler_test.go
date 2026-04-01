package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/user"
)

// mockUserSvc implements user.Service for handler tests.
type mockUserSvc struct {
	users map[string]user.UserResponse
}

func newMockUserSvc() *mockUserSvc {
	return &mockUserSvc{users: make(map[string]user.UserResponse)}
}

func (m *mockUserSvc) List(_ context.Context) ([]user.UserResponse, error) {
	items := make([]user.UserResponse, 0, len(m.users))
	for _, u := range m.users {
		items = append(items, u)
	}
	return items, nil
}

func (m *mockUserSvc) Get(_ context.Context, userID string) (user.UserResponse, error) {
	u, ok := m.users[userID]
	if !ok {
		return user.UserResponse{}, user.ErrNotFound
	}
	return u, nil
}

func (m *mockUserSvc) Create(_ context.Context, req user.CreateUserRequest) (user.UserResponse, error) {
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
	if _, ok := m.users[userID]; !ok {
		return user.ErrNotFound
	}
	delete(m.users, userID)
	return nil
}

func (m *mockUserSvc) AssignRole(_ context.Context, userID string, _ string) error {
	if _, ok := m.users[userID]; !ok {
		return user.ErrNotFound
	}
	return nil
}

func (m *mockUserSvc) RemoveRole(_ context.Context, userID string, _ string) error {
	if _, ok := m.users[userID]; !ok {
		return user.ErrNotFound
	}
	return nil
}

func setupUser() (*chi.Mux, *mockUserSvc) {
	svc := newMockUserSvc()
	h := user.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterProtectedRoutes(r)
	return r, svc
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
	req := httptest.NewRequest(http.MethodGet, "/api/users/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUserHandler_Delete_Success(t *testing.T) {
	r, svc := setupUser()
	svc.users["del-user"] = user.UserResponse{ID: "del-user", Username: "todelete", Roles: []string{}}

	req := httptest.NewRequest(http.MethodDelete, "/api/users/del-user", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestUserHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupUser()
	req := httptest.NewRequest(http.MethodDelete, "/api/users/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUserHandler_AssignRole_Success(t *testing.T) {
	r, svc := setupUser()
	svc.users["u1"] = user.UserResponse{ID: "u1", Username: "alice", Roles: []string{}}

	req := httptest.NewRequest(http.MethodPost, "/api/users/u1/roles/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestUserHandler_RemoveRole_NotFound(t *testing.T) {
	r, _ := setupUser()
	req := httptest.NewRequest(http.MethodDelete, "/api/users/ghost/roles/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
