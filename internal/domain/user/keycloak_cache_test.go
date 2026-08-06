package user

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeycloakClient_RolesCacheReusesEntriesAndReturnsDefensiveCopies(t *testing.T) {
	t.Parallel()

	var roleRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/realms/master/protocol/openid-connect/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":300}`))
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/tenant-a/users/user-1/role-mappings/realm":
			roleRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"name":"viewer"}]`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	client := NewKeycloakUserClient(KeycloakClientConfig{
		BaseURL:       server.URL,
		AdminUsername: "admin",
		AdminPassword: "password",
	}).(*keycloakClient)

	first, err := client.getUserRoles(context.Background(), "tenant-a", "user-1")
	require.NoError(t, err)
	require.Equal(t, []string{"viewer"}, first)

	second, err := client.getUserRoles(context.Background(), "tenant-a", "user-1")
	require.NoError(t, err)
	require.Equal(t, []string{"viewer"}, second)
	second[0] = "mutated-by-caller"
	third, err := client.getUserRoles(context.Background(), "tenant-a", "user-1")
	require.NoError(t, err)
	require.Equal(t, []string{"viewer"}, third)
	require.Equal(t, int32(1), roleRequests.Load(), "the warm cache must avoid another Keycloak role request")
}

func TestKeycloakClient_RolesCacheInvalidatesAfterRoleAssignment(t *testing.T) {
	t.Parallel()

	var roleRequests atomic.Int32
	var assigned atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/realms/master/protocol/openid-connect/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":300}`))
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/tenant-a/users/user-1/role-mappings/realm":
			roleRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if assigned.Load() {
				_, _ = w.Write([]byte(`[{"name":"viewer"},{"name":"admin"}]`))
				return
			}
			_, _ = w.Write([]byte(`[{"name":"viewer"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/tenant-a/roles/admin":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"role-admin","name":"admin"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms/tenant-a/users/user-1/role-mappings/realm":
			assigned.Store(true)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	client := NewKeycloakUserClient(KeycloakClientConfig{
		BaseURL:       server.URL,
		AdminUsername: "admin",
		AdminPassword: "password",
	}).(*keycloakClient)

	_, err := client.getUserRoles(context.Background(), "tenant-a", "user-1")
	require.NoError(t, err)
	_, err = client.getUserRoles(context.Background(), "tenant-a", "user-1")
	require.NoError(t, err)
	require.Equal(t, int32(1), roleRequests.Load())

	require.NoError(t, client.AssignRole(context.Background(), "tenant-a", "user-1", "admin"))
	roles, err := client.getUserRoles(context.Background(), "tenant-a", "user-1")
	require.NoError(t, err)
	require.Equal(t, []string{"viewer", "admin"}, roles)
	require.Equal(t, int32(2), roleRequests.Load(), "a successful assignment must invalidate the cached roles")
}

func TestKeycloakClient_ListUsersPropagatesRoleLookupFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/realms/master/protocol/openid-connect/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":300}`))
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/tenant-a/users":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"user-1","username":"alice","enabled":true}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/tenant-a/users/user-1/role-mappings/realm":
			http.Error(w, "keycloak unavailable", http.StatusServiceUnavailable)
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	client := NewKeycloakUserClient(KeycloakClientConfig{
		BaseURL:       server.URL,
		AdminUsername: "admin",
		AdminPassword: "password",
	}).(*keycloakClient)

	_, err := client.ListUsers(context.Background(), "tenant-a")
	require.ErrorContains(t, err, "get user roles 503")
}

func TestKeycloakClient_GetUserPropagatesRoleLookupFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/realms/master/protocol/openid-connect/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":300}`))
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/tenant-a/users/user-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"user-1","username":"alice","enabled":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/tenant-a/users/user-1/role-mappings/realm":
			http.Error(w, "keycloak unavailable", http.StatusServiceUnavailable)
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	client := NewKeycloakUserClient(KeycloakClientConfig{
		BaseURL:       server.URL,
		AdminUsername: "admin",
		AdminPassword: "password",
	}).(*keycloakClient)

	_, err := client.GetUser(context.Background(), "tenant-a", "user-1")
	require.ErrorContains(t, err, "get user roles 503")
}
