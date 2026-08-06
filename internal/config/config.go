package config

import (
	"fmt"
	"os"
	"strconv"
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
	Port                              string
	DatabaseURL                       string
	KeycloakBaseURL                   string
	KeycloakIssuerURL                 string // KEYCLOAK_ISSUER_URL — bug 281: validar `iss` claim contra esta URL pública. Vazio = sem validação (legacy)
	KeycloakAdmin                     KeycloakAdminConfig
	MinIO                             MinIOConfig
	CORSOrigins                       []string
	LogLevel                          string
	OAuthEncryptionKey                string   // 32-byte AES-256 key; empty disables encryption (dev mode)
	RabbitMQURL                       string   // RABBITMQ_URL — optional; enables document pipeline events when set
	SkillRuntimeURL                   string   // SKILL_RUNTIME_URL — optional; base URL for skill-runtime service
	MCPRuntimeURL                     string   // MCP_RUNTIME_URL — optional; base URL for mcp-client-runtime service
	MCPRuntimeClientID                string   // MCP_RUNTIME_CLIENT_ID — tenant-local Keycloak service client for runtime calls
	MCPRuntimeAudience                string   // MCP_RUNTIME_AUDIENCE — audience required by mcp-client-runtime
	MCPRuntimeCredentialEncryptionKey string   // MCP_RUNTIME_CREDENTIAL_ENCRYPTION_KEY — base64-encoded 32-byte AES-256 key for tenant workload credentials
	MCPRuntimeScopes                  []string // MCP_RUNTIME_SCOPES — optional client-credentials scopes
	EmbeddingURL                      string   // EMBEDDING_URL — optional; base URL for embedding service (enables document_search)
	// LLMCallTimeoutSecs is the per-LLM-call timeout in seconds.
	// P-C102-1: prevents stalled providers from blocking goroutines indefinitely.
	// Default: 300 (5 minutes). Set to 0 to disable.
	LLMCallTimeoutSecs int // LLM_CALL_TIMEOUT_SECS
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Port:              getEnv("PORT", "8081"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		KeycloakBaseURL:   os.Getenv("KEYCLOAK_BASE_URL"),
		KeycloakIssuerURL: os.Getenv("KEYCLOAK_ISSUER_URL"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
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

	corsOrigins := getEnv("CORS_ORIGINS", "https://app.cezar.dev,https://test.cezar.dev,https://chat.cezar.dev,https://*.cezar.dev")
	cfg.CORSOrigins = strings.Split(corsOrigins, ",")

	cfg.OAuthEncryptionKey = os.Getenv("OAUTH_ENCRYPTION_KEY")

	cfg.RabbitMQURL = os.Getenv("RABBITMQ_URL")
	cfg.SkillRuntimeURL = getEnv("SKILL_RUNTIME_URL", "http://agenthub-skill-runtime:8083")
	cfg.MCPRuntimeURL = getEnv("MCP_RUNTIME_URL", "http://agenthub-mcp-client-runtime:8080")
	cfg.MCPRuntimeClientID = getEnv("MCP_RUNTIME_CLIENT_ID", "agenthub-api")
	cfg.MCPRuntimeAudience = getEnv("MCP_RUNTIME_AUDIENCE", "agenthub-mcp-client-runtime")
	cfg.MCPRuntimeCredentialEncryptionKey = os.Getenv("MCP_RUNTIME_CREDENTIAL_ENCRYPTION_KEY")
	cfg.MCPRuntimeScopes = splitNonEmpty(os.Getenv("MCP_RUNTIME_SCOPES"))
	cfg.EmbeddingURL = getEnv("EMBEDDING_URL", "http://agenthub-embedding:8092")

	// P-C102-1: per-LLM-call timeout. Default 300s (5 minutes).
	cfg.LLMCallTimeoutSecs = 300
	if v := os.Getenv("LLM_CALL_TIMEOUT_SECS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.LLMCallTimeoutSecs = n
		}
	}

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

func splitNonEmpty(csv string) []string {
	var values []string
	for _, value := range strings.Split(csv, ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
