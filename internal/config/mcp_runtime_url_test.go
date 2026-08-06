package config_test

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
	"github.com/AgentHub-Studio/agenthub-api/internal/mcpruntime"
)

func TestLoad_MCPRuntimeURLDefaultMatchesServiceContract(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agenthub?sslmode=disable")
	t.Setenv("KEYCLOAK_BASE_URL", "http://localhost:28080")
	t.Setenv("MCP_RUNTIME_URL", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, mcpruntime.DefaultHTTPURL, cfg.MCPRuntimeURL)
}

func TestLoad_ReadsTenantWorkloadIdentityConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agenthub?sslmode=disable")
	t.Setenv("KEYCLOAK_BASE_URL", "http://localhost:28080")
	t.Setenv("MCP_RUNTIME_CLIENT_ID", "agenthub-api")
	t.Setenv("MCP_RUNTIME_AUDIENCE", "agenthub-mcp-client-runtime")
	t.Setenv("MCP_RUNTIME_CREDENTIAL_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012")))
	t.Setenv("MCP_RUNTIME_SCOPES", "openid, profile ,")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "agenthub-api", cfg.MCPRuntimeClientID)
	assert.Equal(t, "agenthub-mcp-client-runtime", cfg.MCPRuntimeAudience)
	assert.Equal(t, []string{"openid", "profile"}, cfg.MCPRuntimeScopes)
	assert.NotEmpty(t, cfg.MCPRuntimeCredentialEncryptionKey)
}
