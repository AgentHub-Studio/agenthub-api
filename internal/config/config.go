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

// MinIOConfig holds MinIO/S3 storage connection configuration.
type MinIOConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	Region          string
	Bucket          string // MINIO_BUCKET — used by the package registry
	DocumentsBucket string // MINIO_DOCUMENTS_BUCKET — used by document uploads
}

// IsConfigured returns true when the MinIO endpoint is set.
func (m MinIOConfig) IsConfigured() bool { return m.Endpoint != "" }

// Config holds all configuration for agenthub-api.
type Config struct {
	Port                string
	DatabaseURL         string
	KeycloakBaseURL     string
	KeycloakAdmin       KeycloakAdminConfig
	MinIO               MinIOConfig
	CORSOrigins         []string
	LogLevel            string
	OAuthEncryptionKey  string // 32-byte AES-256 key; empty disables encryption (dev mode)
	RabbitMQURL         string // RABBITMQ_URL — optional; enables document pipeline events when set
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

	cfg.OAuthEncryptionKey = os.Getenv("OAUTH_ENCRYPTION_KEY")

	cfg.RabbitMQURL = os.Getenv("RABBITMQ_URL")

	cfg.MinIO = MinIOConfig{
		Endpoint:        os.Getenv("MINIO_ENDPOINT"),
		AccessKeyID:     os.Getenv("MINIO_ACCESS_KEY"),
		SecretAccessKey: os.Getenv("MINIO_SECRET_KEY"),
		UseSSL:          os.Getenv("MINIO_USE_SSL") == "true",
		Region:          getEnv("MINIO_REGION", "us-east-1"),
		Bucket:          getEnv("MINIO_BUCKET", "agenthub-packages"),
		DocumentsBucket: getEnv("MINIO_DOCUMENTS_BUCKET", "agenthub-documents"),
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
