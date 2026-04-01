package config

import (
	"fmt"
	"os"
	"strings"
)

// KeycloakAdminConfig holds Keycloak Admin API credentials.
type KeycloakAdminConfig struct {
	AdminUsername  string
	AdminPassword  string
	AdminClientID  string
	AdminRealm     string
	FrontendClient string
}

// Config holds all configuration for agenthub-api.
type Config struct {
	Port            string
	DatabaseURL     string
	KeycloakBaseURL string
	KeycloakAdmin   KeycloakAdminConfig
	CORSOrigins     []string
	LogLevel        string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Port:            getEnv("PORT", "8081"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		KeycloakBaseURL: os.Getenv("KEYCLOAK_BASE_URL"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		KeycloakAdmin: KeycloakAdminConfig{
			AdminUsername:  getEnv("KEYCLOAK_ADMIN_USERNAME", "admin"),
			AdminPassword:  os.Getenv("KEYCLOAK_ADMIN_PASSWORD"),
			AdminClientID:  getEnv("KEYCLOAK_ADMIN_CLIENT_ID", "admin-cli"),
			AdminRealm:     getEnv("KEYCLOAK_ADMIN_REALM", "master"),
			FrontendClient: getEnv("KEYCLOAK_FRONTEND_CLIENT", "agenthub-frontend"),
		},
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.KeycloakBaseURL == "" {
		return nil, fmt.Errorf("config: KEYCLOAK_BASE_URL is required")
	}

	corsOrigins := getEnv("CORS_ORIGINS", "*")
	cfg.CORSOrigins = strings.Split(corsOrigins, ",")

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
