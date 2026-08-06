package agentic_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/workloadidentity"
)

func TestKeycloakServiceTokenProvider_UsesTenantRealmAndCachesToken(t *testing.T) {
	var calls atomic.Int32
	keycloak := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/realms/tenant-a/protocol/openid-connect/token" {
			t.Fatalf("unexpected token path: %s", r.URL.Path)
		}
		clientID, clientSecret, ok := r.BasicAuth()
		if !ok || clientID != "agenthub-api" || clientSecret != "secret" {
			t.Fatal("client credentials were not sent with basic authentication")
		}
		if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "client_credentials" {
			t.Fatalf("unexpected token form: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "workload-token", "expires_in": 300})
	}))
	defer keycloak.Close()

	provider, err := agentic.NewKeycloakServiceTokenProvider(keycloak.URL, "agenthub-api", staticCredentialResolver{credentials: map[string]workloadidentity.Credential{
		"tenant-a": {ClientID: "agenthub-api", ClientSecret: "secret"},
	}}, nil)
	if err != nil {
		t.Fatalf("NewKeycloakServiceTokenProvider() error = %v", err)
	}
	first, err := provider.Token(context.Background(), "tenant-a")
	if err != nil || first != "workload-token" {
		t.Fatalf("first Token() = %q, %v", first, err)
	}
	second, err := provider.Token(context.Background(), "tenant-a")
	if err != nil || second != "workload-token" {
		t.Fatalf("second Token() = %q, %v", second, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("Keycloak calls = %d, want 1", calls.Load())
	}
}

func TestAuthenticatedHTTPMCPClient_SendsWorkloadTokenWithoutTenantParameter(t *testing.T) {
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tools" || r.URL.RawQuery != "" {
			t.Fatalf("unexpected execution URL: %s", r.URL.String())
		}
		if got := r.Header.Get("Authorization"); got != "Bearer workload-token" {
			t.Fatalf("Authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tools": []any{}, "warnings": []string{}})
	}))
	defer runtime.Close()

	client := agentic.NewAuthenticatedHTTPMCPClient(runtime.URL, staticTokenProvider{})
	if _, err := client.ListTools(context.Background(), "tenant-a"); err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
}

func TestHTTPMCPClient_FailsClosedWithoutWorkloadProvider(t *testing.T) {
	client := agentic.NewHTTPMCPClient("http://runtime.invalid")
	if _, err := client.ListTools(context.Background(), "tenant-a"); err == nil {
		t.Fatal("ListTools() unexpectedly accepted a client without workload provider")
	}
}

type staticTokenProvider struct{}

func (staticTokenProvider) Token(context.Context, string) (string, error) {
	return "workload-token", nil
}

type staticCredentialResolver struct {
	credentials map[string]workloadidentity.Credential
}

func (s staticCredentialResolver) Resolve(_ context.Context, tenantID string) (workloadidentity.Credential, error) {
	credential, ok := s.credentials[tenantID]
	if !ok {
		return workloadidentity.Credential{}, workloadidentity.ErrNotFound
	}
	return credential, nil
}

func TestKeycloakServiceTokenProvider_RejectsMismatchedTenantClient(t *testing.T) {
	provider, err := agentic.NewKeycloakServiceTokenProvider("https://keycloak.test", "agenthub-api", staticCredentialResolver{credentials: map[string]workloadidentity.Credential{
		"tenant-a": {ClientID: "other-client", ClientSecret: "secret"},
	}}, nil)
	require.NoError(t, err)

	_, err = provider.Token(context.Background(), "tenant-a")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}
