//go:build e2e

package e2e

import "os"

type e2eConfigValues struct {
	backendURL        string
	keycloakURL       string
	keycloakAdmin     string
	keycloakAdminPass string
	e2eUserPassword   string
}

func e2eConfig() e2eConfigValues {
	return e2eConfigValues{
		backendURL:        getEnvOrDefault("API_URL", "http://127.0.0.1:28081"),
		keycloakURL:       getEnvOrDefault("KEYCLOAK_URL", "http://127.0.0.1:28080"),
		keycloakAdmin:     getEnvOrDefault("KEYCLOAK_ADMIN_USER", "admin"),
		keycloakAdminPass: getEnvOrDefault("KEYCLOAK_ADMIN_PASSWORD", "@admin#"),
		e2eUserPassword:   getEnvOrDefault("E2E_USER_PASSWORD", "E2eTestPass#1"),
	}
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
