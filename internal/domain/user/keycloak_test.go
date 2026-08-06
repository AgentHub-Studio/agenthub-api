package user

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestKeycloakServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/realms/master/protocol/openid-connect/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "admin-token",
				"expires_in":   60,
			})
			return
		}
		if r.Header.Get("Authorization") != "Bearer admin-token" {
			http.Error(w, "missing admin token", http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}))
}

func newTestKeycloakClient(baseURL string) KeycloakUserClient {
	return NewKeycloakUserClient(KeycloakClientConfig{
		BaseURL:       baseURL,
		AdminUsername: "admin",
		AdminPassword: "admin",
	})
}

func TestBuildKeycloakURL_EscapesSegmentsAndPreservesBasePath(t *testing.T) {
	got, err := buildKeycloakURL("https://keycloak.example.com/auth/", "admin", "realms", "tenant space", "users", "user/one")
	require.NoError(t, err)

	assert.Equal(t, "https://keycloak.example.com/auth/admin/realms/tenant%20space/users/user%2Fone", got)
}

func TestBuildKeycloakURL_RejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		segments []string
	}{
		{name: "userinfo", baseURL: "https://user:pass@keycloak.example.com", segments: []string{"realms", "master"}},
		{name: "unsupported scheme", baseURL: "ftp://keycloak.example.com", segments: []string{"realms", "master"}},
		{name: "query", baseURL: "https://keycloak.example.com?tenant=test", segments: []string{"realms", "master"}},
		{name: "empty segment", baseURL: "https://keycloak.example.com", segments: []string{"realms", ""}},
		{name: "control char", baseURL: "https://keycloak.example.com", segments: []string{"realms", "bad\nrealm"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildKeycloakURL(tc.baseURL, tc.segments...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "keycloak:")
		})
	}
}

func TestKeycloakClient_ListRoles_UsesRealmRoles(t *testing.T) {
	var requestedPath string
	server := newTestKeycloakServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/admin/realms/test/roles", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"name":"offline_access"},
			{"name":"default-roles-test"},
			{"name":"user"},
			{"name":"admin"},
			{"name":"uma_authorization"}
		]`))
	})
	defer server.Close()

	client := newTestKeycloakClient(server.URL)
	roles, err := client.ListRoles(context.Background(), "test")
	require.NoError(t, err)

	assert.Equal(t, "/admin/realms/test/roles", requestedPath)
	assert.Equal(t, []string{"admin", "user"}, roles)
}

func TestKeycloakClient_AssignAndRemoveRole_UsesRealmRoleMappings(t *testing.T) {
	var mu sync.Mutex
	var mappingMethods []string
	server := newTestKeycloakServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/test/roles/user":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"role-user","name":"user"}`))
		case (r.Method == http.MethodPost || r.Method == http.MethodDelete) &&
			r.URL.Path == "/admin/realms/test/users/u1/role-mappings/realm":
			mu.Lock()
			mappingMethods = append(mappingMethods, r.Method)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	})
	defer server.Close()

	client := newTestKeycloakClient(server.URL)
	require.NoError(t, client.AssignRole(context.Background(), "test", "u1", "user"))
	require.NoError(t, client.RemoveRole(context.Background(), "test", "u1", "user"))

	assert.Equal(t, []string{http.MethodPost, http.MethodDelete}, mappingMethods)
}

func TestKeycloakClient_GetUser_ReturnsRealmRoleMappings(t *testing.T) {
	server := newTestKeycloakServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/test/users/u1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id":"u1",
				"username":"alice",
				"email":"alice@example.com",
				"enabled":true
			}`))
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/test/users/u1/role-mappings/realm":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"name":"user"},{"name":"admin"}]`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	})
	defer server.Close()

	client := newTestKeycloakClient(server.URL)
	got, err := client.GetUser(context.Background(), "test", "u1")
	require.NoError(t, err)

	assert.Equal(t, "u1", got.ID)
	assert.Equal(t, []string{"user", "admin"}, got.Roles)
}

func TestKeycloakClient_CreateUser_SetsLoginReadyFields(t *testing.T) {
	var createPayload map[string]any
	server := newTestKeycloakServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms/test/users":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&createPayload))
			w.Header().Set("Location", "http://keycloak/admin/realms/test/users/u1")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/test/users/u1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id":"u1",
				"username":"alice",
				"email":"alice@example.com",
				"enabled":true
			}`))
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/test/users/u1/role-mappings/realm":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	})
	defer server.Close()

	client := newTestKeycloakClient(server.URL)
	_, err := client.CreateUser(context.Background(), "test", CreateUserRequest{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "Secret#123",
	})
	require.NoError(t, err)

	assert.Equal(t, true, createPayload["enabled"])
	assert.Equal(t, true, createPayload["emailVerified"])
	assert.Equal(t, "alice", createPayload["firstName"])
	assert.Equal(t, "alice", createPayload["lastName"])
	assert.Empty(t, createPayload["requiredActions"])
	credentials, ok := createPayload["credentials"].([]any)
	require.True(t, ok)
	require.Len(t, credentials, 1)
	credential, ok := credentials[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "password", credential["type"])
	assert.Equal(t, false, credential["temporary"])
}
