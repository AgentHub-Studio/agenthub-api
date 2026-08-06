//go:build e2e

package e2e

import "testing"

func TestE2EConfigDefaultsUseLocalDisposableStack(t *testing.T) {
	for _, key := range []string{
		"API_URL",
		"KEYCLOAK_URL",
		"KEYCLOAK_ADMIN_USER",
		"KEYCLOAK_ADMIN_PASSWORD",
		"E2E_USER_PASSWORD",
	} {
		t.Setenv(key, "")
	}

	config := e2eConfig()
	if config.backendURL != "http://127.0.0.1:28081" {
		t.Fatalf("unexpected local API default: %q", config.backendURL)
	}
	if config.keycloakURL != "http://127.0.0.1:28080" {
		t.Fatalf("unexpected local Keycloak default: %q", config.keycloakURL)
	}
	if config.keycloakAdmin != "admin" || config.keycloakAdminPass != "@admin#" {
		t.Fatalf("unexpected disposable Keycloak admin defaults")
	}
}
