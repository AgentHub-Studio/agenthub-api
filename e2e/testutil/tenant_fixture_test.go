//go:build e2e

package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTenantFixtureID_IsUniqueWithinSameNanosecond(t *testing.T) {
	fixedNow := time.Unix(0, 1783996000000000000)
	first := nextTenantFixtureID(fixedNow)
	second := nextTenantFixtureID(fixedNow)

	require.NotEqual(t, first, second)
	require.LessOrEqual(t, len("e2e-"+second), 63)
	require.True(t, strings.HasPrefix(first, "1783996000000000000-"))
}

func TestTenantFixtureCreateBody_UsesPublicTenantIDContract(t *testing.T) {
	var body map[string]string
	require.NoError(t, json.Unmarshal([]byte(tenantFixtureCreateBody("E2E Tenant", "e2e-unique")), &body))
	require.Equal(t, "E2E Tenant", body["name"])
	require.Equal(t, "e2e-unique", body["id"])
	require.NotContains(t, body, "slug")
}

func TestTenantFixtureDeleteTenantViaCoreAPI_UsesAdministrativeRoute(t *testing.T) {
	cleanupUserDeleted := false
	tenantDeleted := false
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/realms/master/protocol/openid-connect/token", "/realms/core/protocol/openid-connect/token":
			_, _ = w.Write([]byte(`{"access_token":"test-token"}`))
		case "/admin/realms/core/users":
			require.Equal(t, http.MethodPost, r.Method)
			w.Header().Set("Location", server.URL+"/admin/realms/core/users/cleanup-user")
			w.WriteHeader(http.StatusCreated)
		case "/admin/realms/core/roles/admin":
			_, _ = w.Write([]byte(`{"id":"role-id","name":"admin"}`))
		case "/admin/realms/core/users/cleanup-user/role-mappings/realm":
			require.Equal(t, http.MethodPost, r.Method)
			w.WriteHeader(http.StatusNoContent)
		case "/api/admin/tenants/tenant-a":
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			tenantDeleted = true
			w.WriteHeader(http.StatusNoContent)
		case "/admin/realms/core/users/cleanup-user":
			require.Equal(t, http.MethodDelete, r.Method)
			cleanupUserDeleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			body, _ := io.ReadAll(r.Body)
			t.Fatalf("unexpected request: %s %s body=%s", r.Method, r.URL.Path, body)
		}
	}))
	defer server.Close()

	fixture := &TenantFixture{
		Slug:         "tenant-a",
		backendURL:   server.URL,
		userPassword: "E2eTestPass#1",
		keycloak:     NewKeycloakClient(server.URL, "admin", "password"),
	}
	require.NoError(t, fixture.deleteTenantViaCoreAPI())
	require.True(t, tenantDeleted)
	require.True(t, cleanupUserDeleted)
}
