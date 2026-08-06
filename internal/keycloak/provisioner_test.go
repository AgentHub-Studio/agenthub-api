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
	clients := map[string]map[string]any{}
	var audienceMapper map[string]any
	p, _ := newTestProvisioner(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/token"):
			json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"}) //nolint:errcheck
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms":
			calls["realm"]++
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms/test-tenant/clients":
			calls["client"]++
			payload := map[string]any{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			clients[payload["clientId"].(string)] = payload
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/test-tenant/clients":
			_ = json.NewEncoder(w).Encode([]map[string]string{{"id": "workload-internal", "clientId": "agenthub-api"}})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/protocol-mappers/models"):
			_ = json.NewEncoder(w).Encode([]any{})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/protocol-mappers/models"):
			require.NoError(t, json.NewDecoder(r.Body).Decode(&audienceMapper))
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/roles"):
			calls["role"]++
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	credential, err := p.ProvisionRealm(context.Background(), "test-tenant", "Test Tenant")
	require.NoError(t, err)
	assert.Equal(t, 1, calls["realm"])
	assert.Equal(t, 2, calls["client"])
	assert.Equal(t, 4, calls["role"]) // admin, user, mcp-client-runtime, PROXY_SERVICE
	attrs, ok := clients["agenthub-frontend"]["attributes"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "true", attrs["oauth2.device.authorization.grant.enabled"])
	assert.Equal(t, "agenthub-api", credential.ClientID)
	assert.NotEmpty(t, credential.ClientSecret)
	assert.Equal(t, "agenthub-mcp-client-runtime", audienceMapper["config"].(map[string]any)["included.custom.audience"])
	assert.Equal(t, true, clients["agenthub-api"]["serviceAccountsEnabled"])
	assert.Equal(t, false, clients["agenthub-api"]["publicClient"])
}

func TestProvisionRealm_Idempotent(t *testing.T) {
	// 409 Conflict on realm/client/role should not be an error; the existing
	// workload credential and audience mapper are recovered through Admin API.
	p, _ := newTestProvisioner(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/token"):
			json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"}) //nolint:errcheck
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/clients"):
			_ = json.NewEncoder(w).Encode([]map[string]string{{"id": "workload-internal", "clientId": "agenthub-api"}})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/protocol-mappers/models"):
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"name":           workloadAudienceMapperNameForTest,
				"protocolMapper": "oidc-audience-mapper",
				"config": map[string]string{
					"included.custom.audience": "agenthub-mcp-client-runtime",
					"access.token.claim":       "true",
				},
			}})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/client-secret"):
			_ = json.NewEncoder(w).Encode(map[string]string{"value": "existing-workload-secret"})
		default:
			w.WriteHeader(http.StatusConflict)
		}
	})

	credential, err := p.ProvisionRealm(context.Background(), "existing-tenant", "Existing")
	require.NoError(t, err)
	assert.Equal(t, "existing-workload-secret", credential.ClientSecret)
}

func TestProvisionRealm_TokenError(t *testing.T) {
	p, _ := newTestProvisioner(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := p.ProvisionRealm(context.Background(), "any", "Any")
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

	_, err := p.ProvisionRealm(context.Background(), "fail-tenant", "Fail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create realm")
}

const workloadAudienceMapperNameForTest = "agenthub-mcp-runtime-audience"
