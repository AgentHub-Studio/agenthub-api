package keycloak_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AgentHub-Studio/agenthub-api/internal/keycloak"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProvisioner(t *testing.T, handler http.HandlerFunc) (*keycloak.Provisioner, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return keycloak.NewProvisioner(keycloak.Config{
		BaseURL:       srv.URL,
		AdminUsername: "admin",
		AdminPassword: "admin",
	}), srv
}

func TestProvisionRealm_Success(t *testing.T) {
	calls := map[string]int{}
	p, _ := newTestProvisioner(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/token"):
			json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"}) //nolint:errcheck
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms":
			calls["realm"]++
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/clients"):
			calls["client"]++
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/roles"):
			calls["role"]++
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	err := p.ProvisionRealm(context.Background(), "test-tenant", "Test Tenant")
	require.NoError(t, err)
	assert.Equal(t, 1, calls["realm"])
	assert.Equal(t, 1, calls["client"])
	assert.Equal(t, 4, calls["role"]) // admin, user, mcp-client-runtime, PROXY_SERVICE
}

func TestProvisionRealm_Idempotent(t *testing.T) {
	// 409 Conflict on realm/client/role should not be an error.
	p, _ := newTestProvisioner(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"}) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusConflict)
	})

	err := p.ProvisionRealm(context.Background(), "existing-tenant", "Existing")
	require.NoError(t, err)
}

func TestProvisionRealm_TokenError(t *testing.T) {
	p, _ := newTestProvisioner(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	err := p.ProvisionRealm(context.Background(), "any", "Any")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestProvisionRealm_RealmCreateError(t *testing.T) {
	p, _ := newTestProvisioner(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"}) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})

	err := p.ProvisionRealm(context.Background(), "fail-tenant", "Fail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create realm")
}
