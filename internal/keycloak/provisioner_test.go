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

func newTestProvisioner(t *testing.T, handler http.HandlerFunc) *keycloak.Provisioner {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return keycloak.NewProvisioner(keycloak.Config{BaseURL: srv.URL, AdminUsername: "admin", AdminPassword: "admin"})
}

func TestProvisionRealm_CreatesTenantWorkloadServiceAccount(t *testing.T) {
	var workload map[string]any
	p := newTestProvisioner(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/token"):
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "admin-token"})
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms/tenant-a/clients":
			body := map[string]any{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			if body["clientId"] == "agenthub-api" {
				workload = body
			}
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/tenant-a/clients":
			_ = json.NewEncoder(w).Encode([]map[string]string{{"id": "internal-client", "clientId": "agenthub-api"}})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/protocol-mappers/models"):
			_ = json.NewEncoder(w).Encode([]any{})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/protocol-mappers/models"):
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/roles"):
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	credential, err := p.ProvisionRealm(context.Background(), "tenant-a", "Tenant A")

	require.NoError(t, err)
	assert.Equal(t, "agenthub-api", credential.ClientID)
	assert.NotEmpty(t, credential.ClientSecret)
	assert.Equal(t, false, workload["publicClient"])
	assert.Equal(t, true, workload["serviceAccountsEnabled"])
	assert.Equal(t, false, workload["standardFlowEnabled"])
}
