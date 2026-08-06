//go:build e2e

package testutil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeycloakClientDeleteRealm_RejectsUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/realms/master/protocol/openid-connect/token":
			_, _ = w.Write([]byte(`{"access_token":"test-token"}`))
		case "/admin/realms/tenant-a":
			require.Equal(t, http.MethodDelete, r.Method)
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewKeycloakClient(server.URL, "admin", "password")
	require.ErrorContains(t, client.DeleteRealm("tenant-a"), "delete realm 403")
}

func TestKeycloakClientDeleteRealm_AllowsMissingRealm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/realms/master/protocol/openid-connect/token":
			_, _ = w.Write([]byte(`{"access_token":"test-token"}`))
		case "/admin/realms/tenant-a":
			w.WriteHeader(http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewKeycloakClient(server.URL, "admin", "password")
	require.NoError(t, client.DeleteRealm("tenant-a"))
}

func TestKeycloakClientDeleteUser_RejectsUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/realms/master/protocol/openid-connect/token":
			_, _ = w.Write([]byte(`{"access_token":"test-token"}`))
		case "/admin/realms/core/users/user-a":
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewKeycloakClient(server.URL, "admin", "password")
	require.ErrorContains(t, client.DeleteUser("core", "user-a"), "delete user 403")
}

func TestKeycloakClientCreateUser_DoesNotAssignAdminRole(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/realms/master/protocol/openid-connect/token":
			_, _ = w.Write([]byte(`{"access_token":"test-token"}`))
		case "/admin/realms/core/users":
			require.Equal(t, http.MethodPost, r.Method)
			w.Header().Set("Location", server.URL+"/admin/realms/core/users/user-a")
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewKeycloakClient(server.URL, "admin", "password")
	userID, err := client.CreateUser("core", "member", "password")
	require.NoError(t, err)
	require.Equal(t, "user-a", userID)
}
