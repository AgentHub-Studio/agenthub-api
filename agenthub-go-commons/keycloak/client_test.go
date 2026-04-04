package keycloak_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/keycloak"
)

// newTestServer creates a minimal Keycloak stub that handles:
//   - POST /realms/master/protocol/openid-connect/token → returns access_token
//   - any other path → responds with the given handler
func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, keycloak.Config) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Token endpoint
		if r.Method == http.MethodPost && r.URL.Path == "/realms/master/protocol/openid-connect/token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"access_token": "test-token"})
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	cfg := keycloak.Config{
		BaseURL:       srv.URL,
		AdminUsername: "admin",
		AdminPassword: "admin",
		AdminRealm:    "master",
		AdminClientID: "admin-cli",
		ClientID:      "agenthub-frontend",
	}
	return srv, cfg
}

func TestNewAdminClient(t *testing.T) {
	cfg := keycloak.Config{
		BaseURL:       "http://localhost:8080",
		AdminUsername: "admin",
		AdminPassword: "admin",
	}
	client := keycloak.NewAdminClient(cfg)
	assert.NotNil(t, client)
}

func TestProvisionTenant_Success(t *testing.T) {
	_, cfg := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// All admin API calls return 201 Created
		w.WriteHeader(http.StatusCreated)
	})

	client := keycloak.NewAdminClient(cfg)
	err := client.ProvisionTenant(context.Background(), "my-company")
	require.NoError(t, err)
}

func TestProvisionTenant_TokenError(t *testing.T) {
	// Server that returns 401 for the token endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	cfg := keycloak.Config{
		BaseURL:       srv.URL,
		AdminUsername: "admin",
		AdminPassword: "wrong",
		AdminRealm:    "master",
		AdminClientID: "admin-cli",
	}
	client := keycloak.NewAdminClient(cfg)
	err := client.ProvisionTenant(context.Background(), "my-company")
	require.Error(t, err)
}

func TestCreateRealm_AlreadyExists(t *testing.T) {
	_, cfg := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/admin/realms" {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})

	client := keycloak.NewAdminClient(cfg)
	// ProvisionTenant internally calls CreateRealm; 409 is treated as success.
	err := client.ProvisionTenant(context.Background(), "existing-tenant")
	require.NoError(t, err)
}

func TestCreateUser_Success(t *testing.T) {
	_, cfg := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Location uses a dummy base; splitLast extracts the last path segment.
			w.Header().Set("Location", "http://keycloak/admin/realms/test/users/user-uuid-123")
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	client := keycloak.NewAdminClient(cfg)
	id, err := client.CreateUser(context.Background(), "test", keycloak.User{
		Username: "alice",
		Email:    "alice@example.com",
		Enabled:  true,
	})
	require.NoError(t, err)
	assert.Equal(t, "user-uuid-123", id)
}

func TestGetUser_NotFound(t *testing.T) {
	_, cfg := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	client := keycloak.NewAdminClient(cfg)
	_, err := client.GetUser(context.Background(), "test", "ghost-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
